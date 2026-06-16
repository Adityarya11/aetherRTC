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
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	m := &pion.MediaEngine{}

	// CRITICAL FIX: Only register PCMU (G.711) at 8kHz. This forces the browser
	// to downgrade from Opus to standard telephone audio, allowing pure-Go decoding.
	if err := m.RegisterCodec(pion.RTPCodecParameters{
		RTPCodecCapability: pion.RTPCodecCapability{
			MimeType:  pion.MimeTypePCMU,
			ClockRate: 8000,
			Channels:  1,
		},
		PayloadType: 0,
	}, pion.RTPCodecTypeAudio); err != nil {
		log.Fatalf("[WebRTC] Failed to register PCMU codec: %v", err)
	}

	api := pion.NewAPI(pion.WithMediaEngine(m))
	log.Println("[WebRTC] Media Engine initialized (Forcing G.711/PCMU).")

	return &Engine{
		API:    api,
		Config: config,
	}
}
