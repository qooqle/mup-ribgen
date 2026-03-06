package mode2

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"google.golang.org/grpc"

	"github.com/qooqle/mup-ribgen/pkg/ir"
)

// IRHandler is satisfied by *ir.Manager.
type IRHandler interface {
	HandleCreate(info *ir.SessionInformation) error
	HandleUpdate(info *ir.SessionInformation) error
	HandleDelete(seid uint64)
}

// Receiver is the Mode 2 gRPC server that accepts session events from SMF plugins.
type Receiver struct {
	handler IRHandler
	logger  *slog.Logger
}

// NewReceiver creates a Receiver backed by the given IRHandler.
func NewReceiver(handler IRHandler, logger *slog.Logger) *Receiver {
	return &Receiver{handler: handler, logger: logger}
}

// ReportSession implements SMFPluginServiceServer.
func (r *Receiver) ReportSession(ctx context.Context, ev *SessionEvent) (*EventResponse, error) {
	info := sessionInfoToIR(ev.Session)
	var err error
	switch ev.Type {
	case EventCreate:
		err = r.handler.HandleCreate(info)
	case EventUpdate:
		err = r.handler.HandleUpdate(info)
	case EventDelete:
		r.handler.HandleDelete(info.SEID)
	default:
		err = fmt.Errorf("unknown event type: %d", ev.Type)
	}
	if err != nil {
		r.logger.Error("mode2: ReportSession failed",
			"type", ev.Type, "seid", ev.Session.SEID, "err", err)
		return &EventResponse{Accepted: false, ErrorMessage: err.Error()}, nil
	}
	return &EventResponse{Accepted: true}, nil
}

// ListenAndServe starts the gRPC server on addr and blocks until ctx is done.
func (r *Receiver) ListenAndServe(ctx context.Context, addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("mode2 listen %s: %w", addr, err)
	}
	srv := grpc.NewServer()
	RegisterSMFPluginServiceServer(srv, r)
	go func() {
		<-ctx.Done()
		srv.GracefulStop()
	}()
	r.logger.Info("mode2: gRPC server listening", "addr", addr)
	return srv.Serve(lis)
}

func sessionInfoToIR(s SessionInfo) *ir.SessionInformation {
	return &ir.SessionInformation{
		SessionID:       s.SessionID,
		SEID:            s.SEID,
		UEIPAddress:     s.UEIPAddress,
		UEPrefix:        s.UEPrefix,
		TEID:            s.TEID,
		QFI:             s.QFI,
		EndpointAddress: s.EndpointAddress,
		NetworkInstance: s.NetworkInstance,
		Source:          ir.Mode2Plugin,
	}
}
