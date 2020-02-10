package protocolinterface

import (
	"bufio"
	"net"
	"sync"
)

// BasicNet support basic operation for net interface
type BasicNet struct {
	NetServer
	Host            string
	Port            int
	TCPListener     *net.TCPListener
	IfStop          bool
	SessionMap      map[string]*BasicNetSession
	SessionMapMutex sync.Mutex
	ReadBufferSize  int // bio&tcp read buffer size
	WriteBufferSize int // bio&tcp write buffer size
}

// BasicNetSession support basic operation
// for session within server and client
type BasicNetSession struct {
	NetSession
	Conn            *net.TCPConn
	Bufio           *bufio.ReadWriter
	extra           *interface{}
	ID              string
	ReadBufferSize  int // udp/tcp read buffer size
	WriteBufferSize int // udp/tcp write buffer size
}
