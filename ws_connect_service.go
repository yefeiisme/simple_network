package simple_netowrk

import (
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

func ConnectTo(addr string, maxInPack int, maxOutPack int) (ConnSession, error) {
	conn, _, err := websocket.DefaultDialer.Dial(addr, nil)

	if nil != err {
		log.Error("connect to: ", addr, " error: ", err)
		return nil, err
	}

	session := CreateWsSession(conn, maxInPack, maxOutPack)

	return session, nil
}
