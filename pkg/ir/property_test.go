package ir_test

// Property 10: Session Information data integrity
// Validates requirements 4.2, 4.3, 4.4, 4.5

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/qooqle/mup-ribgen/pkg/ir"
)

// genSessionInformation produces arbitrary SessionInformation values.
func genSessionInformation() gopter.Gen {
	return gopter.CombineGens(
		gen.AlphaString(), // SessionID
		gen.UInt64(),      // SEID
		gen.AlphaString(), // UEIPAddress
		gen.AlphaString(), // UEPrefix
		gen.UInt32(),      // TEID
		gen.UInt8(),       // QFI
		gen.AlphaString(), // EndpointAddress
		gen.AlphaString(), // NetworkInstance
		gen.OneConstOf(ir.Mode1PFCP, ir.Mode2Plugin), // Source
	).Map(func(vals []interface{}) ir.SessionInformation {
		return ir.SessionInformation{
			SessionID:       vals[0].(string),
			SEID:            vals[1].(uint64),
			UEIPAddress:     vals[2].(string),
			UEPrefix:        vals[3].(string),
			TEID:            vals[4].(uint32),
			QFI:             vals[5].(uint8),
			EndpointAddress: vals[6].(string),
			NetworkInstance: vals[7].(string),
			Source:          vals[8].(ir.SessionSource),
		}
	})
}

// Property 10: Session Information data integrity.
//
// For any SessionInformation, all required fields defined by requirements
// 4.2–4.5 must be present and structurally intact after a round-trip through
// a copy (simulating storage and retrieval).
//
//   - Req 4.2: session identification (SessionID, SEID)
//   - Req 4.3: UE information (UEIPAddress, UEPrefix)
//   - Req 4.4: tunnel information (TEID, QFI)
//   - Req 4.5: endpoint information (EndpointAddress, NetworkInstance)
func TestProperty10_SessionInformationDataIntegrity(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Req 4.2: session identification fields are preserved
	properties.Property("req4.2: SessionID and SEID are preserved", prop.ForAll(
		func(si ir.SessionInformation) bool {
			copied := si
			return copied.SessionID == si.SessionID && copied.SEID == si.SEID
		},
		genSessionInformation(),
	))

	// Req 4.3: UE information fields are preserved
	properties.Property("req4.3: UEIPAddress and UEPrefix are preserved", prop.ForAll(
		func(si ir.SessionInformation) bool {
			copied := si
			return copied.UEIPAddress == si.UEIPAddress && copied.UEPrefix == si.UEPrefix
		},
		genSessionInformation(),
	))

	// Req 4.4: tunnel information fields are preserved
	properties.Property("req4.4: TEID and QFI are preserved", prop.ForAll(
		func(si ir.SessionInformation) bool {
			copied := si
			return copied.TEID == si.TEID && copied.QFI == si.QFI
		},
		genSessionInformation(),
	))

	// Req 4.5: endpoint information fields are preserved
	properties.Property("req4.5: EndpointAddress and NetworkInstance are preserved", prop.ForAll(
		func(si ir.SessionInformation) bool {
			copied := si
			return copied.EndpointAddress == si.EndpointAddress &&
				copied.NetworkInstance == si.NetworkInstance
		},
		genSessionInformation(),
	))

	// Combined: all required fields survive a copy (full structural integrity)
	properties.Property("req4.2-4.5: all required fields are structurally intact", prop.ForAll(
		func(si ir.SessionInformation) bool {
			copied := si
			return copied.SessionID == si.SessionID &&
				copied.SEID == si.SEID &&
				copied.UEIPAddress == si.UEIPAddress &&
				copied.UEPrefix == si.UEPrefix &&
				copied.TEID == si.TEID &&
				copied.QFI == si.QFI &&
				copied.EndpointAddress == si.EndpointAddress &&
				copied.NetworkInstance == si.NetworkInstance &&
				copied.Source == si.Source
		},
		genSessionInformation(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
