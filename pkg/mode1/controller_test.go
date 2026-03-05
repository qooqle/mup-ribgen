package mode1_test

// Property tests for Mode-1 Controller (req 2.1, 2.2).

import (
	"context"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/qooqle/mup-ribgen/pkg/ir"
	"github.com/qooqle/mup-ribgen/pkg/mode1"
	"github.com/qooqle/mup-ribgen/pkg/pfcp"
)

// --- stub transformer -------------------------------------------------------

type stubTransformer struct{}

func (t *stubTransformer) EstablishmentToState(req *pfcp.PFCPEstablishmentRequest) (*pfcp.PFCPSessionState, error) {
	return &pfcp.PFCPSessionState{
		SEID:         req.SEID,
		PDRs:         make(map[uint16]*pfcp.PDR),
		FARs:         make(map[uint32]*pfcp.FAR),
		QERs:         make(map[uint32]*pfcp.QER),
		LastModified: time.Now(),
	}, nil
}

func (t *stubTransformer) ModificationToState(req *pfcp.PFCPModificationRequest) (*pfcp.PFCPSessionStateDelta, error) {
	return &pfcp.PFCPSessionStateDelta{
		SEID:       req.SEID,
		UpdatePDRs: make(map[uint16]*pfcp.PDR),
		UpdateFARs: make(map[uint32]*pfcp.FAR),
	}, nil
}

func (t *stubTransformer) StateToSessionInfo(state *pfcp.PFCPSessionState) (*ir.SessionInformation, error) {
	return &ir.SessionInformation{
		SEID:   state.SEID,
		Source: ir.Mode1PFCP,
	}, nil
}

// --- stub sniffer -----------------------------------------------------------

type stubSniffer struct {
	packets []*pfcp.RawPacket
	ch      chan *pfcp.RawPacket
}

func newStubSniffer(packets []*pfcp.RawPacket) *stubSniffer {
	return &stubSniffer{packets: packets}
}

func (s *stubSniffer) Start(ctx context.Context) (<-chan *pfcp.RawPacket, error) {
	s.ch = make(chan *pfcp.RawPacket, len(s.packets)+1)
	for _, p := range s.packets {
		s.ch <- p
	}
	close(s.ch)
	return s.ch, nil
}

func (s *stubSniffer) Stop() {}

// buildEstablishmentPacket creates a minimal valid PFCP Session Establishment
// Request binary (no F-SEID IE, S-flag=0) for testing the parser path.
func buildEstablishmentPacket(seqNo uint32) []byte {
	// PFCP header: version=1 S-flag=0, msg_type=50 (est. req), length=4, seqno, spare
	pkt := []byte{
		0x20,       // version=1, spare, MP=0, S=0
		50,         // msg_type = Session Establishment Request
		0x00, 0x04, // message length = 4 (header body after length field)
		byte(seqNo >> 16), byte(seqNo >> 8), byte(seqNo), // seq no
		0x00, // spare
	}
	return pkt
}

// --- Property tests ---------------------------------------------------------

// TestProperty_Mode1Pipeline verifies that the Mode-1 pipeline correctly
// emits establishment events for each captured packet (req 2.1, 2.2).
func TestProperty_Mode1Pipeline(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Property: one establishment packet → one establishment event emitted.
	properties.Property("one establishment packet emits one event", prop.ForAll(
		func(seqNo uint32) bool {
			pkt := &pfcp.RawPacket{
				Data:      buildEstablishmentPacket(seqNo),
				Timestamp: time.Now(),
			}
			sniffer := newStubSniffer([]*pfcp.RawPacket{pkt})
			ctrl := mode1.New(
				mode1.Config{Interface: "test0", ChannelBuffer: 16},
				sniffer,
				&stubTransformer{},
			)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			if err := ctrl.Start(ctx); err != nil {
				return false
			}
			defer ctrl.Stop()

			var events []*mode1.SessionEvent
			for ev := range ctrl.Events() {
				events = append(events, ev)
			}
			return len(events) == 1 && events[0].Type == "establishment"
		},
		gen.UInt32(),
	))

	// Property: N establishment packets → N establishment events, all unique (if SEIDs differ).
	properties.Property("N packets emit N events", prop.ForAll(
		func(n uint8) bool {
			if n == 0 {
				return true
			}
			count := int(n%5) + 1 // 1..5
			var packets []*pfcp.RawPacket
			for i := 0; i < count; i++ {
				packets = append(packets, &pfcp.RawPacket{
					Data:      buildEstablishmentPacket(uint32(i + 1)),
					Timestamp: time.Now(),
				})
			}
			sniffer := newStubSniffer(packets)
			ctrl := mode1.New(
				mode1.Config{Interface: "test0", ChannelBuffer: 32},
				sniffer,
				&stubTransformer{},
			)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := ctrl.Start(ctx); err != nil {
				return false
			}
			defer ctrl.Stop()
			var events []*mode1.SessionEvent
			for ev := range ctrl.Events() {
				events = append(events, ev)
			}
			return len(events) == count
		},
		gen.UInt8(),
	))

	// Property: malformed packet → no event emitted (error is swallowed, req 9.4).
	properties.Property("malformed packet emits no event", prop.ForAll(
		func(garbage []byte) bool {
			if len(garbage) == 0 {
				return true
			}
			pkt := &pfcp.RawPacket{Data: garbage, Timestamp: time.Now()}
			sniffer := newStubSniffer([]*pfcp.RawPacket{pkt})
			ctrl := mode1.New(
				mode1.Config{Interface: "test0", ChannelBuffer: 8},
				sniffer,
				&stubTransformer{},
			)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := ctrl.Start(ctx); err != nil {
				return false
			}
			defer ctrl.Stop()
			var events []*mode1.SessionEvent
			for ev := range ctrl.Events() {
				events = append(events, ev)
			}
			return len(events) == 0
		},
		gen.SliceOf(gen.UInt8()).SuchThat(func(b []byte) bool {
			// Ensure this is not accidentally a valid PFCP packet
			return len(b) < 4 || (b[0]>>5)&0x7 != 1
		}),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
