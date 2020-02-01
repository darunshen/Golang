package rtsp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/darunshen/go/streamProtocol/protocolinterface"
)

// RtspServer rtsp server
type RtspServer struct {
	protocolinterface.BasicNet
}

// const(
// 	OPTIONS_METHOD = 1
// 	DESCRIBE_METHOD = 2
// 	ANNOUNCE_METHOD = 3
// )

// RtspPackage rtsp package
// RtspHeaderMap : store rtsp headers map
type RtspPackage struct {
	protocolinterface.NetPackage
	RtspHeaderMap map[string]string
	Method        string
	URL           string
	Version       string
	Content       string
	ResponseInfo
}

// ResponseInfo response info
type ResponseInfo struct {
	Status         string
	OptionsMethods string
}

// RtspNetSession rtsp net session
type RtspNetSession struct {
	protocolinterface.BasicNetSession
}

// CloseSession close session's connection and bufio
func (session *RtspNetSession) CloseSession() error {
	session.Bufio.Flush()
	session.Conn.Close()
	session.Conn = nil
	return nil
}

// ReadPackage read package for rtsp
func (session *RtspNetSession) ReadPackage() (interface{}, error) {
	fmt.Println("reading package from rtsp session")
	newPackage := new(RtspPackage)
	newPackage.RtspHeaderMap = make(map[string]string)
	reqData := bytes.NewBuffer(nil)
	for ifFirstLine := true; ; {
		if line, isPrefix, err :=
			session.Bufio.ReadLine(); err != nil {
			return nil,
				fmt.Errorf("session.Bufio.ReadLine() : %v", err)
		} else {
			reqData.Write(line)
			reqData.WriteString("\r\n")
			if !isPrefix {
				if ifFirstLine {
					items := regexp.MustCompile("\\s+").
						Split(strings.
							TrimSpace(string(line)), -1)
					if len(items) < 3 ||
						!strings.HasPrefix(items[2], "RTSP") {
						return nil,
							fmt.Errorf("first request line error")
					}
					newPackage.Method = items[0]
					newPackage.URL = items[1]
					newPackage.Version = items[2]
					ifFirstLine = false
				} else {
					if items := regexp.MustCompile(":\\s+").Split(strings.
						TrimSpace(string(line)), 2); len(items) == 2 {
						newPackage.RtspHeaderMap[items[0]] = items[1]
					}
				}
			}
			if len(line) == 0 {
				fmt.Printf("%v", reqData.String())
				if length, exist :=
					newPackage.RtspHeaderMap["Content-Length"]; exist {
					if lengthInt, err := strconv.Atoi(length); err == nil && lengthInt > 0 {
						content := make([]byte, lengthInt)
						if number, err := io.ReadFull(session.Bufio, content); err == nil {
							if number != lengthInt {
								return nil,
									fmt.Errorf("readed content length not equal to expected error")
							}
							newPackage.Content = string(content)
						} else {
							return nil, err
						}
					} else {
						return nil, err
					}
				}
				reqData.Reset()
				break
			}
		}
	}
	return newPackage, nil
}

// WritePackage write package for rtsp
func (session *RtspNetSession) WritePackage(pack interface{}) error {
	outputPackage := pack.(*RtspPackage)
	if seqNum, ok := outputPackage.RtspHeaderMap["CSeq"]; ok {
		responseBuf :=
			fmt.Sprintf("%s %s\r\nCSeq: %s\r\n",
				outputPackage.Version,
				outputPackage.ResponseInfo.Status,
				seqNum,
			)
		switch outputPackage.Method {
		case "OPTIONS":
			responseBuf += outputPackage.ResponseInfo.OptionsMethods
		}
		responseBuf += string("\r\n")
		if sendNum, err :=
			session.Bufio.WriteString(responseBuf); err != nil {
			return fmt.Errorf(`WritePackage's WriteString error,
				error = %v,expected sended 
				data number and real  = %v:%v`,
				err, len(responseBuf), sendNum)
		}
		if err := session.Bufio.Flush(); err != nil {
			return fmt.Errorf(`WritePackage's Flush error,error = %v`, err)
		}
		fmt.Printf("Writed to remote:%v", responseBuf)
	} else {
		return fmt.Errorf("WritePackage error: not find CSeq field")
	}
	return nil
}

// ProcessPackage process input package
func (session *RtspNetSession) ProcessPackage(pack interface{}) error {
	inputPackage := pack.(*RtspPackage)
	switch inputPackage.Method {
	case "OPTIONS":
		inputPackage.ResponseInfo.Status = "200 OK"
		inputPackage.ResponseInfo.OptionsMethods =
			"Public: DESCRIBE, SETUP, TEARDOWN, PLAY, " +
				"PAUSE, OPTIONS, ANNOUNCE, RECORD\r\n"
		return session.WritePackage(inputPackage)
	}
	return nil
}

// StartSession start a session with rtsp client
func (server *RtspServer) StartSession(conn *net.TCPConn) error {
	fmt.Println(
		"start session from ",
		conn.LocalAddr().String(),
		"to", conn.RemoteAddr().String())

	newSession := new(RtspNetSession)
	newSession.Conn = conn
	newSession.Bufio =
		bufio.NewReadWriter(
			bufio.NewReaderSize(conn, server.ReadBufferSize),
			bufio.NewWriterSize(conn, server.WriteBufferSize))
	server.SessionMapMutex.Lock()
	server.SessionMap[conn.
		LocalAddr().String()+conn.RemoteAddr().String()] = &(newSession.BasicNetSession)
	server.SessionMapMutex.Unlock()
	for {
		if pkg, err := newSession.ReadPackage(); err == nil {
			if err = newSession.ProcessPackage(pkg); err != nil {
				fmt.Printf("WritePackage error:%v", err)
				break
			}
		} else {
			fmt.Printf("ReadPackage error:%v", err)
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
