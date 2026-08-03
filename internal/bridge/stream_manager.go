package bridge

import (
	"context"
	"io"
	"log"

	"atherRTC/internal/webrtc"

	gatewaypb "atherRTC/generated/gateway"
)

func RunSession(ctx context.Context, client *Client, session *webrtc.PeerSession) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := client.OpenSession(ctx, session.SessionID)
	if err != nil {
		log.Printf("[Bridge %s] Failed to open gateway session: %v", session.SessionID, err)
		return
	}

	inboundDone := make(chan struct{})
	go func() {
		defer close(inboundDone)
		for {
			select {
			case <-session.DoneChan:
				return
			case pcm := <-session.PCMInboundChan:
				err := stream.Send(&gatewaypb.GatewayEvent{
					SessionId: session.SessionID,
					Payload: &gatewaypb.GatewayEvent_Audio{
						Audio: &gatewaypb.AudioChunk{Data: pcm},
					},
				})
				if err != nil {
					log.Printf("[Bridge %s] Failed to send audio to orchestrator: %v", session.SessionID, err)
					return
				}
			}
		}
	}()

	outboundDone := make(chan struct{})
	go func() {
		droppedOutbound := 0
		defer close(outboundDone)
		for {
			event, err := stream.Recv()
			if err == io.EOF {
				log.Printf("[Bridge %s] Orchestrator closed stream.", session.SessionID)
				return
			}
			if err != nil {
				log.Printf("[Bridge %s] Recv error from orchestrator: %v", session.SessionID, err)
				return
			}

			if audio := event.GetAudio(); audio != nil {
				select {
				case session.PCMOutboundChan <- audio.Data:
				case <-session.DoneChan:
					return
				default:
					droppedOutbound++
					if droppedOutbound%50 == 0 {
						log.Printf("[Bridge %s] PCMOutboundChan full or unwired - dropped %d outbound chunks.", session.SessionID, droppedOutbound)
					}
				}
			}
		}
	}()

	select {
	case <-inboundDone:
	case <-outboundDone:
	case <-session.DoneChan:
	}

	log.Printf("[Bridge %s] Session bridge closing.", session.SessionID)
}
