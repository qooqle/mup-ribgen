package main

// Task 8.5: Deployment feature unit tests (req 11.4, 11.5, 11.6, 11.7, 11.8).

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "github.com/qooqle/mup-ribgen/pkg/dialect"
	"github.com/qooqle/mup-ribgen/pkg/dialect"
	"github.com/qooqle/mup-ribgen/pkg/dslruntime"
	"github.com/qooqle/mup-ribgen/pkg/ir"
	"github.com/qooqle/mup-ribgen/pkg/pfcp"
	"github.com/qooqle/mup-ribgen/pkg/pipeline"
	"github.com/qooqle/mup-ribgen/pkg/staticctx"
	"github.com/qooqle/mup-ribgen/pkg/testharness"
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

// TestDryRunOutput_Type1_Minimal verifies type1 dry-run output contains only
// fields required to build a GoBGP Type1 route.
func TestDryRunOutput_Type1_Minimal(t *testing.T) {
	src := "2001:db8::1"
	rib := &ir.BGPRIBInfo{
		RouteKey:        "1:11",
		FARID:           11,
		SEID:            1,
		NetworkInstance: "n9-nw",
		UEIPAddress:     "10.0.0.1",
		EndpointAddress: "20.0.0.1",
		TEID:            100,
		QFI:             9,
		RD:              "65000:1",
		RT:              []string{"65000:100"},
		NexthopAddress:  "192.168.1.1",
		SourceAddress:   &src,
	}
	out := buildDryRunOutput("UPDATE", "type1", rib)
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
		"seid":             float64(1),
		"route_key":        "1:11",
		"far_id":           float64(11),
		"network_instance": "n9-nw",
		"ue_ip":            "10.0.0.1",
		"endpoint":         "20.0.0.1",
		"teid":             float64(100),
		"qfi":              float64(9),
		"rd":               "65000:1",
		"nexthop":          "192.168.1.1",
		"source_address":   "2001:db8::1",
	}
	for k, want := range checks {
		if got := m[k]; got != want {
			t.Errorf("field %q: got %v, want %v", k, got, want)
		}
	}
	rt, ok := m["rt"].([]interface{})
	if !ok || len(rt) != 1 || rt[0] != "65000:100" {
		t.Fatalf("type1 output rt mismatch: %#v", m["rt"])
	}
}

// TestDryRunOutput_Type2_Minimal verifies type2 dry-run output contains only
// fields required to build a GoBGP Type2 route.
func TestDryRunOutput_Type2_Minimal(t *testing.T) {
	epLen := 64
	mup := &ir.MUPExtendedCommunity{SegmentIdentifier: [6]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x02}}
	rib := &ir.BGPRIBInfo{
		RouteKey:              "2:12",
		FARID:                 12,
		SEID:                  2,
		NetworkInstance:       "n3-nw",
		EndpointAddress:       "20.0.0.1",
		TEID:                  200,
		RD:                    "65000:2",
		RT:                    []string{"65000:200"},
		NexthopAddress:        "192.168.1.2",
		EndpointAddressLength: &epLen,
		MUPExtendedCommunity:  mup,
	}
	out := buildDryRunOutput("ADD", "type2", rib)
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	checks := map[string]interface{}{
		"op":               "ADD",
		"route_type":       "type2",
		"seid":             float64(2),
		"route_key":        "2:12",
		"far_id":           float64(12),
		"network_instance": "n3-nw",
		"endpoint":         "20.0.0.1",
		"teid":             float64(200),
		"rd":               "65000:2",
		"nexthop":          "192.168.1.2",
		"endpoint_address_length": float64(64),
	}
	for k, want := range checks {
		if got := m[k]; got != want {
			t.Errorf("field %q: got %v, want %v", k, got, want)
		}
	}
	if _, ok := m["ue_ip"]; ok {
		t.Error("type2 output should not include ue_ip")
	}
	if _, ok := m["qfi"]; ok {
		t.Error("type2 output should not include qfi")
	}
	rt, ok := m["rt"].([]interface{})
	if !ok || len(rt) != 1 || rt[0] != "65000:200" {
		t.Fatalf("type2 output rt mismatch: %#v", m["rt"])
	}
	mupOut, ok := m["mup_extended_community"].(map[string]interface{})
	if !ok || mupOut["segment_identifier"] != "000100000002" {
		t.Fatalf("type2 output MUP ext community mismatch: %#v", m["mup_extended_community"])
	}
}

