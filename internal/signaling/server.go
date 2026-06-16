package signaling

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"atherRTC/internal/webrtc"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Server holds the active WebRTC engine and active sessions
type Server struct {
	Engine   *webrtc.Engine
	Sessions map[string]*webrtc.PeerSession
	mu       sync.Mutex
}

func NewServer(engine *webrtc.Engine) *Server {
	return &Server{
		Engine:   engine,
		Sessions: make(map[string]*webrtc.PeerSession),
	}
}

func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Signaling] Upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("[Signaling] Connected: %s", r.RemoteAddr)

	// Helper to send JSON back down this specific WebSocket
	sendSignal := func(msg interface{}) error {
		return conn.WriteJSON(msg)
	}

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("[Signaling] Browser disconnected.")
			break
		}

		var sigMsg Signaling
		if err := json.Unmarshal(message, &sigMsg); err != nil {
			log.Printf("[Signaling] Invalid JSON: %v", err)
			continue
		}

		s.mu.Lock()
		session, exists := s.Sessions[sigMsg.SessionID]
		s.mu.Unlock()

		switch sigMsg.Type {
		case "offer":
			log.Printf("[Signaling] Processing OFFER for %s", sigMsg.SessionID)

			// Create a new WebRTC session
			session, err = webrtc.NewPeerSession(s.Engine, sigMsg.SessionID, sendSignal)
			if err != nil {
				log.Printf("[Signaling] Failed to create session: %v", err)
				continue
			}

			s.mu.Lock()
			s.Sessions[sigMsg.SessionID] = session
			s.mu.Unlock()

			// Generate the Answer
			if err := session.ProcessOffer(sigMsg.SDP); err != nil {
				log.Printf("[Signaling] Failed to process offer: %v", err)
			}

		case "candidate":
			if exists && sigMsg.Candidate != nil {
				if err := session.ProcessCandidate(*sigMsg.Candidate); err != nil {
					log.Printf("[Signaling] ICE Candidate error: %v", err)
				}
			}
		}
	}
}
