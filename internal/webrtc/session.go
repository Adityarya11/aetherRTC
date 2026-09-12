package webrtc

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"atherRTC/pkg/codec"

	pion "github.com/pion/webrtc/v4"
	pionmedia "github.com/pion/webrtc/v4/pkg/media"
)

type PeerSession struct {
	SessionID      string
	PeerConnection *pion.PeerConnection
	SendSignal     func(msg interface{}) error

	PCMInboundChan  chan []byte
	PCMOutboundChan chan []byte

	OutboundTrack *pion.TrackLocalStaticSample
	AgentSpeaking atomic.Bool

	DoneChan chan struct{}
	mu       sync.Mutex
}

func NewPeerSession(engine *Engine, sessionID string, sendSignal func(msg interface{}) error) (*PeerSession, error) {
	pc, err := engine.API.NewPeerConnection(engine.Config)
	if err != nil {
		return nil, err
	}

	outboundTrack, err := pion.NewTrackLocalStaticSample(
		pion.RTPCodecCapability{
			MimeType:  pion.MimeTypePCMU,
			ClockRate: 8000,
			Channels:  1,
		},
		"audio",
		"aetherrtc-"+sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create outbound track: %v", err)
	}

	rtpSender, err := pc.AddTrack(outboundTrack)
	if err != nil {
		return nil, fmt.Errorf("failed to add outbound track: %v", err)
	}

	go func() {
		for {
			if _, _, err := rtpSender.ReadRTCP(); err != nil {
				return
			}
		}
	}()

	session := &PeerSession{
		SessionID:       sessionID,
		PeerConnection:  pc,
		SendSignal:      sendSignal,
		PCMInboundChan:  make(chan []byte, 100),
		PCMOutboundChan: make(chan []byte, 100),
		OutboundTrack:   outboundTrack,
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

				if session.AgentSpeaking.Load() {
					continue
				}

				pcmBytes := codec.DecodeUlaw(rtpPacket.Payload)

				select {
				case session.PCMInboundChan <- pcmBytes:
				default:
					droppedCount++
					if droppedCount%50 == 0 {
						log.Printf("[WebRTC %s] PCMInboundChan full - dropped %d packets.", sessionID, droppedCount)
					}
				}
			}
		}()
	})

	go func() {
		const (
			pcmFrameBytes = 320 // 20ms of 8kHz, 16-bit mono PCM
			frameDuration = 20 * time.Millisecond
			// Held past the last emitted frame so gaps in TTS delivery between
			// sentences do not read as the agent having stopped speaking.
			speakingHangover = 400 * time.Millisecond
			maxCatchUpFrames = 5
		)

		ticker := time.NewTicker(frameDuration)
		defer ticker.Stop()

		pcmBuffer := make([]byte, 0, pcmFrameBytes*64)
		var nextFrameAt time.Time
		speaking := false

		for {
			select {
			case <-session.DoneChan:
				return
			case <-ticker.C:
			}

			for draining := true; draining; {
				select {
				case pcm := <-session.PCMOutboundChan:
					pcmBuffer = append(pcmBuffer, pcm...)
				default:
					draining = false
				}
			}

			now := time.Now()

			if !speaking && len(pcmBuffer) >= pcmFrameBytes {
				speaking = true
				nextFrameAt = now
				session.AgentSpeaking.Store(true)
			}

			for emitted := 0; speaking &&
				len(pcmBuffer) >= pcmFrameBytes &&
				!now.Before(nextFrameAt) &&
				emitted < maxCatchUpFrames; emitted++ {

				ulawData := codec.EncodeUlaw(pcmBuffer[:pcmFrameBytes])
				if err := outboundTrack.WriteSample(pionmedia.Sample{
					Data:     ulawData,
					Duration: frameDuration,
				}); err != nil {
					log.Printf("[WebRTC %s] Failed to write outbound sample: %v", sessionID, err)
				}

				pcmBuffer = append(pcmBuffer[:0], pcmBuffer[pcmFrameBytes:]...)
				nextFrameAt = nextFrameAt.Add(frameDuration)
			}

			if speaking && len(pcmBuffer) < pcmFrameBytes && now.Sub(nextFrameAt) >= speakingHangover {
				speaking = false
				session.AgentSpeaking.Store(false)
			}
		}
	}()

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
