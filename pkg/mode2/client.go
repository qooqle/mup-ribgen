package mode2

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is a gRPC client for the SMFPluginService.
// It is used by the free5GC SMF integration plugin and by tests.
type Client struct {
	conn *grpc.ClientConn
}

// NewClient dials the given address and returns a Client.
func NewClient(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

// Close releases the underlying gRPC connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// ReportSession sends a session event to mup-ribgen.
func (c *Client) ReportSession(ctx context.Context, ev *SessionEvent) (*EventResponse, error) {
	out := new(EventResponse)
	err := c.conn.Invoke(ctx, "/mode2.SMFPluginService/ReportSession", ev, out,
		grpc.CallContentSubtype(CodecName),
	)
	if err != nil {
		return nil, err
	}
	return out, nil
}
