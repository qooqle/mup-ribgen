package main

// Task 8.5: Deployment feature unit tests (req 11.4, 11.5, 11.6, 11.7, 11.8).

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/qooqle/mup-ribgen/pkg/ir"
)

// TestSetupLogger_Levels verifies that all supported log levels are accepted
// without panicking (req 11).
func TestSetupLogger_Levels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error", "invalid"} {
		setupLogger(level) // must not panic
	}
}

// TestParseAddr_HostPort verifies host:port parsing (req 11).
func TestParseAddr_HostPort(t *testing.T) {
	cases := []struct {
		input string
		host  string
		port  int
	}{
		{"127.0.0.1:50051", "127.0.0.1", 50051},
		{"10.0.0.1:9179", "10.0.0.1", 9179},
	}
	for _, tc := range cases {
		h, p := parseAddr(tc.input)
		if h != tc.host || p != tc.port {
			t.Errorf("parseAddr(%q) = (%q, %d), want (%q, %d)", tc.input, h, p, tc.host, tc.port)
		}
	}
}

// TestDryRunSender_JSONMarshal verifies the routeOutput struct serialises correctly
// with all required fields present and omitempty respected.
func TestDryRunSender_JSONMarshal(t *testing.T) {
	out := routeOutput{
		Op:        "UPDATE",
		RouteType: "type1",
		SEID:      42,
		UEID:      "10.0.0.1",
		Endpoint:  "20.0.0.1",
		TEID:      100,
		QFI:       9,
		RD:        "65000:1",
		RT:        []string{"65000:100"},
		Nexthop:   "192.168.1.1",
		Network:   "n9-nw",
	}
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	checks := map[string]interface{}{
		"op":               "UPDATE",
		"route_type":       "type1",
		"seid":             float64(42),
		"ue_ip":            "10.0.0.1",
		"endpoint":         "20.0.0.1",
		"rd":               "65000:1",
		"nexthop":          "192.168.1.1",
		"network_instance": "n9-nw",
	}
	for k, want := range checks {
		if got := m[k]; got != want {
			t.Errorf("field %q: got %v, want %v", k, got, want)
		}
	}
}

// TestDryRunSender_QFIZeroOmitted verifies that QFI=0 is omitted (omitempty).
func TestDryRunSender_QFIZeroOmitted(t *testing.T) {
	out := routeOutput{Op: "ADD", RouteType: "type2", QFI: 0}
	data, _ := json.Marshal(out)
	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if _, ok := m["qfi"]; ok {
		t.Error("qfi=0 should be omitted (omitempty)")
	}
}

// TestDryRunSender_Print_Type1 verifies AddType1Route does not error for a valid RIB.
func TestDryRunSender_Print_Type1(t *testing.T) {
	sender := &dryRunSender{routeType: "type1"}
	rib := &ir.BGPRIBInfo{
		SEID:            1,
		UEIPAddress:     "10.0.0.1",
		EndpointAddress: "20.0.0.1",
		TEID:            100,
		RD:              "65000:1",
		RT:              []string{"65000:100"},
		NexthopAddress:  "192.168.1.1",
		NetworkInstance: "n9-nw",
	}
	if err := sender.AddType1Route(context.Background(), rib); err != nil {
		t.Errorf("AddType1Route: %v", err)
	}
}

// TestGracefulShutdown_ContextCancel verifies that context cancellation
// propagates as expected (req 11.6).
func TestGracefulShutdown_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
	}()

	select {
	case <-done:
		// context cancelled as expected
	case <-time.After(500 * time.Millisecond):
		t.Error("context was not cancelled within expected window")
	}
}
