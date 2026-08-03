package bridge

import (
	"context"
	"fmt"

	gatewaypb "atherRTC/generated/gateway"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const sourceSampleRate = 8000 // G.711/PCMU, fixed by engine.go's forced codec

type Client struct {
	conn *grpc.ClientConn
}

func NewClient(orchestratorAddr string) (*Client, error) {
	conn, err := grpc.NewClient(
		orchestratorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("bridge: failed to connect to orchestrator: %v", err)
	}

	return &Client{conn: conn}, nil
}

func (c *Client) OpenSession(ctx context.Context, sessionID string) (gatewaypb.Gateway_StreamAudioClient, error) {
	client := gatewaypb.NewGatewayClient(c.conn)

	stream, err := client.StreamAudio(ctx)
	if err != nil {
		return nil, fmt.Errorf("bridge: failed to open StreamAudio: %v", err)
	}

	err = stream.Send(&gatewaypb.GatewayEvent{
		SessionId: sessionID,
		Payload: &gatewaypb.GatewayEvent_Control{
			Control: &gatewaypb.GatewayControl{
				Type:             gatewaypb.GatewayControl_START_SESSION,
				SourceSampleRate: sourceSampleRate,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("bridge: failed to send START_SESSION: %v", err)
	}

	return stream, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
