package main

import (
	"fmt"
	"time"
)

var readChan = make(chan []byte, 2)
var gogo bool = false

func main() {
	go func() {
		for {
			if gogo == true {
				s := <-readChan
				fmt.Println(string(s))
			}
			// for {
			// 	time.Sleep(time.Duration(2) * time.Second)
			// }
		}
	}()
	go func() {
		for {
			readChan <- []byte(string("asd"))
			readChan <- []byte(string("asd"))
			// readChan <- []byte(string("asd"))
			// readChan <- []byte(string("asd"))
			gogo = true
			for {
				time.Sleep(time.Duration(2) * time.Second)
			}
		}
	}()
	for {
	}
}
