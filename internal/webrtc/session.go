package webrtc

import (
	"log"
	"sync"

	pion "github.com/pion/webrtc/v4"
)

type PeerSession struct {
	SessionID      string
	PeerConnection *pion.PeerConnection

	// Function to send messages back to the Browser via WebSocket
	SendSignal func(msg interface{}) error

	DoneChan chan struct{}
	mu       sync.Mutex
}

func NewPeerSession(engine *Engine, sessionID string, sendSignal func(msg interface{}) error) (*PeerSession, error) {
	pc, err := engine.API.NewPeerConnection(engine.Config)
	if err != nil {
		return nil, err
	}

	session := &PeerSession{
		SessionID:      sessionID,
		PeerConnection: pc,
		SendSignal:     sendSignal,
		DoneChan:       make(chan struct{}),
	}

	// CALLBACK: When AetherRTC finds its own IP address, send it to the Browser
	pc.OnICECandidate(func(candidate *pion.ICECandidate) {
		if candidate == nil {
			return
		}
		// Wrap candidate in our JSON structure
		candidateJSON := candidate.ToJSON()
		msg := map[string]interface{}{
			"session_id": sessionID,
			"type":       "candidate",
			"candidate":  candidateJSON,
		}
		sendSignal(msg)
	})

	// CALLBACK: Connection State Changes
	pc.OnICEConnectionStateChange(func(state pion.ICEConnectionState) {
		log.Printf("[WebRTC %s] ICE State: %s", sessionID, state.String())
		if state == pion.ICEConnectionStateDisconnected || state == pion.ICEConnectionStateFailed || state == pion.ICEConnectionStateClosed {
			session.Close()
		}
	})

	// CALLBACK: Audio track arrives from the browser!
	pc.OnTrack(func(track *pion.TrackRemote, receiver *pion.RTPReceiver) {
		log.Printf("[WebRTC %s] INBOUND TRACK DETECTED! Codec: %s", sessionID, track.Codec().MimeType)
	})

	log.Printf("[WebRTC %s] PeerSession initialized.", sessionID)
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
