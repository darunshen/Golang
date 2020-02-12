package rtsp

import (
	"fmt"
	"sync"

	"gortc.io/sdp"
)

//PusherPullersPair one pusher maps multiple pullers
type PusherPullersPair struct {
	Pusher       *RtpRtcpSession
	Pullers      []*RtpRtcpSession
	PullersMutex sync.Mutex
}

//PusherPullersSession session includes pusher and pullers
type PusherPullersSession struct {
	PusherPullersPairMap map[MediaType]*PusherPullersPair
	SdpMessage           *sdp.Message // sdp info from pusher
	SdpContent           *string      // sdp raw content
	AudioStreamName      *string      // audio stream name from sdp content
	VideoStreamName      *string      // video stream name from sdp content
}

//AddRtpRtcpSession add a rtp-rtcp-session to this pusher-pullers-session
func (session *PusherPullersSession) AddRtpRtcpSession(
	clientType ClientType, mediaType MediaType,
	rtpPort, rtcpPort, remoteIP *string) error {
	session.PusherPullersPairMap = make(map[MediaType]*PusherPullersPair)
	switch clientType {
	case PusherClient:
		if _, ok := session.PusherPullersPairMap[mediaType]; ok {
			return fmt.Errorf("pusher's request's url resource already used")
		}
		ppp := new(PusherPullersPair)
		session.PusherPullersPairMap[mediaType] = ppp
		rrs := new(RtpRtcpSession)
		if err := rrs.StartRtpRtcpSession(clientType, mediaType, nil); err != nil {
			return err
		}
		ppp.Pusher = rrs
		rtpPort = rrs.RtpServerPort
		rtcpPort = rrs.RtcpServerPort
	case PullerClient:
		ppp, ok := session.PusherPullersPairMap[mediaType]
		if !ok {
			return fmt.Errorf("puller's request's url resource not found")
		}
		rrs := new(RtpRtcpSession)
		if err := rrs.StartRtpRtcpSession(clientType, mediaType, &PullerClientInfo{
			RtpRemotePort:  rtpPort,
			RtcpRemotePort: rtcpPort,
			IPRemote:       remoteIP,
		}); err != nil {
			return err
		}
		ppp.PullersMutex.Lock()
		ppp.Pullers = append(ppp.Pullers, rrs)
		ppp.PullersMutex.Unlock()
	default:
		return fmt.Errorf("clientType error : not support")
	}
	return nil
}
