package simple_network

import (
	"context"
	"encoding/binary"
	log "github.com/sirupsen/logrus"
	"io"
	"math"
	"net"
	"runtime/debug"
	"sync/atomic"
	"unsafe"
)

const (
	TcpSessionStop = iota
	TcpSessionRunning
)

type createPackFunc = func() []byte

type TcpSession struct {
	conn             net.Conn
	packFunction     createPackFunc
	ctx              context.Context
	cancel           context.CancelFunc
	headBuffer       []byte
	inBuffer         chan []byte
	outBuffer        chan []byte
	externalStopChan chan struct{} // 外部发起的stop
	internalStopChan chan struct{} // rcv或者send协议导致的stop
	running          uint32        // 连接状态
	inPackMaxSize    uint32        // 数据包最大的大小
	packHeadSize     uint8
}

func CreateTcpSession(conn net.Conn, maxInPack int, maxOutPack int, inPackMaxSize uint32, packHead any) ConnSession {
	ctx, cancel := context.WithCancel(context.Background())
	var session = &TcpSession{
		conn:             conn,
		ctx:              ctx,
		cancel:           cancel,
		inBuffer:         make(chan []byte, maxInPack),
		outBuffer:        make(chan []byte, maxOutPack),
		running:          TcpSessionRunning,
		externalStopChan: make(chan struct{}),
		internalStopChan: make(chan struct{}, 2),
	}

	if 0 == inPackMaxSize {
		session.inPackMaxSize = math.MaxUint32
	} else {
		session.inPackMaxSize = inPackMaxSize
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
		log.Error("rcv session:", s.conn.RemoteAddr(), " pack head size:", packLen, " less than require header size", s.packHeadSize)
		return nil
	}

	pack := make([]byte, packLen)
	binary.LittleEndian.PutUint16(pack, packLen)
	return pack
}

func (s *TcpSession) createUint32HeaderPack() []byte {
	packLen := binary.LittleEndian.Uint32(s.headBuffer)
	if packLen < uint32(s.packHeadSize) {
		log.Error("rcv session:", s.conn.RemoteAddr(), " pack head size:", packLen, " less than require header size", s.packHeadSize)
		return nil
	}

	pack := make([]byte, packLen)
	binary.LittleEndian.PutUint32(pack, packLen)
	return pack
}

func (s *TcpSession) run() {
	go s.rcvGoroutine()
	go s.sendGoroutine()

	// 等待第一个协程退出
	<-s.internalStopChan

	// 尝试改变状态，如果能改变，就关闭conn和chan，触发rcv、send协程退出，如果不能，则是外部调用了stop
	s.Stop()

	// 等待另一个协程退出
	<-s.internalStopChan
}

func (s *TcpSession) rcvGoroutine() {
	defer func() {
		if err := recover(); err != nil {
			log.Error(err, string(debug.Stack()))
		}

		log.Debug("receive routine exit")

		s.internalStopChan <- struct{}{}
	}()

	for {
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

		// 这里是否要修改为select模式，以解决接收的包满导致的阻塞
		//s.inBuffer <- pack

		// 非阻塞放入队列
		select {
		case s.inBuffer <- pack:
			// 成功
		case <-s.ctx.Done():
			// 连接已停止
			return
		default:
			// 队列满，丢弃数据包
			log.Warn("inBuffer full, dropping packet from:", s.conn.RemoteAddr())
		}
	}
}

func (s *TcpSession) sendGoroutine() {
	defer func() {
		if err := recover(); err != nil {
			log.Error(err, string(debug.Stack()))
		}

		log.Debug("send routine exit")

		s.internalStopChan <- struct{}{}
	}()

	for msg := range s.outBuffer {
		if _, err := s.conn.Write(msg); nil != err {
			log.Error(err)
			return
		}
	}
}

func (s *TcpSession) IsRunning() bool {
	return atomic.LoadUint32(&s.running) == TcpSessionRunning
}

// Stop 这个函数只能被外部的业务逻辑层调用，用于告知Run协程：外部已经不再对此conn作任何的调用了
func (s *TcpSession) Stop() {
	if atomic.CompareAndSwapUint32(&(s.running), TcpSessionRunning, TcpSessionStop) {
		s.cancel() // 先取消context
		s.conn.Close()
		close(s.outBuffer)
	}
}

func (s *TcpSession) Join() {
}

func (s *TcpSession) GetAddr() string {
	return s.conn.RemoteAddr().String()
}

func (s *TcpSession) GetPack() []byte {
	select {
	//case <-s.ctx.Done(): // 连接已停止
	//	return nil
	case pack := <-s.inBuffer:
		return pack
	default:
		return nil
	}
}

func (s *TcpSession) SendPack(pack []byte) bool {
	select {
	case <-s.ctx.Done(): // 连接已停止
		return false
	case s.outBuffer <- pack:
		return true
	default:
		return false
	}
}
