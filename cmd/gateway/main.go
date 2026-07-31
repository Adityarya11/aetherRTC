package main

import (
	"log"
	"net/http"

	"atherRTC/internal/bridge"
	"atherRTC/internal/signaling"
	"atherRTC/internal/webrtc"
)

func main() {
	log.Println("[AetherRTC] Booting Edge Media Gateway...")

	engine := webrtc.NewEngine()

	bridgeClient, err := bridge.NewClient("localhost:50052")
	if err != nil {
		log.Fatalf("[AetherRTC] Failed to initialize bridge to orchestrator: %v", err)
	}
	defer bridgeClient.Close()

	sigServer := signaling.NewServer(engine, bridgeClient)

	http.HandleFunc("/ws", sigServer.HandleWebSocket)

	port := ":8080"
	log.Printf("[AetherRTC] Listening on ws://localhost%s/ws", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("[AetherRTC] Server crashed: %v", err)
	}
}
