package rtsp

import (
	"bytes"
	"fmt"
	"io"
	"net"
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

//ClientType rtsp client type
type ClientType int

const (
	//PusherClient the rtsp client is pusher
	PusherClient ClientType = 0
	//PullerClient the rtsp client is puller
	PullerClient ClientType = 1
)

//PackageType rtp/rtcp package type
type PackageType int

const (
	//RtpAudio stand for rtp audio package
	RtpAudio PackageType = 0
	//RtpVideo stand for rtp video package
	RtpVideo PackageType = 1
	//RtcpAudio stand for rctp audio package
	RtcpAudio PackageType = 2
	//RtcpVideo stand for rctp video package
	RtcpVideo PackageType = 3
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
	SdpMessage          *sdp.Message
	RtpChannel          int                         // for rtp in tcp session
	RtcpChannel         int                         // for rtcp　in tcp session
	RtpPortVideoClient  string                      // rtp client video port in udp session
	RtcpPortVideoClient string                      // rtcp client video port in udp session
	RtpPortAudioClient  string                      // rtp client audio port in udp session
	RtcpPortAudioClient string                      // rtcp client audio port in udp session
	RtpPortVideoServer  string                      // rtp Server video port in udp session
	RtcpPortVideoServer string                      // rtcp Server video port in udp session
	RtpPortAudioServer  string                      // rtp Server audio port in udp session
	RtcpPortAudioServer string                      // rtcp Server audio port in udp session
	SessionType         ClientType                  // session type (pusher or puller)
	AudioStreamName     string                      // audio stream name
	VideoStreamName     string                      // video stream name
	RtspURL             *url.URL                    // resource path in url
	ResourceMap         map[string]*ResourceSession // map url's resource to sessions
	ResourceMapMutex    *sync.Mutex                 // provide ResourceMap's atom
	SdpContent          string                      // sdp raw data from announce request
	RtpUDPAudioConn     *net.UDPConn                // rtp udp connection to puller audio
	RtcpUDPAudioConn    *net.UDPConn                // rtcp udp connection to puller audio
	RtpUDPVideoConn     *net.UDPConn                // rtp udp connection to puller video
	RtcpUDPVideoConn    *net.UDPConn                // rtcp udp connection to puller video
}

//ResourceSession resource session includes pusher and puller
type ResourceSession struct {
	Pusher      *NetSession
	Puller      []*NetSession
	PullerMutex sync.Mutex
}

// CloseSession close session's connection and bufio
func (session *NetSession) CloseSession() error {
	session.Bufio.Flush()
	session.Conn.Close()
	session.Conn = nil
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
		session.SdpMessage = sdpMessage
		session.SdpContent = string(inputPackage.Content)
		session.SessionType = PusherClient

		if err := session.ProcessSdpMessage(sdpMessage, inputPackage); err != nil {
			inputPackage.ResponseInfo.Error = InternalServerError
			return fmt.Errorf("ProcessSdpMessage error:%v", err)
		}
		if cmd, err := session.AddSessionToResourceMap(session.SessionType); err != nil {
			inputPackage.ResponseInfo.Error = cmd
			return fmt.Errorf("AddSessionToResourceMap error:%v", err)
		}
	case "SETUP":
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
				rtpPortClient := udpChannelMatcher[1]
				rtcpPortClient := udpChannelMatcher[3]
				var (
					rtpPort  string
					rtcpPort string
					err      error
				)
				if session.SessionType == PusherClient {
					if strings.Contains(inputPackage.URL, session.VideoStreamName) {
						session.RtpPortVideoClient = rtpPortClient
						session.RtcpPortVideoClient = rtcpPortClient
						rtpPort, err = session.StartUDPServer(RtpVideo)
						if err != nil {
							inputPackage.ResponseInfo.Error = InternalServerError
							return err
						}
						session.RtpPortVideoServer = rtpPort
						rtcpPort, err = session.StartUDPServer(RtcpVideo)
						if err != nil {
							inputPackage.ResponseInfo.Error = InternalServerError
							return err
						}
						session.RtcpPortVideoServer = rtcpPort
						fmt.Printf("rtp server port for video = %v,and rtcp port = %v\n",
							rtpPort, rtcpPort)
					}
					if strings.Contains(inputPackage.URL, session.AudioStreamName) {
						session.RtpPortAudioClient = rtpPortClient
						session.RtcpPortAudioClient = rtcpPortClient
						rtpPort, err = session.StartUDPServer(RtpAudio)
						if err != nil {
							inputPackage.ResponseInfo.Error = InternalServerError
							return err
						}
						session.RtpPortAudioServer = rtpPort
						rtcpPort, err = session.StartUDPServer(RtcpAudio)
						if err != nil {
							inputPackage.ResponseInfo.Error = InternalServerError
							return err
						}
						session.RtcpPortAudioServer = rtcpPort
						fmt.Printf("rtp server port for audio = %v,and rtcp port = %v\n",
							rtpPort, rtcpPort)
					}
					inputPackage.ResponseInfo.SetupTransport =
						fmt.Sprintf("Transport: %v;server_port=%v-%v",
							transport, rtpPort, rtcpPort)
					inputPackage.ResponseInfo.Error = Ok
				} else if session.SessionType == PullerClient {
					if strings.Contains(inputPackage.URL, session.VideoStreamName) {
						session.RtpPortVideoClient = rtpPortClient
						session.RtcpPortVideoClient = rtcpPortClient
						if err = session.StartUDPClient(RtpVideo); err != nil {
							inputPackage.ResponseInfo.Error = InternalServerError
							return err
						}
						if err = session.StartUDPClient(RtcpVideo); err != nil {
							inputPackage.ResponseInfo.Error = InternalServerError
							return err
						}
						fmt.Printf("connected to puller\n\trtp port for video = %v,and rtcp port = %v\n",
							rtpPortClient, rtcpPortClient)
					}
					if strings.Contains(inputPackage.URL, session.AudioStreamName) {
						session.RtpPortAudioClient = rtpPortClient
						session.RtcpPortAudioClient = rtcpPortClient
						if err = session.StartUDPClient(RtpVideo); err != nil {
							inputPackage.ResponseInfo.Error = InternalServerError
							return err
						}
						if err = session.StartUDPClient(RtcpVideo); err != nil {
							inputPackage.ResponseInfo.Error = InternalServerError
							return err
						}
						fmt.Printf("connected to puller\n\trtp port for audio = %v,and rtcp port = %v\n",
							rtpPortClient, rtcpPortClient)
					}
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
		if cmd, err := session.AddSessionToResourceMap(session.SessionType); err != nil {
			inputPackage.ResponseInfo.Error = cmd
			return fmt.Errorf("AddSessionToResourceMap error:%v", err)
		}
		pusherSession := session.ResourceMap[session.RtspURL.Path].Pusher
		inputPackage.ResponseInfo.DescribeContent =
			fmt.Sprintf("Content-Type: application/sdp\r\nContent-Length: %v\r\n%v",
				len(pusherSession.SdpContent), pusherSession.SdpContent)
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
		responseBuf += string("\r\n")
		fmt.Println(responseBuf)
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

//StartUDPServer start udp server for rtp rtcp
func (session *NetSession) StartUDPServer(packageType PackageType) (string, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return "", err
	}
	udpConnection, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return "", err
	}
	if err := udpConnection.SetReadBuffer(session.ReadBufferSize); err != nil {
		return "", err
	}
	if err := udpConnection.SetWriteBuffer(session.WriteBufferSize); err != nil {
		return "", err
	}
	if port := regexp.MustCompile(":(\\d+)").
		FindStringSubmatch(udpConnection.LocalAddr().String()); port != nil {
		return port[1], nil
	} else {
		return "", fmt.Errorf("not find udp port number in %v",
			udpConnection.LocalAddr().String())
	}
}

//StartUDPClient start udp connection to puller
func (session *NetSession) StartUDPClient(packageType PackageType) error {
	host := session.Conn.RemoteAddr().String()
	ip := host[:strings.LastIndex(host, ":")]
	var port string
	switch packageType {
	case RtpAudio:
		port = session.RtpPortAudioClient
	case RtpVideo:
		port = session.RtpPortVideoClient
	case RtcpAudio:
		port = session.RtcpPortAudioClient
	case RtcpVideo:
		port = session.RtcpPortVideoClient
	}
	host = ip + ":" + port
	udpAddr, err := net.ResolveUDPAddr("udp", host)
	if err != nil {
		return err
	}
	udpConnection, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return err
	}
	if err := udpConnection.SetReadBuffer(session.ReadBufferSize); err != nil {
		return err
	}
	if err := udpConnection.SetWriteBuffer(session.WriteBufferSize); err != nil {
		return err
	}
	switch packageType {
	case RtpAudio:
		session.RtpUDPAudioConn = udpConnection
	case RtcpAudio:
		session.RtcpUDPAudioConn = udpConnection
	case RtpVideo:
		session.RtpUDPVideoConn = udpConnection
	case RtcpVideo:
		session.RtcpUDPVideoConn = udpConnection
	}
	return nil
}

