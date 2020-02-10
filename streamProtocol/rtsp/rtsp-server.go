package rtsp

import (
	"bufio"
	"fmt"
	"net"
	"sync"

	"github.com/darunshen/go/streamProtocol/protocolinterface"
	"github.com/teris-io/shortid"
)

// Server rtsp server
type Server struct {
	protocolinterface.BasicNet
	ResourceMap      map[string]*ResourceSession
	ResourceMapMutex sync.Mutex
}

// StartSession start a session with rtsp client
func (server *Server) StartSession(conn *net.TCPConn) error {
	fmt.Println(
		"start session from ",
		conn.LocalAddr().String(),
		"to", conn.RemoteAddr().String())

	newSession := new(NetSession)
	newSession.Conn = conn
	newSession.ResourceMap = server.ResourceMap
	newSession.ResourceMapMutex = &server.ResourceMapMutex
	newSession.Bufio =
		bufio.NewReadWriter(
			bufio.NewReaderSize(conn, server.ReadBufferSize),
			bufio.NewWriterSize(conn, server.WriteBufferSize))
	newSession.ID = shortid.MustGenerate()
	newSession.ReadBufferSize = server.ReadBufferSize
	newSession.WriteBufferSize = server.WriteBufferSize
	for {
		if pkg, err := newSession.ReadPackage(); err == nil {
			if err = newSession.ProcessPackage(pkg); err != nil {
				fmt.Printf("ProcessPackage error:%v\n", err)
				break
			}
			if err = newSession.WritePackage(pkg); err != nil {
				fmt.Printf("WritePackage error:%v\n", err)
				break
			}
		} else {
			fmt.Printf("ReadPackage error:%v\n", err)
			break
		}
	}
	if err := newSession.CloseSession(); err != nil {
		return fmt.Errorf("CloseSession error:%v", err)
	}
	return fmt.Errorf("StartSession error")
}

/*Start start a rtsp server
bufferReadSize: bio&tcp read buffer size
bufferWriteSize: bio&tcp write buffer size
*/
func (server *Server) Start(address string,
	bufferReadSize int,
	bufferWriteSize int) error {
	server.ReadBufferSize = bufferReadSize
	server.WriteBufferSize = bufferWriteSize
	server.ResourceMap = make(map[string]*ResourceSession)
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
