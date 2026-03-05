package bgp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	gobgpapi "github.com/osrg/gobgp/v3/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/qooqle/mup-ribgen/pkg/ir"
)

// Config holds the configuration for the GoBGP gRPC client (req 8.1).
type Config struct {
	// Address is the GoBGP daemon host (default "127.0.0.1").
	Address string
	// Port is the GoBGP gRPC port (default 50051).
	Port int
	// DialTimeout is the maximum time to wait for connection (default 5s).
	DialTimeout time.Duration
	// MaxRetries is the number of retry attempts on transient errors (default 3).
	MaxRetries int
}

func (c *Config) defaults() {
	if c.Address == "" {
		c.Address = "127.0.0.1"
	}
	if c.Port == 0 {
		c.Port = 50051
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = 5 * time.Second
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
}

// Client manages the gRPC connection to a GoBGP daemon and provides
// MUP SAFI route operations (req 8.1).
type Client struct {
	cfg   Config
	mu    sync.Mutex
	conn  *grpc.ClientConn
	gobgp gobgpapi.GobgpApiClient
}

// NewClient creates a Client with the given configuration.
func NewClient(cfg Config) *Client {
	cfg.defaults()
	return &Client{cfg: cfg}
}

// Connect establishes the gRPC connection to the GoBGP daemon.
// It is idempotent: calling Connect on an already-connected Client is a no-op.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil // already connected
	}

	target := fmt.Sprintf("%s:%d", c.cfg.Address, c.cfg.Port)
	dialCtx, cancel := context.WithTimeout(ctx, c.cfg.DialTimeout)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, target, //nolint:staticcheck
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("bgp: dial %s: %w", target, err)
	}

	c.conn = conn
	c.gobgp = gobgpapi.NewGobgpApiClient(conn)
	slog.Info("bgp: connected to GoBGP daemon", "target", target)
	return nil
}

// Close terminates the gRPC connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	c.gobgp = nil
	return err
}

// AddType1Route sends a Type_1_Session_Transformed_Route to GoBGP (req 8.2, 8.6).
func (c *Client) AddType1Route(ctx context.Context, rib *ir.BGPRIBInfo) error {
	req, err := BuildType1Route(rib)
	if err != nil {
		return err
	}
	return c.withRetry(ctx, func() error {
		_, err := c.gobgp.AddPath(ctx, req)
		return err
	})
}

// AddType2Route sends a Type_2_Session_Transformed_Route to GoBGP (req 8.2, 8.7).
func (c *Client) AddType2Route(ctx context.Context, rib *ir.BGPRIBInfo) error {
	req, err := BuildType2Route(rib)
	if err != nil {
		return err
	}
	return c.withRetry(ctx, func() error {
		_, err := c.gobgp.AddPath(ctx, req)
		return err
	})
}

// UpdateType1Route updates a Type_1_Session_Transformed_Route in GoBGP (req 8.3).
// GoBGP replaces an existing path with the same NLRI key on AddPath.
func (c *Client) UpdateType1Route(ctx context.Context, rib *ir.BGPRIBInfo) error {
	return c.AddType1Route(ctx, rib)
}

// UpdateType2Route updates a Type_2_Session_Transformed_Route in GoBGP (req 8.3).
func (c *Client) UpdateType2Route(ctx context.Context, rib *ir.BGPRIBInfo) error {
	return c.AddType2Route(ctx, rib)
}

// DeleteType1Route removes a Type_1_Session_Transformed_Route from GoBGP (req 8.4).
func (c *Client) DeleteType1Route(ctx context.Context, rib *ir.BGPRIBInfo) error {
	req, err := BuildDeleteType1Route(rib)
	if err != nil {
		return err
	}
	return c.withRetry(ctx, func() error {
		_, err := c.gobgp.DeletePath(ctx, req)
		return err
	})
}

// DeleteType2Route removes a Type_2_Session_Transformed_Route from GoBGP (req 8.4).
func (c *Client) DeleteType2Route(ctx context.Context, rib *ir.BGPRIBInfo) error {
	req, err := BuildDeleteType2Route(rib)
	if err != nil {
		return err
	}
	return c.withRetry(ctx, func() error {
		_, err := c.gobgp.DeletePath(ctx, req)
		return err
	})
}

// withRetry executes fn up to MaxRetries times on error, with exponential backoff.
// It logs a warning after each failure (req 9.5).
func (c *Client) withRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 500 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
		if err := fn(); err != nil {
			lastErr = err
			slog.Warn("bgp: GoBGP RPC error, will retry",
				"attempt", attempt+1, "max", c.cfg.MaxRetries, "err", err)
			continue
		}
		return nil
	}
	return fmt.Errorf("bgp: operation failed after %d retries: %w", c.cfg.MaxRetries, lastErr)
}
