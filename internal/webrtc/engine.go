package webrtc

import (
	"log"

	pion "github.com/pion/webrtc/v4"
)

type Engine struct {
	API    *pion.API
	Config pion.Configuration
}

func NewEngine() *Engine {
	config := pion.Configuration{
		ICEServers: []pion.ICEServer{
			{
				URLs: []string{
					"stun:stun.l.google.com:19302",
					"stun:stun1.l.google.com:19302",
				},
			},
		},
	}

	m := &pion.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		log.Fatalf("[WebRTC] Failed to register codecs: %v", err)
	}

	api := pion.NewAPI(pion.WithMediaEngine(m))

	log.Println("[WebRTC] PION webrtc initialised ... .. ")

	return &Engine{
		API:    api,
		Config: config,
	}
}
