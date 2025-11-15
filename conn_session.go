package simple_network

type ConnSession interface {
	IsRunning() bool
	Stop()
	Join()
	GetAddr() string
	GetPack() []byte
	SendPack(pack []byte) bool
}
