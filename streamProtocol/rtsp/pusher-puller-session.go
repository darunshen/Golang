package rtsp

import (
	"fmt"
	"sync"

	"gortc.io/sdp"
)

//PusherPullersPair one pusher maps multiple pullers
type PusherPullersPair struct {
	Pusher          *RtpRtcpSession
	Pullers         []*RtpRtcpSession
	PullersMutex    sync.Mutex
	rtpPackageChan  chan RtpRtcpPackage
	rtcpPackageChan chan RtpRtcpPackage
	IfStop          bool
}

//PusherPullersSession session includes pusher and pullers
type PusherPullersSession struct {
	PusherPullersPairMap map[MediaType]*PusherPullersPair // has vidio and audio
	SdpMessage           *sdp.Message                     // sdp info from pusher
	SdpContent           *string                          // sdp raw content
	AudioStreamName      *string                          // audio stream name from sdp content
	VideoStreamName      *string                          // video stream name from sdp content
}

//AddRtpRtcpSession add a rtp-rtcp-session to this pusher-pullers-session
func (session *PusherPullersSession) AddRtpRtcpSession(
	clientType ClientType, mediaType MediaType,
	rtpPort, rtcpPort, remoteIP *string) error {
	if session.PusherPullersPairMap == nil {
		session.PusherPullersPairMap = make(map[MediaType]*PusherPullersPair)
	}
	switch clientType {
	case PusherClient:
		if _, ok := session.PusherPullersPairMap[mediaType]; ok {
			return fmt.Errorf("pusher's request's url resource already used")
		}
		ppp := new(PusherPullersPair)
		ppp.rtpPackageChan = make(chan RtpRtcpPackage, PushChannelBufferSize)
		ppp.rtcpPackageChan = make(chan RtpRtcpPackage, PullChannelBufferSize)
		session.PusherPullersPairMap[mediaType] = ppp
		rrs := new(RtpRtcpSession)
		if err := rrs.StartRtpRtcpSession(clientType, mediaType, nil); err != nil {
			return err
		}
		if err := rrs.BeginTransfer(clientType,
			ppp.rtpPackageChan,
			ppp.rtcpPackageChan); err != nil {
			return err
		}
		if err := ppp.Start(); err != nil {
			return err
		}
		ppp.Pusher = rrs
		*rtpPort = *rrs.RtpServerPort
		*rtcpPort = *rrs.RtcpServerPort
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
		if err := rrs.BeginTransfer(clientType, nil, nil); err != nil {
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

//Start start package(rtp/rtcp) transfer from pusher to pullers
func (session *PusherPullersPair) Start() error {
	go func() {
		for !session.IfStop {
			data := <-session.rtpPackageChan
			for _, puller := range session.Pullers {
				puller.RtpPackageChannel <- &data
			}
		}
	}()
	go func() {
		for !session.IfStop {
			data := <-session.rtcpPackageChan
			for _, puller := range session.Pullers {
				puller.RtcpPackageChannel <- &data
			}
		}
	}()
	return nil
}

//StopSession stop this session and goroutines created by this
func (session *PusherPullersSession) StopSession() []error {
	returnErr := make([]error, 1)
	for _, ppp := range session.PusherPullersPairMap {
		if err := ppp.Stop(); len(err) != 0 {
			returnErr = append(returnErr, err...)
		}
	}
	return returnErr
}

//Stop stop package(rtp/rtcp) transfer from pusher to pullers
func (session *PusherPullersPair) Stop() []error {
	returnErr := make([]error, 1)
	if err := session.Pusher.StopTransfer(); err != nil {
		returnErr = append(returnErr, err)
	}
	for _, puller := range session.Pullers {
		if err := puller.StopTransfer(); err != nil {
			returnErr = append(returnErr, err)
		}
	}
	session.IfStop = true
	return returnErr
}
