package bgp_test

// Property 23: BGP route attribute completeness (req 8.5, 8.6, 8.7).

import (
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	gobgpapi "github.com/osrg/gobgp/v3/api"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/qooqle/mup-ribgen/pkg/bgp"
	"github.com/qooqle/mup-ribgen/pkg/ir"
)

// baseRIB returns a minimal valid BGPRIBInfo for IPv4 routes.
func baseRIB(seid uint64, teid uint32, qfi uint8) *ir.BGPRIBInfo {
	return &ir.BGPRIBInfo{
		SEID:            seid,
		UEIPAddress:     "10.0.0.1",
		TEID:            teid,
		QFI:             qfi,
		EndpointAddress: "20.0.0.1",
		RD:              "65000:100",
		RT:              []string{"65000:200"},
		NexthopAddress:  "192.168.1.1",
	}
}

// hasMsg returns true if attrs contains an anypb.Any whose type URL ends with msgName.
func hasMsg(attrs []*anypb.Any, msgName string) bool {
	for _, a := range attrs {
		if strings.HasSuffix(a.TypeUrl, msgName) {
			return true
		}
	}
	return false
}

// TestProperty23_BGPRouteAttributeCompleteness verifies req 8.5, 8.6, 8.7:
// the BGP RIB Info contains all attributes required for MUP SAFI routes.
func TestProperty23_BGPRouteAttributeCompleteness(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Property 23a: Type1 route always contains MpReachNLRI and RT ExtComm (req 8.6).
	properties.Property("Property23a: Type1 route has MpReachNLRI and ExtendedCommunities", prop.ForAll(
		func(seid uint64, teid uint32, qfi uint8) bool {
			rib := baseRIB(seid, teid, qfi)
			req, err := bgp.BuildType1Route(rib)
			if err != nil {
				return false
			}
			return hasMsg(req.Path.Pattrs, "MpReachNLRIAttribute") &&
				hasMsg(req.Path.Pattrs, "ExtendedCommunitiesAttribute")
		},
		gen.UInt64().SuchThat(func(v uint64) bool { return v != 0 }),
		gen.UInt32(),
		gen.UInt8(),
	))

	// Property 23b: Type1 route NLRI carries the UE prefix and TEID (req 8.6).
	properties.Property("Property23b: Type1 NLRI carries correct TEID and prefix", prop.ForAll(
		func(teid uint32) bool {
			rib := baseRIB(1, teid, 9)
			req, err := bgp.BuildType1Route(rib)
			if err != nil {
				return false
			}
			if req.Path.Family.Safi != gobgpapi.Family_SAFI_MUP {
				return false
			}
			var nlri gobgpapi.MUPType1SessionTransformedRoute
			if err := req.Path.Nlri.UnmarshalTo(&nlri); err != nil {
				return false
			}
			return nlri.Teid == teid && nlri.Prefix == "10.0.0.1/32"
		},
		gen.UInt32(),
	))

	// Property 23c: Type2 route always contains MpReachNLRI and ExtComm (req 8.7).
	properties.Property("Property23c: Type2 route has MpReachNLRI and ExtendedCommunities", prop.ForAll(
		func(seid uint64, teid uint32) bool {
			rib := baseRIB(seid, teid, 0)
			req, err := bgp.BuildType2Route(rib)
			if err != nil {
				return false
			}
			return hasMsg(req.Path.Pattrs, "MpReachNLRIAttribute") &&
				hasMsg(req.Path.Pattrs, "ExtendedCommunitiesAttribute")
		},
		gen.UInt64().SuchThat(func(v uint64) bool { return v != 0 }),
		gen.UInt32(),
	))

	// Property 23d: Type2 route NLRI carries correct TEID and endpoint (req 8.7).
	properties.Property("Property23d: Type2 NLRI carries correct TEID and endpoint", prop.ForAll(
		func(teid uint32) bool {
			rib := baseRIB(1, teid, 0)
			req, err := bgp.BuildType2Route(rib)
			if err != nil {
				return false
			}
			if req.Path.Family.Safi != gobgpapi.Family_SAFI_MUP {
				return false
			}
			var nlri gobgpapi.MUPType2SessionTransformedRoute
			if err := req.Path.Nlri.UnmarshalTo(&nlri); err != nil {
				return false
			}
			return nlri.Teid == teid && nlri.EndpointAddress == "20.0.0.1"
		},
		gen.UInt32(),
	))

	// Property 23e: Type2 with MUPExtendedCommunity appends MUPExtended attr.
	properties.Property("Property23e: Type2 with MUP community includes MUPExtended", prop.ForAll(
		func(teid uint32) bool {
			rib := baseRIB(1, teid, 0)
			rib.MUPExtendedCommunity = &ir.MUPExtendedCommunity{
				SegmentIdentifier: [6]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x02},
			}
			req, err := bgp.BuildType2Route(rib)
			if err != nil {
				return false
			}
			return hasMsg(req.Path.Pattrs, "MUPExtended")
		},
		gen.UInt32(),
	))

	// Property 23f: missing RD causes an error for both route types.
	properties.Property("Property23f: missing RD returns error", prop.ForAll(
		func(teid uint32) bool {
			rib := baseRIB(1, teid, 0)
			rib.RD = ""
			_, err1 := bgp.BuildType1Route(rib)
			_, err2 := bgp.BuildType2Route(rib)
			return err1 != nil && err2 != nil
		},
		gen.UInt32(),
	))

	// Property 23g: missing NexthopAddress causes an error for both route types.
	properties.Property("Property23g: missing NexthopAddress returns error", prop.ForAll(
		func(teid uint32) bool {
			rib := baseRIB(1, teid, 0)
			rib.NexthopAddress = ""
			_, err1 := bgp.BuildType1Route(rib)
			_, err2 := bgp.BuildType2Route(rib)
			return err1 != nil && err2 != nil
		},
		gen.UInt32(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
