package main

import (
	"log"

	"github.com/darunshen/go/streamProtocol/rtsp"
)

func init() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
}
func main() {
	rtspServer := rtsp.Server{}
	rtspServer.Start("0.0.0.0:2333", 10485760, 10485760)
}
