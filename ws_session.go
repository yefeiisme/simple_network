package simple_network

import (
	"strings"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type WsSession struct {
	conn             *websocket.Conn // websocket“句柄”
	recvChan         chan []byte     // 接收缓冲
	sendChan         chan []byte     // 发送缓冲
	running          bool            // 连接状态
	externalStopChan chan struct{}   // 外部发起的stop
	internalStopChan chan struct{}   // recv或者send协议导致的stop
}

func CreateWsSession(conn *websocket.Conn, maxRecv int, maxSend int) ConnSession {
	session := &WsSession{
		conn:             conn,
		recvChan:         make(chan []byte, maxRecv),
		sendChan:         make(chan []byte, maxSend),
		running:          true,
		externalStopChan: make(chan struct{}),
		internalStopChan: make(chan struct{}, 2),
	}

	go session.run()

	return session
}

func (s *WsSession) SetConn(connPtr *websocket.Conn) {
	s.conn = connPtr
}

func (s *WsSession) IsRunning() bool {
	return s.running
}

func (s *WsSession) run() {
	go s.readThread()
	go s.writeThread()

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

func (s *WsSession) Stop() {
	if s.running {
		s.running = false
		s.externalStopChan <- struct{}{}
	}
}

func (s *WsSession) Join() {
}

func (s *WsSession) GetAddr() string {
	return s.conn.RemoteAddr().String()
}

func (s *WsSession) readThread() {
	defer func() {
		log.Debug("receive routine exit")

		s.internalStopChan <- struct{}{}
	}()

	for s.running {
		msgType, msg, err := s.conn.ReadMessage()
		if err != nil /*&& !strings.Contains(err.Error(), "websocket: close 1005")*/ {
			if strings.Contains(err.Error(), "websocket: close 1000") || strings.Contains(err.Error(), "websocket: close 1001") {
				log.Info("ws session:", s.conn.RemoteAddr().String(), " closed, reason:", err.Error())
			} else {
				log.Error("ws session:", s.conn.RemoteAddr().String(), " read message error:", err.Error())
			}
			return
		}

		switch msgType {
		case websocket.TextMessage:
			log.Error("error: received TextMessage, need BinaryMessage")
			return
		case websocket.BinaryMessage:
			//log.Debug("Receive BinaryMessage:", msg)
			s.recvChan <- msg
			break
		case websocket.PingMessage:
			// 接收到了ping，重置链接的活跃时间
			s.keepAlive()
			break
		case websocket.PongMessage:
			// 暂不处理
			break
		case websocket.CloseMessage:
			log.Info("ws session:", s.conn.RemoteAddr().String(), " websocket.CloseMessage")
			// 退出
			return
		}
	}
}

func (s *WsSession) writeThread() {
	defer func() {
		log.Debug("send routine exit")

		s.internalStopChan <- struct{}{}
	}()

	for s.running {
		select {
		case msg := <-s.sendChan:
			if err := s.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				log.Error(err)
				return
			}
			break
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func (s *WsSession) addRecvedMsg(msg []byte) {
	s.recvChan <- msg
}

func (s *WsSession) SendPack(pack []byte) bool {
	select {
	case s.sendChan <- pack:
		return true
	default:
		return false
	}
}

func (s *WsSession) GetPack() []byte {
	select {
	case pack := <-s.recvChan:
		return pack
	default:
		return nil
	}
}

func (s *WsSession) keepAlive() {

}
