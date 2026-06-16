package main

import (
	"atherRTC/internal/signaling"
	"log"
	"net/http"
)

func main() {
	log.Println("[AtherRTC] Booting Edge Media Gateway .... ")

	http.HandleFunc("/ws", signaling.HandleWebSocket)

	port := ":8080"

	log.Printf("[AtherRTC] Signaling server listening on ws://localhost%s/ws", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("[AtherRTC] Server Failed : %v", err)
	}
}
