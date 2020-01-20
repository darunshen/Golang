package chat

// @todo 完成cmd调用的server　和　client功能
import (
	"fmt"
	"net"
	"strconv"
	"sync"
)

var readData = make([]byte, 1000)
var writeData = make(chan byte, 1000)
var stopData = make(chan bool, 1)
var connMap sync.Map

// 读取用户输入的消息并记录日志以及转发
// 每个人应该一次发送最多1000个字节
func read(con *net.Conn) {
	for {
		_, err := (*con).Read(readData)
		if err != nil {
			fmt.Println(err)
			break
		}
		connMap.Range(func(key, value interface{}) bool {
			if value != con {
				write(readData, value.(*net.Conn))
			}
			return true
		})
	}
}

// 向其他所有用户发送此连接输入的消息
// 每个人应该一次发送最多1000个字节
func write(data []byte, con *net.Conn) error {
	n, err := (*con).Write(data)
	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("Write error")
	}
	fmt.Println(
		"write to " +
			(*con).RemoteAddr().String() +
			"\ndata = " +
			string(data[0:n]))
	return nil
}

// Start a chat server at host:port
func Start(host string, port int) error {
	addr := host + ":" + strconv.Itoa(port)
	fmt.Print("start server at ", addr)

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
			String()+con.RemoteAddr().String(), &con)
		go read(&con)
	}
	return nil
}
