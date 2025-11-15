package simple_network

import (
	"net"
)

func TcpConnectTo(netType string, addr string, maxInPack int, maxOutPack int, packHeadSize any) (ConnSession, error) {
	conn, err := net.Dial(netType, addr)

	if nil != err {
		return nil, err
	}

	session := CreateTcpSession(conn, maxInPack, maxOutPack, packHeadSize)

	return session, nil
}