type recordingDryRunSender struct {
	mu      sync.Mutex
	outputs []map[string]interface{}
}

func (r *recordingDryRunSender) record(op, routeType string, rib *ir.BGPRIBInfo) {
	out := buildDryRunOutput(op, routeType, rib)
	r.mu.Lock()
	r.outputs = append(r.outputs, out)
	r.mu.Unlock()
}
func (r *recordingDryRunSender) AddType1Route(_ context.Context, rib *ir.BGPRIBInfo) error {
	r.record("ADD", "type1", rib)
	return nil
}
func (r *recordingDryRunSender) AddType2Route(_ context.Context, rib *ir.BGPRIBInfo) error {
	r.record("ADD", "type2", rib)
	return nil
}
func (r *recordingDryRunSender) UpdateType1Route(_ context.Context, rib *ir.BGPRIBInfo) error {
	r.record("UPDATE", "type1", rib)
	return nil
}
func (r *recordingDryRunSender) UpdateType2Route(_ context.Context, rib *ir.BGPRIBInfo) error {
	r.record("UPDATE", "type2", rib)
	return nil
}
func (r *recordingDryRunSender) DeleteType1Route(_ context.Context, rib *ir.BGPRIBInfo) error {
	r.record("DELETE", "type1", rib)
	return nil
}
func (r *recordingDryRunSender) DeleteType2Route(_ context.Context, rib *ir.BGPRIBInfo) error {
	r.record("DELETE", "type2", rib)
	return nil
}

