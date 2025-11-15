package simple_network

import (
	"encoding/binary"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"time"
	"unsafe"
)

type createPackFunc = func() []byte

type TcpSession struct {
	conn             net.Conn
	packFunction     createPackFunc
	headBuffer       []byte
	inBuffer         chan []byte
	outBuffer        chan []byte
	externalStopChan chan struct{} // 外部发起的stop
	internalStopChan chan struct{} // recv或者send协议导致的stop
	packHeadSize     uint8
	running          bool // 连接状态
}

func CreateTcpSession(conn net.Conn, maxInPack int, maxOutPack int, packHead any) ConnSession {
	var session = &TcpSession{
		conn:             conn,
		inBuffer:         make(chan []byte, maxInPack),
		outBuffer:        make(chan []byte, maxOutPack),
		running:          true,
		externalStopChan: make(chan struct{}),
		internalStopChan: make(chan struct{}, 2),
	}

	switch packHead.(type) {
	case uint16:
		session.packFunction = session.createUint16HeaderPack
		session.headBuffer = make([]byte, unsafe.Sizeof(uint16(0)))
		session.packHeadSize = uint8(unsafe.Sizeof(uint16(0)))
		break
	case uint32:
		session.packFunction = session.createUint32HeaderPack
		session.headBuffer = make([]byte, unsafe.Sizeof(uint32(0)))
		session.packHeadSize = uint8(unsafe.Sizeof(uint32(0)))
		break
	default:
		log.Errorf("invalid type:%T of pack head", packHead)
		return nil
	}

	go session.run()

	return session
}

func (s *TcpSession) createUint16HeaderPack() []byte {
	packLen := binary.LittleEndian.Uint16(s.headBuffer)
	if packLen < uint16(s.packHeadSize) {
		log.Error("recv session:", s.conn.RemoteAddr(), " pack head size:", packLen, " less than require header size", s.packHeadSize)
		return nil
	}

	pack := make([]byte, packLen)
	binary.LittleEndian.PutUint16(pack, packLen)
	return pack
}

func (s *TcpSession) createUint32HeaderPack() []byte {
	packLen := binary.LittleEndian.Uint32(s.headBuffer)
	if packLen < uint32(s.packHeadSize) {
		log.Error("recv session:", s.conn.RemoteAddr(), " pack head size:", packLen, " less than require header size", s.packHeadSize)
		return nil
	}

	pack := make([]byte, packLen)
	binary.LittleEndian.PutUint32(pack, packLen)
	return pack
}

func (s *TcpSession) run() {
	go s.recvGoroutine()
	go s.sendGoroutine()

	select {
	case <-s.externalStopChan:
		s.running = false
		s.conn.Close()

		// 等待recv和send协程退出
		<-s.internalStopChan
		<-s.internalStopChan
		break
	case <-s.internalStopChan:
		s.running = false
		s.conn.Close()

		// 等待另一个协程退出
		<-s.internalStopChan
		break
	}
}

func (s *TcpSession) recvGoroutine() {
	defer func() {
		log.Debug("receive routine exit")

		s.internalStopChan <- struct{}{}
	}()

	for s.running {
		if _, err := io.ReadFull(s.conn, s.headBuffer); nil != err {
			if io.EOF == err {
				log.Error("connection has been closed by client")
			} else {
				log.Error("conn read error: ", err)
			}
			return
		}

		pack := s.packFunction()
		if nil == pack {
			log.Error("create pack failed")
			return
		}

		packBody := pack[s.packHeadSize:]

		if _, err := io.ReadFull(s.conn, packBody); nil != err {
			if io.EOF == err {
				log.Error("connection has been closed by client")
			} else {
				log.Error("conn read error: ", err)
			}
			return
		}

		s.inBuffer <- pack
	}
}

func (s *TcpSession) sendGoroutine() {
	defer func() {
		log.Debug("send routine exit")

		s.internalStopChan <- struct{}{}
	}()

	for s.running {
		select {
		case msg := <-s.outBuffer:
			// 发送数据内容
			if _, err := s.conn.Write(msg); nil != err {
				//if err := binary.Write(s.conn, binary.LittleEndian, msg); nil != err {
				log.Error(err)
				return
			}
			break
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func (s *TcpSession) IsRunning() bool {
	return s.running
}

// 这个函数只能被外部的业务逻辑层调用，用于告知Run协程：外部已经不再对此conn作任何的调用了
func (s *TcpSession) Stop() {
	if s.running {
		s.running = false
		s.externalStopChan <- struct{}{}
	}
}

func (s *TcpSession) Join() {
}

func (s *TcpSession) GetAddr() string {
	return s.conn.RemoteAddr().String()
}

func (s *TcpSession) GetPack() []byte {
	select {
	case pack := <-s.inBuffer:
		return pack
	default:
		return nil
	}
}

func (s *TcpSession) SendPack(pack []byte) bool {
	select {
	case s.outBuffer <- pack:
		return true
	default:
		return false
	}
}
