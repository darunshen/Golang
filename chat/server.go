package chat

import (
	"fmt"
	"net"
	"sync"
)

var readData = make(chan byte, 1000)
var writeData = make(chan byte, 1000)
var stopData = make(chan bool, 1)
var connMap sync.Map

// 读取用户输入的消息并记录日志以及转发
// 每个人应该一次发送最多1000个字节
func read(con net.Conn) {
	for {
		n, err := con.Read(readData)
		if err != nil {
			fmt.Println(err)
			break
		}
		fmt.Println(string(data[0:n]))
	}
}

// 向其他所有用户发送此连接输入的消息
// 每个人应该一次发送最多1000个字节
func write(con net.Conn) {
	for {
		// select {
		// 	case
		// }
		n, err := con.Write(data)
		if err != nil {
			fmt.Println(err)
			break
		}
		fmt.Println(string(data[0:n]))
	}
}

// Start a chat server at host:port
func Start(host string, port int16) error {
	addr := host + ":" + string(port)
	fmt.Println("start server at ", addr)

	listen, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println(err)
		return err
	}
	for {
		con, err := listen.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		connMap.Store(con.LocalAddr().
			String()+con.RemoteAddr().String(), con)
		go Read(con)
	}
	return nil
}
