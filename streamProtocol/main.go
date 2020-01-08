package main

import (
	"github.com/darunshen/go/streamProtocol/rtsp"
	"log"
)

func init() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
}
func main() {
	rtspServer := rtsp.RtspServer{}
	rtspServer.Start("0.0.0.0:2333", 10485760, 10485760)
}
