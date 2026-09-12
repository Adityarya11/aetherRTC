package bridge

import (
	"context"
	"fmt"
	"strings"

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
		dialTarget(orchestratorAddr),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("bridge: failed to connect to orchestrator: %v", err)
	}

	conn.Connect()

	return &Client{conn: conn}, nil
}

// grpc.NewClient defaults to the dns resolver, which blocks the first RPC on a
// TXT lookup for service config that can take over ten seconds to fail. These
// targets are always a literal host:port, so dial them directly.
func dialTarget(addr string) string {
	if strings.Contains(addr, "://") {
		return addr
	}
	return "passthrough:///" + addr
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