//ProcessSdpMessage print sdp message content
func (session *NetSession) ProcessSdpMessage(sdpMessage *sdp.Message, rtspPackage *Package) error {
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
		switch media.Description.Type {
		case "audio":
			session.AudioStreamName = media.Attributes.Value("control")
		case "video":
			session.VideoStreamName = media.Attributes.Value("control")
		default:
			rtspPackage.Error = UnsupportedMediaType
			return fmt.Errorf("Unsupported Media Type")
		}
	}
	return nil
}

//AddSessionToResourceMap Add Session To ResourceMap based on sessionType
func (session *NetSession) AddSessionToResourceMap(
	sessionType ClientType) (CommandError, error) {
	session.ResourceMapMutex.Lock()
	defer session.ResourceMapMutex.Unlock()
	switch sessionType {
	case PusherClient:
		if _, ok := session.ResourceMap[session.RtspURL.Path]; ok {
			return Forbidden, fmt.Errorf("pusher's request's url already used")
		}
		session.ResourceMap[session.RtspURL.Path] = &ResourceSession{
			Pusher: session,
			Puller: make([]*NetSession, 5),
		}
	case PullerClient:
		if _, ok := session.ResourceMap[session.RtspURL.Path]; !ok {
			return Forbidden, fmt.Errorf("puller's request's url not found")
		}
		session.ResourceMap[session.RtspURL.Path].Puller =
			append(session.ResourceMap[session.RtspURL.Path].Puller, session)
		session.AudioStreamName =
			session.ResourceMap[session.RtspURL.Path].Pusher.AudioStreamName
		session.VideoStreamName =
			session.ResourceMap[session.RtspURL.Path].Pusher.VideoStreamName
	}
	return Ok, nil
}
