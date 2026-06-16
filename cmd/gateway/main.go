package main

import (
	"log"
	"net/http"

	"atherRTC/internal/signaling"
	"atherRTC/internal/webrtc"
)

func main() {
	log.Println("[AetherRTC] Booting Edge Media Gateway...")

	// 1. Initialize the Pion WebRTC Engine
	engine := webrtc.NewEngine()

	// 2. Initialize the Signaling Server with the Engine
	sigServer := signaling.NewServer(engine)

	// 3. Mount the WebSocket route
	http.HandleFunc("/ws", sigServer.HandleWebSocket)

	port := ":8080"
	log.Printf("[AetherRTC] Listening on ws://localhost%s/ws", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("[AetherRTC] Server crashed: %v", err)
	}
}
