package mode2_test

// Task 7.6: Mode 2 end-to-end integration test (req 3.1-3.5).
// Exercises the full gRPC path: Client.ReportSession → Receiver.ReportSession → IRHandler.

import (
	"context"
	"log/slog"
	"net"
	"testing"

	"google.golang.org/grpc"

	"github.com/qooqle/mup-ribgen/pkg/mode2"
)

// startTestServer starts a Receiver on a random port and returns its address and a cleanup func.
func startTestServer(t *testing.T, handler mode2.IRHandler) (addr string, stop func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	recv := mode2.NewReceiver(handler, slog.Default())
	srv := grpc.NewServer()
	mode2.RegisterSMFPluginServiceServer(srv, recv)
	go func() { _ = srv.Serve(lis) }()
	return lis.Addr().String(), srv.GracefulStop
}

// TestIntegration_Mode2_Create verifies end-to-end CREATE over real gRPC (req 3.1, 3.3).
func TestIntegration_Mode2_Create(t *testing.T) {
	handler := &mockIRHandler{}
	addr, stop := startTestServer(t, handler)
	defer stop()

	client, err := mode2.NewClient(addr)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	ev := &mode2.SessionEvent{
		Type: mode2.EventCreate,
		Session: mode2.SessionInfo{
			SessionID:       "sess-1",
			SEID:            0xABCD,
			UEIPAddress:     "10.0.0.1",
			NetworkInstance: "internet",
		},
	}
	resp, err := client.ReportSession(context.Background(), ev)
	if err != nil {
		t.Fatalf("ReportSession: %v", err)
	}
	if !resp.Accepted {
		t.Fatalf("not accepted: %s", resp.ErrorMessage)
	}
	if len(handler.created) != 1 || handler.created[0].SessionID != "sess-1" {
		t.Errorf("expected 1 create with sess-1, got %v", handler.created)
	}
}

// TestIntegration_Mode2_UpdateDelete verifies UPDATE and DELETE over real gRPC (req 3.4, 3.5).
func TestIntegration_Mode2_UpdateDelete(t *testing.T) {
	handler := &mockIRHandler{}
	addr, stop := startTestServer(t, handler)
	defer stop()

	client, err := mode2.NewClient(addr)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	update := &mode2.SessionEvent{
		Type:    mode2.EventUpdate,
		Session: mode2.SessionInfo{SEID: 42, SessionID: "s42"},
	}
	if resp, err := client.ReportSession(ctx, update); err != nil || !resp.Accepted {
		t.Fatalf("update: err=%v accepted=%v", err, resp)
	}

	del := &mode2.SessionEvent{
		Type:    mode2.EventDelete,
		Session: mode2.SessionInfo{SEID: 42},
	}
	if resp, err := client.ReportSession(ctx, del); err != nil || !resp.Accepted {
		t.Fatalf("delete: err=%v accepted=%v", err, resp)
	}

	if len(handler.updated) != 1 || handler.updated[0].SEID != 42 {
		t.Errorf("expected 1 update for SEID=42, got %v", handler.updated)
	}
	if len(handler.deleted) != 1 || handler.deleted[0] != 42 {
		t.Errorf("expected 1 delete for SEID=42, got %v", handler.deleted)
	}
}
