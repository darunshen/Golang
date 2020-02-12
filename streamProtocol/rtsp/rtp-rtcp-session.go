package rtsp

import (
	"fmt"
	"net"
	"regexp"
)

//MediaType the media type of this session
type MediaType int

const (
	//MediaVideo stand for video
	MediaVideo MediaType = 0
	//MediaAudio stand for audio
	MediaAudio MediaType = 1
)

//ClientType rtsp client type(remote end point of connection)func (session *RtpRtcpSession)
type ClientType int

const (
	//PusherClient the rtsp client is pusher
	PusherClient ClientType = 0
	//PullerClient the rtsp client is puller
	PullerClient ClientType = 1
)

//RtpRtcpSession a pair of rtp-rtcp sessions
type RtpRtcpSession struct {
	RtpUDPConnToPuller  *net.UDPConn // rtp udp connection to puller
	RtcpUDPConnToPuller *net.UDPConn // rtcp udp connection to puller
	RtpUDPConnToPusher  *net.UDPConn // rtp udp connection to pusher
	RtcpUDPConnToPusher *net.UDPConn // rtcp udp connection to pusher
	RtpServerPort       *string      // rtp Server port in udp session
	RtcpServerPort      *string      // rtcp Server port in udp session
	SessionMediaType    MediaType    // this session's media type
	SessionClientType   ClientType   // this session's client type(connected to pusher or puller)
}

//PullerClientInfo the puller 's info as input to create rtp/rtcp
type PullerClientInfo struct {
	RtpRemotePort, RtcpRemotePort, IPRemote *string
}

//StartRtpRtcpSession Start a pair of rtp-rtcp sessions
func (session *RtpRtcpSession) StartRtpRtcpSession(
	clientType ClientType, mediaType MediaType, pullerClientInfo *PullerClientInfo) error {
	var err error
	switch clientType {
	case PusherClient:
		session.RtpUDPConnToPusher, session.RtpServerPort, err =
			session.startUDPServer()
		if err != nil {
			return fmt.Errorf("startUDPServer failed : %v", err)
		}
		session.RtcpUDPConnToPusher, session.RtcpServerPort, err =
			session.startUDPServer()
		if err != nil {
			return fmt.Errorf("startUDPServer failed : %v", err)
		}
		fmt.Printf("rtp server port for video = %v,and rtcp port = %v\n",
			session.RtpUDPConnToPusher, session.RtcpUDPConnToPusher)
	case PullerClient:
		if pullerClientInfo == nil {
			return fmt.Errorf("StartRtpRtcpSession :pullerClientInfo is nil")
		}
		session.RtpUDPConnToPuller, err = session.startUDPClient(
			pullerClientInfo.IPRemote, pullerClientInfo.RtpRemotePort)
		if err != nil {
			return fmt.Errorf("startUDPClient failed : %v", err)
		}
		session.RtcpUDPConnToPuller, err = session.startUDPClient(
			pullerClientInfo.IPRemote, pullerClientInfo.RtcpRemotePort)
		if err != nil {
			return fmt.Errorf("startUDPClient failed : %v", err)
		}
	default:
		return fmt.Errorf("clientType error,not support")
	}
	session.SessionMediaType = mediaType
	session.SessionClientType = clientType
	return nil
}

//startUDPServer start udp server for rtp/rtcp,
func (session *RtpRtcpSession) startUDPServer() (*net.UDPConn, *string, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return nil, nil, err
	}
	udpConnection, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, nil, err
	}
	if err := udpConnection.SetReadBuffer(ReadBufferSize); err != nil {
		return nil, nil, err
	}
	if err := udpConnection.SetWriteBuffer(WriteBufferSize); err != nil {
		return nil, nil, err
	}
	port := regexp.MustCompile(":(\\d+)").
		FindStringSubmatch(udpConnection.LocalAddr().String())
	if port != nil {
		return udpConnection, &port[1], nil
	}
	return nil, nil, fmt.Errorf("not find udp port number in %v",
		udpConnection.LocalAddr().String())

}

//startUDPClient start udp connection to puller
func (session *RtpRtcpSession) startUDPClient(ip, port *string) (*net.UDPConn, error) {
	host := *ip + ":" + *port
	udpAddr, err := net.ResolveUDPAddr("udp", host)
	if err != nil {
		return nil, err
	}
	udpConnection, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}
	if err := udpConnection.SetReadBuffer(ReadBufferSize); err != nil {
		return nil, err
	}
	if err := udpConnection.SetWriteBuffer(WriteBufferSize); err != nil {
		return nil, err
	}
	return udpConnection, nil
}
