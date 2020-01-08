package rtsp

import (
	"bufio"
	"fmt"
	"github.com/darunshen/go/streamProtocol/protocolinterface"
	"net"
)

// RtspServer rtsp server
type RtspServer struct {
	protocolinterface.BasicNet
}

// ReadPackage read package for rtsp
func (server *RtspServer) ReadPackage() error {
	fmt.Println("read package from rtsp server")
	return nil
}

// WritePackage read package for rtsp
func (server *RtspServer) WritePackage(pack *protocolinterface.NetPackage) error {
	fmt.Println("write package from rtsp server")
	return nil
}

// StartSession start a session with rtsp client
func (server *RtspServer) StartSession(conn *net.TCPConn) error {
	fmt.Println(
		"start session from ",
		conn.LocalAddr().String(),
		"to", conn.RemoteAddr().String())

	newSession := new(protocolinterface.BasicNetSession)
	newSession.Conn = conn
	newSession.Bufio =
		bufio.NewReadWriter(
			bufio.NewReaderSize(conn, server.ReadBufferSize),
			bufio.NewWriterSize(conn, server.WriteBufferSize))
	server.SessionMapMutex.Lock()
	server.SessionMap[conn.
		LocalAddr().String()+conn.RemoteAddr().String()] = newSession
	server.SessionMapMutex.Unlock()
	server.ReadPackage()
	return nil
}

// Start start a rtsp server
func (server *RtspServer) Start(address string,
	bufferReadSize int,
	bufferWriteSize int) error {
	server.ReadBufferSize = bufferReadSize
	server.WriteBufferSize = bufferWriteSize
	server.SessionMap = make(map[string]*protocolinterface.BasicNetSession)
	addr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return fmt.Errorf("address resolving failed : %v", err)
	}
	server.Host, _, _ = net.SplitHostPort(address)
	server.Port = addr.Port
	if listener, err := net.ListenTCP("tcp", addr); err != nil {
		return fmt.Errorf("listen tcp failed : %v", err)
	} else {
		fmt.Println("Start listening at ", address)
		server.TCPListener = listener
	}

	server.IfStop = false
	for !server.IfStop {
		conn, err := server.TCPListener.AcceptTCP()
		if err != nil {
			fmt.Println("AcceptTCP failed : ", err)
			continue
		}
		if err := conn.SetReadBuffer(server.ReadBufferSize); err != nil {
			return fmt.Errorf("SetReadBuffer error, %v", err)
		}
		if err := conn.SetWriteBuffer(server.WriteBufferSize); err != nil {
			return fmt.Errorf("SetWriteBuffer error, %v", err)
		}
		go server.StartSession(conn)
	}
	return nil
}
