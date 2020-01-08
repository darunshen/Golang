package protocolinterface

import (
	"net"
)

// NetPackage support protocol package expression
type NetPackage interface {
}

// NetSession support connection info with server and client
type NetSession interface{}

// NetServer support protocol server operation
type NetServer interface {
	Start(address string) error
	StartSession(conn *net.TCPConn) error
	ReadPackage() error
	WritePackage(pack *NetPackage) error
	// test func()
}

// func (netServer *NetServer) test() {
// 	netServer.ReadPackage(nil)
// 	netServer.WritePackage(nil, nil)
// }
