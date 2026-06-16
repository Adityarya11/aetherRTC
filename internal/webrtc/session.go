package webrtc

import (
	"log"
	"sync"

	pion "github.com/pion/webrtc/v4"
)

// this struct will manage session for one browser life-cycle
type PeerSession struct {
	SessionID string

	PeerConnection *pion.PeerConnection
	AudioTrackOut  *pion.TrackLocalStaticSample

	PCMInBoundChan  chan []byte // voice coming from the browser
	PCMOutBoundChan chan []byte // voice going to the browser

	DoneChan chan struct{}
	mu       sync.Mutex
}

func NewPeerSession(engine *Engine, session_id string) (*PeerSession, error) {
	pc, err := engine.API.NewPeerConnection(engine.Config)
	if err != nil {
		return nil, err
	}

	session := &PeerSession{
		SessionID:       session_id,
		PeerConnection:  pc,
		PCMInBoundChan:  make(chan []byte, 100),
		PCMOutBoundChan: make(chan []byte, 100),
		DoneChan:        make(chan struct{}),
	}

	// webrtc lifecycle

	pc.OnICEConnectionStateChange(func(is pion.ICEConnectionState) {
		log.Printf("[WebRTC %s] ICE state changed: %s", session_id, is.String())
		if is == pion.ICEConnectionStateDisconnected || is == pion.ICEConnectionStateFailed || is == pion.ICEConnectionStateClosed {
			session.Close()
		}
	})

	// tr = track  && r = reciever
	pc.OnTrack(func(tr *pion.TrackRemote, r *pion.RTPReceiver) {
		log.Printf("[WebRTC %s] Inbound track detected! Codec : %s", session_id, tr.Codec().MimeType)

		// this will decode the RTP packects read
	})

	log.Printf("[WebRTC %s] New PeerSession Connected ... .. ", session_id)

	return session, nil
}

func (s *PeerSession) Close() {
	s.mu.Lock()

	defer s.mu.Unlock()

	select {
	case <-s.DoneChan:
		return
	default:
	}

	log.Printf("[WebRTC %s] Closing PeerSession...", s.SessionID)
	close(s.DoneChan)

	if s.PeerConnection != nil {
		s.PeerConnection.Close()
	}

	close(s.PCMInBoundChan)
}
