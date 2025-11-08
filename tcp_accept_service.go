package simple_netowrk

import (
	log "github.com/sirupsen/logrus"
	"net"
	"sync"
)

type TcpAcceptCallback = func(session net.Conn)

type TcpAcceptService struct {
	listener  net.Listener
	onConnect TcpAcceptCallback
	wg        *sync.WaitGroup
}

func TcpAcceptor(netType string, localAddr string, onConnect TcpAcceptCallback) *TcpAcceptService {
	service := &TcpAcceptService{
		listener:  nil,
		onConnect: onConnect,
		wg:        &sync.WaitGroup{},
	}

	var err error
	service.listener, err = net.Listen(netType, localAddr)
	if nil != err {
		log.Error("net.Listen(", netType, ", ", localAddr, ") failed")
		return nil
	}

	go service.run()

	return service
}

func (s *TcpAcceptService) Stop() {
	if nil != s.listener {
		s.listener.Close()
	}
}

func (s *TcpAcceptService) Join() {
	s.wg.Wait()
}

func (s *TcpAcceptService) run() {
	defer func() {
		s.wg.Done()
	}()

	s.wg.Add(1)

	for {
		if conn, err := s.listener.Accept(); err != nil {
			if err.(*net.OpError).Err == net.ErrClosed {
				log.Info("Stop accept, because listener closed")
				return
			}

			log.Error("Accept error:", err)
		} else {
			log.Debug("Incoming conn")
			s.onConnect(conn)
		}
	}
}
