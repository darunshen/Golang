package main

import (
	"log"

	"github.com/darunshen/go/streamProtocol/rtsp"
)

func init() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
}

const (
	//ReadBuffer bio&tcp&udp read buffer size
	ReadBuffer int = 10485760
	//WriteBuffer bio&tcp&udp write buffer size
	WriteBuffer int = 10485760
	//PushChannelBuffer pusher channel buffer size
	PushChannelBuffer int = 1
	//PullChannelBuffer puller channel buffer size
	PullChannelBuffer int = 1
)

func main() {
	rtspServer := rtsp.Server{}
	rtspServer.Start("0.0.0.0:2333",
		ReadBuffer, WriteBuffer, PushChannelBuffer, PullChannelBuffer)
}
