package webrtc

import (
	"log"
	"sync"

	"atherRTC/pkg/codec"

	pion "github.com/pion/webrtc/v4"
)

type PeerSession struct {
	SessionID      string
	PeerConnection *pion.PeerConnection
	SendSignal     func(msg interface{}) error

	PCMInboundChan  chan []byte
	PCMOutboundChan chan []byte

	DoneChan chan struct{}
	mu       sync.Mutex
}

func NewPeerSession(engine *Engine, sessionID string, sendSignal func(msg interface{}) error) (*PeerSession, error) {
	pc, err := engine.API.NewPeerConnection(engine.Config)
	if err != nil {
		return nil, err
	}

	session := &PeerSession{
		SessionID:       sessionID,
		PeerConnection:  pc,
		SendSignal:      sendSignal,
		PCMInboundChan:  make(chan []byte, 100),
		PCMOutboundChan: make(chan []byte, 100),
		DoneChan:        make(chan struct{}),
	}

	pc.OnICECandidate(func(candidate *pion.ICECandidate) {
		if candidate != nil {
			sendSignal(map[string]interface{}{
				"session_id": sessionID,
				"type":       "candidate",
				"candidate":  candidate.ToJSON(),
			})
		}
	})

	pc.OnICEConnectionStateChange(func(state pion.ICEConnectionState) {
		log.Printf("[WebRTC %s] ICE State: %s", sessionID, state.String())
		if state == pion.ICEConnectionStateDisconnected || state == pion.ICEConnectionStateClosed {
			session.Close()
		}
	})

	pc.OnTrack(func(track *pion.TrackRemote, receiver *pion.RTPReceiver) {
		log.Printf("[WebRTC %s] INBOUND TRACK DETECTED! Codec: %s", sessionID, track.Codec().MimeType)

		go func() {
			droppedCount := 0
			for {
				rtpPacket, _, err := track.ReadRTP()
				if err != nil {
					log.Printf("[WebRTC %s] Audio track stream ended.", sessionID)
					return
				}

				pcmBytes := codec.DecodeUlaw(rtpPacket.Payload)

				select {
				case session.PCMInboundChan <- pcmBytes:
				default:
					droppedCount++
					if droppedCount%50 == 0 {
						log.Printf("[WebRTC %s] PCMInboundChan full — dropped %d packets.", sessionID, droppedCount)
					}
				}
			}
		}()
	})

	return session, nil
}

// ProcessOffer takes the browser's SDP, sets it, and generates an Answer
func (s *PeerSession) ProcessOffer(sdp string) error {
	offer := pion.SessionDescription{
		Type: pion.SDPTypeOffer,
		SDP:  sdp,
	}

	if err := s.PeerConnection.SetRemoteDescription(offer); err != nil {
		return err
	}

	answer, err := s.PeerConnection.CreateAnswer(nil)
	if err != nil {
		return err
	}

	if err := s.PeerConnection.SetLocalDescription(answer); err != nil {
		return err
	}

	// Send the answer back to the browser
	msg := map[string]interface{}{
		"session_id": s.SessionID,
		"type":       "answer",
		"sdp":        answer.SDP,
	}
	return s.SendSignal(msg)
}

// ProcessCandidate adds the browser's IP address to our WebRTC engine
func (s *PeerSession) ProcessCandidate(candidate pion.ICECandidateInit) error {
	return s.PeerConnection.AddICECandidate(candidate)
}

func (s *PeerSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.DoneChan:
		return
	default:
	}

	log.Printf("[WebRTC %s] Closing session.", s.SessionID)
	close(s.DoneChan)
	if s.PeerConnection != nil {
		s.PeerConnection.Close()
	}
}
