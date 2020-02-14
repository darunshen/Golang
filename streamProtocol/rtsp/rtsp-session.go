package rtsp

//@todo add session 'Client State Machine'
import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/darunshen/go/streamProtocol/protocolinterface"
	"gortc.io/sdp"
)

//CommandError rtsp command error number
type CommandError string

const (
	//Ok it's ok
	Ok CommandError = "200 OK"
	//NotSupport not support this mothod
	NotSupport CommandError = "405 Method Not Allowed"
	//InternalServerError internal server error
	InternalServerError CommandError = "500 Internal Server Error"
	//UnsupportedMediaType Unsupported Media Type
	UnsupportedMediaType CommandError = "415 Unsupported Media Type"
	//UnsupportedTransport Unsupported Transport
	UnsupportedTransport CommandError = "461 Unsupported transport"
	//BadRequest bad request like url invalid
	BadRequest CommandError = "400 Bad Request"
	//Forbidden forbidden request like pusher's request's resource path already used
	Forbidden CommandError = "403 Forbidden"
	//NotFound not found the resource path for puller client
	NotFound CommandError = "404 Not Found"
)

// ResponseInfo response info
type ResponseInfo struct {
	Error           CommandError
	OptionsMethods  string
	SetupTransport  string
	DescribeContent string
}

// Package rtsp package
type Package struct {
	protocolinterface.NetPackage
	RtspHeaderMap map[string]string
	Method        string
	URL           string
	Version       string
	Content       []byte
	ResponseInfo
}

// NetSession rtsp net session
type NetSession struct {
	protocolinterface.BasicNetSession
	SessionType                  ClientType                       // type for pusher or puller
	RtpChannel                   int                              // for rtp in tcp session
	RtcpChannel                  int                              // for rtcp　in tcp session
	AudioStreamName              string                           // audio stream name
	VideoStreamName              string                           // video stream name
	RtspURL                      *url.URL                         // resource path in url
	PusherPullersSessionMap      map[string]*PusherPullersSession // map url's resource to sessions
	PusherPullersSessionMapMutex *sync.Mutex                      // provide ResourceMap's atom
	SdpContent                   string                           // sdp raw data from announce request
	SourcePath                   string                           // Source Path of request url
}

// CloseSession close session's connection and bufio
func (session *NetSession) CloseSession() error {
	// session.Bufio.Flush()
	session.Conn.Close()
	session.Conn = nil
	if _, ok := session.PusherPullersSessionMap[session.SourcePath]; ok {
		session.PusherPullersSessionMapMutex.Lock()
		delete(session.PusherPullersSessionMap, session.SourcePath)
		session.PusherPullersSessionMapMutex.Unlock()
	}
	return nil
}

