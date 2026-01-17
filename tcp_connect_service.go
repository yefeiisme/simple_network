package simple_network

import (
	"net"
)

func TcpConnectTo(netType string, addr string, maxInPack int, maxOutPack int, inPackMaxSize uint32, packHeadSize any) (ConnSession, error) {
	conn, err := net.Dial(netType, addr)

	if nil != err {
		return nil, err
	}

	session := CreateTcpSession(conn, maxInPack, maxOutPack, inPackMaxSize, packHeadSize)

	return session, nil
}
