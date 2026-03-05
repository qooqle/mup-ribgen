package ir_test

// Properties 20, 21, 22: IR Manager BGP RIB lifecycle (req 8.2, 8.3, 8.4).

import (
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/qooqle/mup-ribgen/pkg/ir"
)

// --- stub StaticContextProvider ------------------------------------------------

type stubSctx struct {
	ctx *ir.StaticContext
}

func (s *stubSctx) GetContext(_ string) (*ir.StaticContext, error) {
	if s.ctx == nil {
		return nil, fmt.Errorf("not found")
	}
	return s.ctx, nil
}

func fixedSctx() *stubSctx {
	rd := "65000:100"
	nh := "10.0.0.1"
	return &stubSctx{ctx: &ir.StaticContext{
		NetworkInstance: "test-nw",
		RD:              rd,
		RT:              []string{"65000:200"},
		NexthopAddress:  nh,
	}}
}

// genNonEmptySEID generates a non-zero uint64 for use as SEID.
func genNonEmptySEID() gopter.Gen {
	return gen.UInt64().SuchThat(func(v uint64) bool { return v != 0 })
}

// genInfo generates a SessionInformation with a fixed non-empty NetworkInstance.
func genInfo(seid uint64) gopter.Gen {
	return gopter.CombineGens(
		gen.AlphaString(), // SessionID
		gen.AlphaString(), // UEIPAddress
		gen.UInt32(),      // TEID
		gen.UInt8(),       // QFI
		gen.AlphaString(), // EndpointAddress
	).Map(func(vals []interface{}) *ir.SessionInformation {
		return &ir.SessionInformation{
			SessionID:       vals[0].(string),
			SEID:            seid,
			UEIPAddress:     vals[1].(string),
			TEID:            vals[2].(uint32),
			QFI:             vals[3].(uint8),
			EndpointAddress: vals[4].(string),
			NetworkInstance: "test-nw",
			Source:          ir.Mode1PFCP,
		}
	})
}

// TestProperty20_IRManagerAddSessionInfo verifies req 8.2:
// adding a SessionInformation causes BGPRIBInfo to be created.
func TestProperty20_IRManagerAddSessionInfo(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("Property20: HandleCreate emits create event and stores RIB", prop.ForAll(
		func(seid uint64) bool {
			m := ir.NewManager(fixedSctx(), 16)
			info := &ir.SessionInformation{
				SEID:            seid,
				NetworkInstance: "test-nw",
			}
			if err := m.HandleCreate(info); err != nil {
				return false
			}
			// RIB entry must exist
			rib, ok := m.Get(seid)
			if !ok || rib == nil {
				return false
			}
			// SEID must be preserved
			if rib.SEID != seid {
				return false
			}
			// Static context must be merged
			if rib.RD != "65000:100" {
				return false
			}
			// Event must be emitted with type "create"
			select {
			case ev := <-m.Events():
				return ev.Type == ir.BGPEventCreate && ev.SEID == seid
			default:
				return false
			}
		},
		genNonEmptySEID(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty21_IRManagerUpdateSessionInfo verifies req 8.3:
// updating a SessionInformation causes the BGPRIBInfo to be updated.
func TestProperty21_IRManagerUpdateSessionInfo(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("Property21: HandleUpdate emits update event and updates RIB", prop.ForAll(
		func(seid uint64, teid1, teid2 uint32) bool {
			m := ir.NewManager(fixedSctx(), 32)

			// Create initial entry
			info1 := &ir.SessionInformation{SEID: seid, TEID: teid1, NetworkInstance: "test-nw"}
			if err := m.HandleCreate(info1); err != nil {
				return false
			}
			// Drain create event
			<-m.Events()

			// Update
			info2 := &ir.SessionInformation{SEID: seid, TEID: teid2, NetworkInstance: "test-nw"}
			if err := m.HandleUpdate(info2); err != nil {
				return false
			}

			// RIB must reflect updated TEID
			rib, ok := m.Get(seid)
			if !ok || rib.TEID != teid2 {
				return false
			}

			// Event must be emitted with type "update"
			select {
			case ev := <-m.Events():
				return ev.Type == ir.BGPEventUpdate && ev.SEID == seid
			default:
				return false
			}
		},
		genNonEmptySEID(),
		gen.UInt32(),
		gen.UInt32(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty22_IRManagerDeleteSessionInfo verifies req 8.4:
// deleting a SessionInformation removes the BGPRIBInfo and emits a delete event.
func TestProperty22_IRManagerDeleteSessionInfo(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("Property22: HandleDelete emits delete event and removes RIB", prop.ForAll(
		func(seid uint64) bool {
			m := ir.NewManager(fixedSctx(), 16)

			// Create first
			info := &ir.SessionInformation{SEID: seid, NetworkInstance: "test-nw"}
			if err := m.HandleCreate(info); err != nil {
				return false
			}
			// Drain create event
			<-m.Events()

			// Delete
			m.HandleDelete(seid)

			// RIB must no longer exist
			_, ok := m.Get(seid)
			if ok {
				return false
			}

			// Event must be emitted with type "delete"
			select {
			case ev := <-m.Events():
				return ev.Type == ir.BGPEventDelete && ev.SEID == seid
			default:
				return false
			}
		},
		genNonEmptySEID(),
	))

	// Deleting a non-existent SEID must not emit any event.
	properties.Property("Property22b: HandleDelete of unknown SEID emits no event", prop.ForAll(
		func(seid uint64) bool {
			m := ir.NewManager(fixedSctx(), 16)
			m.HandleDelete(seid)
			select {
			case <-m.Events():
				return false // unexpected event
			default:
				return true
			}
		},
		genNonEmptySEID(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
