package gotest

import (
	"github.com/darunshen/go/streamProtocol/rtsp"
	"testing"
)

func TestWriteReadRtspPackage(t *testing.T) {
	server := rtsp.RtspServer{}
	server.WritePackage(nil)
	server.ReadPackage()
}
