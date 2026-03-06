package free5gc

import (
	"context"
	"log/slog"

	"github.com/qooqle/mup-ribgen/pkg/mode2"
)

// MUPClient sends session events to mup-ribgen via gRPC.
// It is safe for concurrent use.
type MUPClient struct {
	client *mode2.Client
	logger *slog.Logger
}

// NewMUPClient creates a MUPClient connected to the given mup-ribgen address
// (e.g., "127.0.0.1:9182"). Call Close when done.
func NewMUPClient(addr string, logger *slog.Logger) (*MUPClient, error) {
	c, err := mode2.NewClient(addr)
	if err != nil {
		return nil, err
	}
	return &MUPClient{client: c, logger: logger}, nil
}

// Close releases the underlying gRPC connection.
func (m *MUPClient) Close() error {
	return m.client.Close()
}

// OnSessionEstablished is called when free5GC SMF establishes a PDU session.
func (m *MUPClient) OnSessionEstablished(s SessionData) {
	m.report(mode2.EventCreate, s)
}

// OnSessionModified is called when free5GC SMF modifies a PDU session.
func (m *MUPClient) OnSessionModified(s SessionData) {
	m.report(mode2.EventUpdate, s)
}

// OnSessionDeleted is called when free5GC SMF deletes a PDU session.
func (m *MUPClient) OnSessionDeleted(s SessionData) {
	m.report(mode2.EventDelete, s)
}

func (m *MUPClient) report(t mode2.EventType, s SessionData) {
	ev := &mode2.SessionEvent{
		Type: t,
		Session: mode2.SessionInfo{
			SessionID:       s.SessionID,
			SEID:            s.SEID,
			UEIPAddress:     s.UEIPAddress,
			UEPrefix:        s.UEPrefix,
			TEID:            s.TEID,
			QFI:             s.QFI,
			EndpointAddress: s.EndpointAddress,
			NetworkInstance: s.NetworkInstance,
		},
	}
	resp, err := m.client.ReportSession(context.Background(), ev)
	if err != nil {
		m.logger.Error("free5gc plugin: ReportSession failed",
			"type", t, "seid", s.SEID, "err", err)
		return
	}
	if !resp.Accepted {
		m.logger.Warn("free5gc plugin: event not accepted",
			"type", t, "seid", s.SEID, "msg", resp.ErrorMessage)
	}
}
