// Package mode1 implements Mode-1 passive PFCP sniffing.
// It connects the pipeline: PFCP Sniffer → PFCP Parser → Dialect Transformer
// → PFCP Session State Manager, producing SessionInformation events (req 2.1, 2.2).
package mode1

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/qooqle/mup-ribgen/pkg/ir"
	"github.com/qooqle/mup-ribgen/pkg/pfcp"
)

// SessionEvent is emitted by the Controller for each significant session change.
type SessionEvent struct {
	// Info is the updated SessionInformation; nil for deletion events.
	Info *ir.SessionInformation
	// SEID is always set (even for deletion events where Info is nil).
	SEID uint64
	// Type describes the event ("establishment", "modification", "deletion").
	Type string
}

// Config holds the runtime configuration for a Mode-1 Controller.
type Config struct {
	// Interface is the network interface to sniff (e.g. "eth0").
	Interface string
	// ChannelBuffer controls how many SessionEvents are buffered (req 2.2).
	ChannelBuffer int
}

// Controller is the Mode-1 entry point. It owns the full pipeline:
//
//	Sniffer → Parser → SessionManager
//
// and emits SessionEvent objects on the Events() channel for downstream
// consumers (e.g. the IR Manager).
type Controller struct {
	cfg     Config
	sniffer pfcp.Sniffer
	sm      *pfcp.SessionManager
	events  chan *SessionEvent
	cancel  context.CancelFunc
}

// New creates a Controller with the given config, sniffer, and transformer.
func New(cfg Config, sniffer pfcp.Sniffer, transformer pfcp.Transformer) *Controller {
	if cfg.ChannelBuffer <= 0 {
		cfg.ChannelBuffer = 128
	}
	return &Controller{
		cfg:     cfg,
		sniffer: sniffer,
		sm:      pfcp.NewSessionManager(transformer),
		events:  make(chan *SessionEvent, cfg.ChannelBuffer),
	}
}

// Events returns the channel on which SessionEvent pointers are sent.
// The channel is closed when the Controller stops.
func (c *Controller) Events() <-chan *SessionEvent { return c.events }

// Start begins the capture pipeline. It launches the sniffer and a processing
// goroutine and returns immediately (non-blocking, req 2.2).
// Call Stop or cancel the provided context to shut down gracefully.
func (c *Controller) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	rawCh, err := c.sniffer.Start(ctx)
	if err != nil {
		cancel()
		return fmt.Errorf("mode1: sniffer start: %w", err)
	}
	go c.process(ctx, rawCh)
	return nil
}

// Stop terminates the capture pipeline and closes the Events channel.
func (c *Controller) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.sniffer.Stop()
}

// process is the goroutine that runs the Parse → Transform → Manage pipeline.
func (c *Controller) process(ctx context.Context, rawCh <-chan *pfcp.RawPacket) {
	defer close(c.events)
	for {
		select {
		case <-ctx.Done():
			return
		case pkt, ok := <-rawCh:
			if !ok {
				return
			}
			c.handlePacket(pkt)
		}
	}
}

// handlePacket parses one raw packet and routes it to the session manager.
func (c *Controller) handlePacket(pkt *pfcp.RawPacket) {
	msg, err := pfcp.ParseMessage(pkt.Data)
	if err != nil {
		slog.Debug("mode1: parse error", "err", err)
		return // req 9.4: continue processing after parse errors
	}

	switch msg.Type {
	case pfcp.MsgTypeSessionEstablishmentRequest:
		info, err := c.sm.HandleEstablishment(msg.ToEstablishmentRequest())
		if err != nil {
			slog.Warn("mode1: establishment error", "seid", msg.SEID, "err", err)
			return
		}
		c.emit(&SessionEvent{Info: info, SEID: info.SEID, Type: "establishment"})

	case pfcp.MsgTypeSessionModificationRequest:
		info, err := c.sm.HandleModification(msg.ToModificationRequest())
		if err != nil {
			slog.Warn("mode1: modification error", "seid", msg.SEID, "err", err)
			return
		}
		c.emit(&SessionEvent{Info: info, SEID: info.SEID, Type: "modification"})

	case pfcp.MsgTypeSessionEstablishmentResponse:
		// In passive sniffing mode we observe both request and response.
		// The response header carries cpSEID; the F-SEID IE carries upSEID.
		// Register the alias so subsequent Modification/Deletion (which use upSEID) resolve correctly.
		cpSEID := msg.SEID
		if pfcpFields, ok := msg.Fields["pfcp"].(map[string]interface{}); ok {
			if fSEID, ok := pfcpFields["f_seid"].(map[string]interface{}); ok {
				if upSEID, ok := fSEID["seid"].(uint64); ok && upSEID != 0 {
					c.sm.RegisterSEIDAlias(upSEID, cpSEID)
					slog.Debug("mode1: SEID alias registered", "cp_seid", cpSEID, "up_seid", upSEID)
				}
			}
		}

	case pfcp.MsgTypeSessionDeletionRequest:
		req := msg.ToDeletionRequest()
		c.sm.HandleDeletion(req)
		c.emit(&SessionEvent{SEID: req.SEID, Type: "deletion"})

	default:
		// heartbeats, association messages, responses – ignored
	}
}

// emit sends a SessionEvent on the events channel without blocking.
// If the channel buffer is full, the event is dropped and a warning is logged.
func (c *Controller) emit(ev *SessionEvent) {
	select {
	case c.events <- ev:
	default:
		slog.Warn("mode1: event channel full, dropping event",
			"seid", ev.SEID, "type", ev.Type)
	}
}
