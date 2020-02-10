package rtsp

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"

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
	Error          CommandError
	OptionsMethods string
	SetupTransport string
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
	SdpMessage      *sdp.Message
	RtpChannel      int        // for rtp in tcp session
	RtcpChannel     int        // for rtcp　in tcp session
	RtpPortClient   int        // rtp client port in udp session
	RtcpPortClient  int        // rtcp client port in udp session
	RtpPortServer   int        // rtp Server port in udp session
	RtcpPortServer  int        // rtcp Server port in udp session
	SessionType     ClientType // session type (pusher or puller)
	AudioStreamName string     // audio stream name
	VideoStreamName string     // video stream name
	*net.UDPConn
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
	switch inputPackage.Method {
	case "OPTIONS":
		inputPackage.ResponseInfo.Error = Ok
		inputPackage.ResponseInfo.OptionsMethods =
			"Public: DESCRIBE, SETUP, TEARDOWN, PLAY, " +
				"PAUSE, OPTIONS, ANNOUNCE, RECORD\r\n"
	case "ANNOUNCE":
		var (
			sdpSession sdp.Session
			err        error
		)
		if sdpSession, err = sdp.DecodeSession(inputPackage.Content, sdpSession); err != nil {
			return err
		}
		sdpDecoder := sdp.NewDecoder(sdpSession)
		sdpMessage := new(sdp.Message)
		if err = sdpDecoder.Decode(sdpMessage); err != nil {
			return fmt.Errorf("err:%v", err)
		}
		session.SdpMessage = sdpMessage
		session.SessionType = PusherClient
		if err := session.ProcessSdpMessage(sdpMessage, inputPackage); err == nil {
			inputPackage.ResponseInfo.Error = Ok
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
				session.RtpPortClient, _ = strconv.Atoi(udpChannelMatcher[1])
				session.RtcpPortClient, _ = strconv.Atoi(udpChannelMatcher[3])
				var (
					rtpPort  int
					rtcpPort int
					err      error
				)
				if session.SessionType == PusherClient {
					if strings.Contains(inputPackage.URL, session.VideoStreamName) {
						rtpPort, err = session.StartUDPServer(RtpVideo)
						if err != nil {
							inputPackage.ResponseInfo.Error = InternalServerError
							return err
						}
						session.RtpPortServer = rtpPort
						rtcpPort, err = session.StartUDPServer(RtcpVideo)
						if err != nil {
							inputPackage.ResponseInfo.Error = InternalServerError
							return err
						}
						session.RtcpPortServer = rtcpPort
						fmt.Printf("rtp port for video = %v,and rtcp port = %v\n",
							rtpPort, rtcpPort)
					}
					if strings.Contains(inputPackage.URL, session.AudioStreamName) {

						rtpPort, err = session.StartUDPServer(RtpAudio)
						if err != nil {
							inputPackage.ResponseInfo.Error = InternalServerError
							return err
						}
						session.RtpPortServer = rtpPort
						rtcpPort, err = session.StartUDPServer(RtcpAudio)
						if err != nil {
							inputPackage.ResponseInfo.Error = InternalServerError
							return err
						}
						session.RtcpPortServer = rtcpPort
						fmt.Printf("rtp port for audio = %v,and rtcp port = %v\n",
							rtpPort, rtcpPort)
					}
					inputPackage.ResponseInfo.SetupTransport =
						fmt.Sprintf("Transport: %v;server_port=%v-%v",
							transport, rtpPort, rtcpPort)
					inputPackage.ResponseInfo.Error = Ok
				}
			} else {
				inputPackage.ResponseInfo.Error = UnsupportedTransport
			}
		}
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
			}
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
		fmt.Printf("Writed to remote:\r\n%v", responseBuf)
	} else {
		return fmt.Errorf("WritePackage error: not find CSeq field")
	}
	return nil
}

//StartUDPServer start udp server for rtp rtcp
func (session *NetSession) StartUDPServer(packageType PackageType) (int, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return 0, err
	}
	udpConnection, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return 0, err
	}
	if err := udpConnection.SetReadBuffer(session.ReadBufferSize); err != nil {
		return 0, err
	}
	if err := udpConnection.SetWriteBuffer(session.WriteBufferSize); err != nil {
		return 0, err
	}
	if port := regexp.MustCompile(":(\\d+)").
		FindStringSubmatch(udpConnection.LocalAddr().String()); port != nil {
		if portNum, err := strconv.Atoi(port[1]); err != nil {
			return 0, err
		} else {
			return portNum, nil
		}
	} else {
		return 0, fmt.Errorf("not find udp port number in %v",
			udpConnection.LocalAddr().String())
	}
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
