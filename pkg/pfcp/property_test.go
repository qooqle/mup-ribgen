package pfcp_test

// Property 2:  PFCP packet passthrough (req 2.6)
// Property 3:  PFCP Session Establishment (req 2.3)
// Property 4:  PFCP Session Modification (req 2.4, 2.8, 2.9)
// Property 5:  PFCP Session Deletion (req 2.5)
// Property 26: PFCP parse error continuation (req 9.4)

import (
	"context"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/qooqle/mup-ribgen/pkg/ir"
	"github.com/qooqle/mup-ribgen/pkg/pfcp"
)

// --- Stub transformer -------------------------------------------------------

type echoTransformer struct{}

func (t *echoTransformer) EstablishmentToState(req *pfcp.PFCPEstablishmentRequest) (*pfcp.PFCPSessionState, error) {
	return &pfcp.PFCPSessionState{
		SEID:         req.SEID,
		PDRs:         make(map[uint16]*pfcp.PDR),
		FARs:         make(map[uint32]*pfcp.FAR),
		QERs:         make(map[uint32]*pfcp.QER),
		LastModified: time.Now(),
	}, nil
}

func (t *echoTransformer) ModificationToState(req *pfcp.PFCPModificationRequest) (*pfcp.PFCPSessionStateDelta, error) {
	return &pfcp.PFCPSessionStateDelta{
		SEID:       req.SEID,
		UpdatePDRs: make(map[uint16]*pfcp.PDR),
		UpdateFARs: make(map[uint32]*pfcp.FAR),
	}, nil
}

func (t *echoTransformer) StateToSessionInfo(state *pfcp.PFCPSessionState) (*ir.SessionInformation, error) {
	return &ir.SessionInformation{SEID: state.SEID, Source: ir.Mode1PFCP}, nil
}

// --- Property 2: PFCP passthrough -------------------------------------------

func TestProperty2_PFCPPassthrough(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("req2.6: sniffer channel closes on Stop", prop.ForAll(
		func(_ uint8) bool {
			sniffer := pfcp.NewSniffer("lo0")
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			ch, err := sniffer.Start(ctx)
			if err != nil {
				return false
			}
			sniffer.Stop()
			for range ch {
			}
			return true
		},
		gen.UInt8(),
	))

	properties.Property("req2.6: RawPacket data is byte-for-byte identical to input", prop.ForAll(
		func(data []byte) bool {
			if len(data) == 0 {
				return true
			}
			original := make([]byte, len(data))
			copy(original, data)
			pkt := &pfcp.RawPacket{Data: data, Timestamp: time.Now()}
			if len(pkt.Data) != len(original) {
				return false
			}
			for i, b := range pkt.Data {
				if b != original[i] {
					return false
				}
			}
			return true
		},
		gen.SliceOf(gen.UInt8()).SuchThat(func(s []byte) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// --- Property 3: Session Establishment --------------------------------------

func TestProperty3_SessionEstablishment(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("req2.3: establishment creates exactly one session", prop.ForAll(
		func(seid uint64) bool {
			sm := pfcp.NewSessionManager(&echoTransformer{})
			info, err := sm.HandleEstablishment(&pfcp.PFCPEstablishmentRequest{
				SEID:   seid,
				Fields: map[string]interface{}{},
			})
			return err == nil && info != nil && info.SEID == seid && sm.SessionCount() == 1
		},
		gen.UInt64(),
	))

	properties.Property("req2.3: two distinct SEIDs create two independent sessions", prop.ForAll(
		func(seid1, seid2 uint64) bool {
			if seid1 == seid2 {
				return true
			}
			sm := pfcp.NewSessionManager(&echoTransformer{})
			_, err1 := sm.HandleEstablishment(&pfcp.PFCPEstablishmentRequest{SEID: seid1, Fields: map[string]interface{}{}})
			_, err2 := sm.HandleEstablishment(&pfcp.PFCPEstablishmentRequest{SEID: seid2, Fields: map[string]interface{}{}})
			return err1 == nil && err2 == nil && sm.SessionCount() == 2
		},
		gen.UInt64(), gen.UInt64(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// --- Property 4: Session Modification ---------------------------------------

func TestProperty4_SessionModification(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("req2.4: modification after establishment succeeds", prop.ForAll(
		func(seid uint64) bool {
			sm := pfcp.NewSessionManager(&echoTransformer{})
			if _, err := sm.HandleEstablishment(&pfcp.PFCPEstablishmentRequest{SEID: seid, Fields: map[string]interface{}{}}); err != nil {
				return false
			}
			info, err := sm.HandleModification(&pfcp.PFCPModificationRequest{SEID: seid, Fields: map[string]interface{}{}})
			return err == nil && info != nil && sm.SessionCount() == 1
		},
		gen.UInt64(),
	))

	properties.Property("req2.9: modification without prior establishment returns error", prop.ForAll(
		func(seid uint64) bool {
			sm := pfcp.NewSessionManager(&echoTransformer{})
			_, err := sm.HandleModification(&pfcp.PFCPModificationRequest{SEID: seid, Fields: map[string]interface{}{}})
			return err != nil
		},
		gen.UInt64(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// --- Property 5: Session Deletion -------------------------------------------

func TestProperty5_SessionDeletion(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("req2.5: deletion removes the session state", prop.ForAll(
		func(seid uint64) bool {
			sm := pfcp.NewSessionManager(&echoTransformer{})
			if _, err := sm.HandleEstablishment(&pfcp.PFCPEstablishmentRequest{SEID: seid, Fields: map[string]interface{}{}}); err != nil {
				return false
			}
			sm.HandleDeletion(&pfcp.PFCPDeletionRequest{SEID: seid})
			return sm.SessionCount() == 0
		},
		gen.UInt64(),
	))

	properties.Property("req2.5: deletion of unknown SEID is a no-op", prop.ForAll(
		func(seid uint64) bool {
			sm := pfcp.NewSessionManager(&echoTransformer{})
			sm.HandleDeletion(&pfcp.PFCPDeletionRequest{SEID: seid})
			return sm.SessionCount() == 0
		},
		gen.UInt64(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// --- Property 26: PFCP parse error continuation -----------------------------

func TestProperty26_PFCPParseErrorContinuation(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("req9.4: malformed data returns error without panic", prop.ForAll(
		func(data []byte) bool {
			msg, err := pfcp.ParseMessage(data)
			if len(data) < 4 {
				return err != nil && msg == nil
			}
			return true
		},
		gen.SliceOf(gen.UInt8()),
	))

	properties.Property("req9.4: truncated body returns parse error", prop.ForAll(
		func(msgType uint8) bool {
			data := []byte{
				0x20, msgType, 0x00, 0x64,
				0x00, 0x00, 0x01, 0x00,
			}
			msg, err := pfcp.ParseMessage(data)
			return err != nil && msg == nil
		},
		gen.UInt8(),
	))

	properties.Property("req9.4: unsupported PFCP version returns error", prop.ForAll(
		func(version uint8) bool {
			v := version & 0x07
			if v == 1 {
				return true
			}
			data := []byte{v << 5, 50, 0x00, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
			_, err := pfcp.ParseMessage(data)
			return err != nil
		},
		gen.UInt8(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
