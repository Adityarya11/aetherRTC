package signaling

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all origin -> This is a development feature.
	},
}

func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Signaling] Failed to upgrade the connection: %v", err)
		return
	}

	defer conn.Close()

	log.Printf("[Signaling] New browser connection established from : %s", r.RemoteAddr)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[Signaling] Error in reading message: %v", err)
			}

			log.Printf("[Signaling] Browser Disconnected")
			break
		}

		var sigMsg Signaling
		if err := json.Unmarshal(message, &sigMsg); err != nil {
			log.Printf("[Signaling] Invalid Json recieved: %v", err)
			continue
		}

		log.Printf("[Signaling] Recieved %s from Session: %s", sigMsg.Type, sigMsg.SessionID)

	}
}