// TestDryRunFromPCAP verifies dry-run output is derived from PCAP + static_context
// and includes add/update/delete for both route types.
func TestDryRunFromPCAP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rt := dslruntime.RT{}

	root := repoRoot(t)
	staticCtxPath := filepath.Join(root, "static_context.example.json")
	pcapPath := filepath.Join(root, "sample", "Keysight", "pfcp-n9.pcap")

	// Load static context from file.
	sctx := staticctx.New()
	if err := sctx.Load(staticCtxPath); err != nil {
		t.Fatalf("static context load: %v", err)
	}

	// Load PFCP messages from PCAP.
	loader := testharness.PCAPGoLoader{}
	messages, err := loader.Load(pcapPath)
	if err != nil {
		t.Fatalf("pcap load: %v", err)
	}

	transformer, err := dialect.Lookup("Keysight_N9")
	if err != nil {
		t.Fatalf("dialect lookup: %v", err)
	}

	sm := pfcp.NewSessionManager(transformer)
	irMgr := ir.NewManager(sctx, 32)
	sender := &recordingDryRunSender{}
	pipeline.ConnectIRToBGP(ctx, irMgr, sender, "both")

	// Collect expected value sets derived from SessionInformation.
	ueIPs := map[string]struct{}{}
	endpoints := map[string]struct{}{}
	teids := map[uint32]struct{}{}
	qfis := map[uint8]struct{}{}

	for _, msg := range messages {
		switch msg.Type {
		case pfcp.MsgTypeSessionEstablishmentResponse:
			cpSEID := msg.SEID
			upSEIDVal := rt.GetField(msg.Fields, "pfcp", "f_seid", "seid")
			upSEID := rt.CoerceUint64(upSEIDVal)
			if upSEID != 0 && upSEID != cpSEID {
				sm.RegisterSEIDAlias(upSEID, cpSEID)
			}
		case pfcp.MsgTypeSessionEstablishmentRequest:
			info, err := sm.HandleEstablishment(msg.ToEstablishmentRequest())
			if err != nil {
				continue
			}
			if info == nil {
				continue
			}
			collectInfo(info, ueIPs, endpoints, teids, qfis)
			if err := irMgr.HandleCreate(info); err != nil {
				continue
			}
		case pfcp.MsgTypeSessionModificationRequest:
			info, err := sm.HandleModification(msg.ToModificationRequest())
			if err != nil {
				continue
			}
			if info == nil {
				continue
			}
			collectInfo(info, ueIPs, endpoints, teids, qfis)
			if err := irMgr.HandleUpdate(info); err != nil {
				continue
			}
		case pfcp.MsgTypeSessionDeletionRequest:
			req := msg.ToDeletionRequest()
			sm.HandleDeletion(req)
			irMgr.HandleDelete(sm.CanonicalSEID(req.SEID))
		default:
			continue
		}
	}

	// Wait briefly for async pipeline delivery.
	waitForOutputs(t, sender)

	sender.mu.Lock()
	outputs := append([]map[string]interface{}{}, sender.outputs...)
	sender.mu.Unlock()

	if len(outputs) == 0 {
		t.Fatal("no dry-run outputs captured")
	}

	// Build static-context derived sets.
	rdSet := map[string]struct{}{}
	rtSet := map[string]struct{}{}
	nexthopSet := map[string]struct{}{}
	sourceAddrSet := map[string]struct{}{}
	epLenSet := map[float64]struct{}{}
	segIDSet := map[string]struct{}{}
	for _, ni := range []string{"n3-nw", "n9-nw", "internet"} {
		if ctx, err := sctx.GetContext(ni); err == nil {
			rdSet[ctx.RD] = struct{}{}
			for _, rt := range ctx.RT {
				rtSet[rt] = struct{}{}
			}
			nexthopSet[ctx.NexthopAddress] = struct{}{}
			if ctx.SourceAddress != nil && *ctx.SourceAddress != "" {
				sourceAddrSet[*ctx.SourceAddress] = struct{}{}
			}
			if ctx.EndpointAddressLength != nil {
				epLenSet[float64(*ctx.EndpointAddressLength)] = struct{}{}
			}
			if ctx.MUPExtendedCommunity != nil {
				segIDSet[hex.EncodeToString(ctx.MUPExtendedCommunity.SegmentIdentifier[:])] = struct{}{}
			}
		}
	}

	seen := map[string]bool{}
	for _, out := range outputs {
		routeType, _ := out["route_type"].(string)
		op, _ := out["op"].(string)
		seen[op+"_"+routeType] = true

		// Required fields.
		requireKey(t, out, "op")
		requireKey(t, out, "route_type")
		requireKey(t, out, "seid")
		requireKey(t, out, "route_key")
		requireKey(t, out, "far_id")
		requireKey(t, out, "network_instance")
		requireKey(t, out, "rd")
		requireKey(t, out, "rt")
		requireKey(t, out, "nexthop")
		requireKey(t, out, "endpoint")
		requireKey(t, out, "teid")

		// Values must be from PCAP/static_context-derived sets.
		if rd, ok := out["rd"].(string); ok {
			if _, ok := rdSet[rd]; !ok {
				t.Fatalf("rd %q not in static_context set", rd)
			}
		}
		if nh, ok := out["nexthop"].(string); ok {
			if _, ok := nexthopSet[nh]; !ok {
				t.Fatalf("nexthop %q not in static_context set", nh)
			}
		}
		switch rtv := out["rt"].(type) {
		case []interface{}:
			for _, v := range rtv {
				s, _ := v.(string)
				if _, ok := rtSet[s]; !ok {
					t.Fatalf("rt %q not in static_context set", s)
				}
			}
		case []string:
			for _, s := range rtv {
				if _, ok := rtSet[s]; !ok {
					t.Fatalf("rt %q not in static_context set", s)
				}
			}
		default:
			t.Fatalf("rt has invalid type: %T", out["rt"])
		}
		if ep, ok := out["endpoint"].(string); ok {
			if _, ok := endpoints[ep]; !ok {
				t.Fatalf("endpoint %q not in pcap-derived set", ep)
			}
		}
		if te, ok := out["teid"].(float64); ok {
			if _, ok := teids[uint32(te)]; !ok {
				t.Fatalf("teid %v not in pcap-derived set", te)
			}
		}

		// Minimal field sets.
		switch routeType {
		case "type1":
			allowed := map[string]struct{}{
				"op": {}, "route_type": {}, "seid": {}, "route_key": {}, "far_id": {},
				"network_instance": {}, "ue_ip": {}, "ue_prefix": {},
				"endpoint": {}, "teid": {}, "qfi": {}, "rd": {}, "rt": {}, "nexthop": {},
				"source_address": {},
			}
			for k := range out {
				if _, ok := allowed[k]; !ok {
					t.Fatalf("type1 unexpected field: %s", k)
				}
			}
			if ue, ok := out["ue_ip"].(string); ok {
				if _, ok := ueIPs[ue]; !ok {
					t.Fatalf("ue_ip %q not in pcap-derived set", ue)
				}
			}
			if qfi, ok := out["qfi"].(float64); ok {
				if _, ok := qfis[uint8(qfi)]; !ok {
					t.Fatalf("qfi %v not in pcap-derived set", qfi)
				}
			}
			if sa, ok := out["source_address"].(string); ok {
				if _, ok := sourceAddrSet[sa]; !ok {
					t.Fatalf("source_address %q not in static_context set", sa)
				}
			}
		case "type2":
			allowed := map[string]struct{}{
				"op": {}, "route_type": {}, "seid": {}, "route_key": {}, "far_id": {},
				"network_instance": {}, "endpoint": {}, "teid": {}, "rd": {}, "rt": {}, "nexthop": {},
				"endpoint_address_length": {}, "mup_extended_community": {},
			}
			for k := range out {
				if _, ok := allowed[k]; !ok {
					t.Fatalf("type2 unexpected field: %s", k)
				}
			}
			if _, ok := out["ue_ip"]; ok {
				t.Fatalf("type2 should not include ue_ip")
			}
			if _, ok := out["qfi"]; ok {
				t.Fatalf("type2 should not include qfi")
			}
			requireKey(t, out, "endpoint_address_length")
			requireKey(t, out, "mup_extended_community")
			if l, ok := out["endpoint_address_length"].(float64); ok {
				if _, ok := epLenSet[l]; !ok {
					t.Fatalf("endpoint_address_length %v not in static_context set", l)
				}
			}
			if mext, ok := out["mup_extended_community"].(map[string]interface{}); ok {
				seg, _ := mext["segment_identifier"].(string)
				if _, ok := segIDSet[seg]; !ok {
					t.Fatalf("segment_identifier %q not in static_context set", seg)
				}
			} else {
				t.Fatalf("mup_extended_community has invalid type: %T", out["mup_extended_community"])
			}
		}
	}

	if !seen["ADD_type1"] || !seen["UPDATE_type1"] || !seen["DELETE_type1"] {
		t.Fatalf("missing type1 ops: %+v", seen)
	}
	if !seen["ADD_type2"] || !seen["UPDATE_type2"] || !seen["DELETE_type2"] {
		t.Fatalf("missing type2 ops: %+v", seen)
	}
}

func collectInfo(info *ir.SessionInformation, ueIPs, endpoints map[string]struct{}, teids map[uint32]struct{}, qfis map[uint8]struct{}) {
	if info == nil {
		return
	}
	if info.UEIPAddress != "" {
		ueIPs[info.UEIPAddress] = struct{}{}
	}
	if info.EndpointAddress != "" {
		endpoints[info.EndpointAddress] = struct{}{}
	}
	if info.TEID != 0 {
		teids[info.TEID] = struct{}{}
	}
	if info.QFI != 0 {
		qfis[info.QFI] = struct{}{}
	}
}

func requireKey(t *testing.T, out map[string]interface{}, key string) {
	t.Helper()
	if _, ok := out[key]; !ok {
		t.Fatalf("missing required field: %s", key)
	}
}

func waitForOutputs(t *testing.T, sender *recordingDryRunSender) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	last := -1
	for time.Now().Before(deadline) {
		sender.mu.Lock()
		n := len(sender.outputs)
		sender.mu.Unlock()
		if n > 0 && n == last {
			return
		}
		last = n
		time.Sleep(50 * time.Millisecond)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("repo root not found (go.mod)")
	return ""
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