// ReadPackage read package for rtsp
func (session *NetSession) ReadPackage() (interface{}, error) {
	fmt.Println("reading package from rtsp session")
	newPackage := new(Package)
	newPackage.RtspHeaderMap = make(map[string]string)
	newPackage.Error = Ok
	reqData := bytes.NewBuffer(nil)
	for ifFirstLine := true; ; {
		line, isPrefix, err :=
			session.Bufio.ReadLine()
		if err != nil {
			return nil,
				fmt.Errorf("session.Bufio.ReadLine() : %v", err)
		}
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
					if _, err := io.ReadFull(session.Bufio, content); err == nil {
						newPackage.Content = content
						fmt.Print(string(content))
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
	return newPackage, nil
}

// ProcessPackage process input package
func (session *NetSession) ProcessPackage(pack interface{}) error {
	inputPackage := pack.(*Package)
	var err error
	if session.RtspURL, err = url.Parse(inputPackage.URL); err != nil {
		inputPackage.ResponseInfo.Error = BadRequest
		return fmt.Errorf("url.Parse error:%v", err)
	}
	switch inputPackage.Method {
	case "OPTIONS":
		inputPackage.ResponseInfo.Error = Ok
		inputPackage.ResponseInfo.OptionsMethods =
			"Public: DESCRIBE, SETUP, TEARDOWN, PLAY, " +
				"PAUSE, OPTIONS, ANNOUNCE, RECORD\r\n"
	case "ANNOUNCE":
		var (
			sdpSession sdp.Session
		)
		session.SessionType = PusherClient
		if _, ok := session.PusherPullersSessionMap[session.RtspURL.Path]; ok {
			inputPackage.ResponseInfo.Error = Forbidden
			return fmt.Errorf("pusher's request's url already used")
		}
		pps := new(PusherPullersSession)
		session.PusherPullersSessionMapMutex.Lock()
		session.PusherPullersSessionMap[session.RtspURL.Path] = pps
		session.PusherPullersSessionMapMutex.Unlock()
		session.SourcePath = session.RtspURL.Path
		if sdpSession, err = sdp.DecodeSession(inputPackage.Content, sdpSession); err != nil {
			inputPackage.ResponseInfo.Error = InternalServerError
			return fmt.Errorf("sdp.DecodeSession error:%v", err)
		}
		sdpDecoder := sdp.NewDecoder(sdpSession)
		sdpMessage := new(sdp.Message)
		if err = sdpDecoder.Decode(sdpMessage); err != nil {
			inputPackage.ResponseInfo.Error = InternalServerError
			return fmt.Errorf("sdpDecoder.Decode error:%v", err)
		}
		pps.SdpMessage = sdpMessage
		sdpC := string(inputPackage.Content)
		pps.SdpContent = &sdpC

		if err := session.ProcessSdpMessage(sdpMessage, inputPackage, pps); err != nil {
			inputPackage.ResponseInfo.Error = InternalServerError
			return fmt.Errorf("ProcessSdpMessage error:%v", err)
		}
	case "SETUP":
		/*
			setup the udp/tcp connection for audio/video media in rtp/rtcp protocol

			if udp and puller,start two connections to puller
		*/
		if transport, ok := inputPackage.RtspHeaderMap["Transport"]; ok {
			if tcpChannelMatcher :=
				regexp.MustCompile("interleaved=(\\d+)(-(\\d+))?").
					FindStringSubmatch(transport); tcpChannelMatcher != nil {
				session.RtpChannel, _ = strconv.Atoi(tcpChannelMatcher[1])
				session.RtcpChannel, _ = strconv.Atoi(tcpChannelMatcher[3])
				inputPackage.ResponseInfo.Error = Ok
			} else if udpChannelMatcher :=
				regexp.MustCompile("client_port=(\\d+)(-(\\d+))?").
					FindStringSubmatch(transport); udpChannelMatcher != nil {
				var rtpPort = new(string)
				var rtcpPort = new(string)
				*rtpPort = udpChannelMatcher[1]
				*rtcpPort = udpChannelMatcher[3]
				var (
					mediaType    MediaType
					mediaName    string
					resourcePath string
				)
				if index := strings.Index(session.RtspURL.Path, "/streamid="); index != -1 {
					resourcePath = session.RtspURL.Path[:index]
				} else {
					resourcePath = session.RtspURL.Path
				}
				pps, ok := session.PusherPullersSessionMap[resourcePath]
				if !ok {
					inputPackage.ResponseInfo.Error = InternalServerError
					return fmt.Errorf("not find pusher-puller-session of url:%v",
						session.RtspURL.Path)
				}

				if strings.Contains(inputPackage.URL, *pps.VideoStreamName) {
					mediaType = MediaVideo
					mediaName = "video"
				}
				if strings.Contains(inputPackage.URL, *pps.AudioStreamName) {
					mediaType = MediaAudio
					mediaName = "audio"
				}
				if err := pps.AddRtpRtcpSession(
					session.SessionType,
					mediaType,
					rtpPort,
					rtcpPort,
					session.RemoteIP,
				); err != nil {
					inputPackage.ResponseInfo.Error = InternalServerError
					return fmt.Errorf("AddRtpRtcpSession faied:%v", err)
				}
				if session.SessionType == PusherClient {
					fmt.Printf("rtp server port for %v = %v,and rtcp port = %v\n",
						mediaName, *rtpPort, *rtcpPort)
					inputPackage.ResponseInfo.SetupTransport =
						fmt.Sprintf("Transport: %v;server_port=%v-%v\n",
							transport, *rtpPort, *rtcpPort)
					inputPackage.ResponseInfo.Error = Ok
				} else if session.SessionType == PullerClient {
					fmt.Printf("connected to puller\n\trtp port for %v = %v,and rtcp port = %v\n",
						mediaName, *rtpPort, *rtcpPort)
					inputPackage.ResponseInfo.SetupTransport =
						fmt.Sprintf("Transport: %v", transport)
					inputPackage.ResponseInfo.Error = Ok
				}
			} else {
				inputPackage.ResponseInfo.Error = UnsupportedTransport
			}
		}
	case "DESCRIBE":
		session.SessionType = PullerClient
		pps, ok := session.PusherPullersSessionMap[session.RtspURL.Path]
		if !ok {
			inputPackage.ResponseInfo.Error = Forbidden
			return fmt.Errorf("puller's request's url not found")
		}
		inputPackage.ResponseInfo.DescribeContent =
			fmt.Sprintf("Content-Type: application/sdp\r\nContent-Length: %v\r\n\r\n%v",
				len(*pps.SdpContent), *pps.SdpContent)

	default:
		inputPackage.Error = NotSupport
	}
	return nil
}

// WritePackage write package for rtsp
func (session *NetSession) WritePackage(pack interface{}) error {
	outputPackage := pack.(*Package)
	if seqNum, ok := outputPackage.RtspHeaderMap["CSeq"]; ok {
		responseBuf :=
			fmt.Sprintf("%s %s\r\nCSeq: %s\r\nSession: %s\r\n",
				outputPackage.Version,
				outputPackage.ResponseInfo.Error,
				seqNum,
				session.ID,
			)
		if outputPackage.Error == Ok {
			switch outputPackage.Method {
			case "OPTIONS":
				responseBuf += outputPackage.ResponseInfo.OptionsMethods
			case "SETUP":
				responseBuf += outputPackage.ResponseInfo.SetupTransport
			case "DESCRIBE":
				responseBuf += outputPackage.ResponseInfo.DescribeContent
			}
		}
		if outputPackage.Method != "DESCRIBE" {
			responseBuf += string("\r\n")
		}
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
		fmt.Printf("Writed to remote:\r\n%v", responseBuf)
	} else {
		return fmt.Errorf("WritePackage error: not find CSeq field")
	}
	return nil
}

//ProcessSdpMessage print sdp message content
func (session *NetSession) ProcessSdpMessage(
	sdpMessage *sdp.Message, rtspPackage *Package, pps *PusherPullersSession) error {
	fmt.Println("URI", sdpMessage.URI)
	fmt.Println("Info:", sdpMessage.Info)
	fmt.Println("Origin.Address:", sdpMessage.Origin.Address)
	fmt.Println("Origin.AddressType:", sdpMessage.Origin.AddressType)
	fmt.Println("Origin.NetworkType:", sdpMessage.Origin.NetworkType)
	fmt.Println("Origin.SessionID:", sdpMessage.Origin.SessionID)
	fmt.Println("Origin.SessionVersion:", sdpMessage.Origin.SessionVersion)
	fmt.Println("Origin.Username:", sdpMessage.Origin.Username)
	for index, media := range sdpMessage.Medias {
		fmt.Println("index =", index)
		fmt.Println("media.Title:", media.Title)
		fmt.Println("media.Description.Type:", media.Description.Type)
		fmt.Println("media.Description.Port:", media.Description.Port)
		fmt.Println("media.Description.PortsNumber:", media.Description.PortsNumber)
		fmt.Println("media.Description.Protocol:", media.Description.Protocol)
		fmt.Println("media.Description.Formats:", media.Description.Formats)
		fmt.Println("media.Connection.NetworkType:", media.Connection.NetworkType)
		fmt.Println("media.Connection.AddressType:", media.Connection.AddressType)
		fmt.Println("media.Connection.IP:", media.Connection.IP)
		fmt.Println("media.Connection.TTL:", media.Connection.TTL)
		fmt.Println("media.Connection.Addresses:", media.Connection.Addresses)
		fmt.Println("media.Attributes[control]:", media.Attributes.Value("control"))
		fmt.Println("media.Bandwidths:")
		for bwType, bwValue := range media.Bandwidths {
			fmt.Println("type =", bwType, "value =", bwValue)
		}
		asn := media.Attributes.Value("control")
		switch media.Description.Type {
		case "audio":
			pps.AudioStreamName = &asn
		case "video":
			pps.VideoStreamName = &asn
		default:
			rtspPackage.Error = UnsupportedMediaType
			return fmt.Errorf("Unsupported Media Type")
		}
	}
	return nil
}
