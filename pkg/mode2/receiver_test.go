package mode2_test

// Property 6: Mode2 Session Information received → IR create dispatched (req 3.1, 3.3)
// Property 7: Mode2 Session Information updated → IR update dispatched (req 3.4)
// Property 8: Mode2 Session Information deleted → IR delete dispatched (req 3.5)

import (
	"context"
	"log/slog"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"github.com/qooqle/mup-ribgen/pkg/ir"
	"github.com/qooqle/mup-ribgen/pkg/mode2"
)

// mockIRHandler captures calls to HandleCreate/HandleUpdate/HandleDelete.
type mockIRHandler struct {
	created []*ir.SessionInformation
	updated []*ir.SessionInformation
	deleted []uint64
}

func (m *mockIRHandler) HandleCreate(info *ir.SessionInformation) error {
	m.created = append(m.created, info)
	return nil
}

func (m *mockIRHandler) HandleUpdate(info *ir.SessionInformation) error {
	m.updated = append(m.updated, info)
	return nil
}

func (m *mockIRHandler) HandleDelete(seid uint64) {
	m.deleted = append(m.deleted, seid)
}

// genSessionInfo generates arbitrary mode2.SessionInfo values.
func genSessionInfo() gopter.Gen {
	return gopter.CombineGens(
		gen.AlphaString(), // SessionID
		gen.UInt64(),      // SEID
		gen.AlphaString(), // UEIPAddress
		gen.AlphaString(), // UEPrefix
		gen.UInt32(),      // TEID
		gen.UInt8(),       // QFI
		gen.AlphaString(), // EndpointAddress
		gen.AlphaString(), // NetworkInstance
	).Map(func(vals []interface{}) mode2.SessionInfo {
		return mode2.SessionInfo{
			SessionID:       vals[0].(string),
			SEID:            vals[1].(uint64),
			UEIPAddress:     vals[2].(string),
			UEPrefix:        vals[3].(string),
			TEID:            vals[4].(uint32),
			QFI:             vals[5].(uint8),
			EndpointAddress: vals[6].(string),
			NetworkInstance: vals[7].(string),
		}
	})
}

// TestProperty6_Mode2SessionCreate verifies that any session create event
// results in exactly one HandleCreate call with matching fields (req 3.1, 3.3).
func TestProperty6_Mode2SessionCreate(t *testing.T) {
	properties := gopter.NewProperties(nil)
	properties.Property(
		"Property6: Mode2 EventCreate dispatches HandleCreate with correct SessionInformation",
		prop.ForAll(
			func(s mode2.SessionInfo) bool {
				handler := &mockIRHandler{}
				recv := mode2.NewReceiver(handler, slog.Default())
				ev := &mode2.SessionEvent{Type: mode2.EventCreate, Session: s}

				resp, err := recv.ReportSession(context.Background(), ev)
				if err != nil || !resp.Accepted {
					return false
				}
				if len(handler.created) != 1 {
					return false
				}
				got := handler.created[0]
				return got.SessionID == s.SessionID &&
					got.SEID == s.SEID &&
					got.UEIPAddress == s.UEIPAddress &&
					got.TEID == s.TEID &&
					got.QFI == s.QFI &&
					got.Source == ir.Mode2Plugin
			},
			genSessionInfo(),
		),
	)
	properties.TestingRun(t)
}

// TestProperty7_Mode2SessionUpdate verifies that any session update event
// results in exactly one HandleUpdate call with matching fields (req 3.4).
func TestProperty7_Mode2SessionUpdate(t *testing.T) {
	properties := gopter.NewProperties(nil)
	properties.Property(
		"Property7: Mode2 EventUpdate dispatches HandleUpdate with correct SessionInformation",
		prop.ForAll(
			func(s mode2.SessionInfo) bool {
				handler := &mockIRHandler{}
				recv := mode2.NewReceiver(handler, slog.Default())
				ev := &mode2.SessionEvent{Type: mode2.EventUpdate, Session: s}

				resp, err := recv.ReportSession(context.Background(), ev)
				if err != nil || !resp.Accepted {
					return false
				}
				if len(handler.updated) != 1 {
					return false
				}
				got := handler.updated[0]
				return got.SessionID == s.SessionID &&
					got.SEID == s.SEID &&
					got.NetworkInstance == s.NetworkInstance &&
					got.Source == ir.Mode2Plugin
			},
			genSessionInfo(),
		),
	)
	properties.TestingRun(t)
}

// TestProperty8_Mode2SessionDelete verifies that any session delete event
// results in exactly one HandleDelete call with the matching SEID (req 3.5).
func TestProperty8_Mode2SessionDelete(t *testing.T) {
	properties := gopter.NewProperties(nil)
	properties.Property(
		"Property8: Mode2 EventDelete dispatches HandleDelete with correct SEID",
		prop.ForAll(
			func(s mode2.SessionInfo) bool {
				handler := &mockIRHandler{}
				recv := mode2.NewReceiver(handler, slog.Default())
				ev := &mode2.SessionEvent{Type: mode2.EventDelete, Session: s}

				resp, err := recv.ReportSession(context.Background(), ev)
				if err != nil || !resp.Accepted {
					return false
				}
				return len(handler.deleted) == 1 && handler.deleted[0] == s.SEID
			},
			genSessionInfo(),
		),
	)
	properties.TestingRun(t)
}
