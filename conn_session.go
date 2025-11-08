package simple_netowrk

type ConnSession interface {
	IsRunning() bool
	Stop()
	Join()
	GetAddr() string
	GetPack() []byte
	SendPack(pack []byte) bool
}
