// Package bgp provides GoBGP gRPC client and MUP SAFI route construction
// for the MUP Controller (req 8.1, 8.6, 8.7).
package bgp

import (
	"fmt"
	"net"

	gobgpapi "github.com/osrg/gobgp/v3/api"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/qooqle/mup-ribgen/pkg/ir"
)

// BuildType1Route constructs a GoBGP AddPathRequest for a
// Type_1_Session_Transformed_Route from rib (req 8.6).
//
// Required fields in rib:
//   - UEIPAddress or UEPrefix (UE IP prefix)
//   - TEID, QFI, EndpointAddress
//   - RD, RT, NexthopAddress (from StaticContext)
//
// SourceAddress is optional.
func BuildType1Route(rib *ir.BGPRIBInfo) (*gobgpapi.AddPathRequest, error) {
	if rib == nil {
		return nil, fmt.Errorf("bgp: nil BGPRIBInfo")
	}

	// Determine UE prefix
	prefix := rib.UEPrefix
	if prefix == "" {
		if rib.UEIPAddress == "" {
			return nil, fmt.Errorf("bgp: Type1: UEIPAddress and UEPrefix both empty for SEID=%d", rib.SEID)
		}
		ip := net.ParseIP(rib.UEIPAddress)
		if ip == nil {
			return nil, fmt.Errorf("bgp: Type1: invalid UEIPAddress %q", rib.UEIPAddress)
		}
		if ip.To4() != nil {
			prefix = rib.UEIPAddress + "/32"
		} else {
			prefix = rib.UEIPAddress + "/128"
		}
	}
	if rib.RD == "" {
		return nil, fmt.Errorf("bgp: Type1: empty RD for SEID=%d", rib.SEID)
	}
	if rib.NexthopAddress == "" {
		return nil, fmt.Errorf("bgp: Type1: empty NexthopAddress for SEID=%d", rib.SEID)
	}

	rdAny, err := parseRD(rib.RD)
	if err != nil {
		return nil, fmt.Errorf("bgp: Type1: %w", err)
	}

	epLen, err := addressBitLen(rib.EndpointAddress)
	if err != nil {
		return nil, fmt.Errorf("bgp: Type1: endpoint: %w", err)
	}

	var sourceAddr string
	var sourceAddrLen uint32
	if rib.SourceAddress != nil && *rib.SourceAddress != "" {
		sourceAddr = *rib.SourceAddress
		sourceAddrLen, err = addressBitLen(sourceAddr)
		if err != nil {
			return nil, fmt.Errorf("bgp: Type1: source address: %w", err)
		}
	}

	nlri, err := anypb.New(&gobgpapi.MUPType1SessionTransformedRoute{
		Rd:                    rdAny,
		Prefix:                prefix,
		Teid:                  rib.TEID,
		Qfi:                   uint32(rib.QFI),
		EndpointAddressLength: epLen,
		EndpointAddress:       rib.EndpointAddress,
		SourceAddressLength:   sourceAddrLen,
		SourceAddress:         sourceAddr,
	})
	if err != nil {
		return nil, fmt.Errorf("bgp: Type1: marshal NLRI: %w", err)
	}

	family := mupFamily(rib.UEIPAddress, rib.UEPrefix)
	attrs, err := buildPathAttrs(rib, nlri, family)
	if err != nil {
		return nil, err
	}

	return &gobgpapi.AddPathRequest{
		TableType: gobgpapi.TableType_GLOBAL,
		Path: &gobgpapi.Path{
			Nlri:   nlri,
			Pattrs: attrs,
			Family: family,
		},
	}, nil
}

// BuildType2Route constructs a GoBGP AddPathRequest for a
// Type_2_Session_Transformed_Route from rib (req 8.7).
//
// Required fields in rib:
//   - EndpointAddress, TEID
//   - RD, RT, NexthopAddress, EndpointAddressLength (from StaticContext)
//
// MUPExtendedCommunity is optional.
func BuildType2Route(rib *ir.BGPRIBInfo) (*gobgpapi.AddPathRequest, error) {
	if rib == nil {
		return nil, fmt.Errorf("bgp: nil BGPRIBInfo")
	}
	if rib.RD == "" {
		return nil, fmt.Errorf("bgp: Type2: empty RD for SEID=%d", rib.SEID)
	}
	if rib.EndpointAddress == "" {
		return nil, fmt.Errorf("bgp: Type2: empty EndpointAddress for SEID=%d", rib.SEID)
	}
	if rib.NexthopAddress == "" {
		return nil, fmt.Errorf("bgp: Type2: empty NexthopAddress for SEID=%d", rib.SEID)
	}

	rdAny, err := parseRD(rib.RD)
	if err != nil {
		return nil, fmt.Errorf("bgp: Type2: %w", err)
	}

	epLen := uint32(0)
	if rib.EndpointAddressLength != nil {
		epLen = uint32(*rib.EndpointAddressLength)
	} else {
		epLen, err = addressBitLen(rib.EndpointAddress)
		if err != nil {
			return nil, fmt.Errorf("bgp: Type2: endpoint: %w", err)
		}
	}

	nlri, err := anypb.New(&gobgpapi.MUPType2SessionTransformedRoute{
		Rd:                    rdAny,
		EndpointAddressLength: epLen,
		EndpointAddress:       rib.EndpointAddress,
		Teid:                  rib.TEID,
	})
	if err != nil {
		return nil, fmt.Errorf("bgp: Type2: marshal NLRI: %w", err)
	}

	family := mupFamily(rib.EndpointAddress, "")
	attrs, err := buildPathAttrs(rib, nlri, family)
	if err != nil {
		return nil, err
	}

	if rib.MUPExtendedCommunity != nil {
		mupAttr, err := buildMUPExtComm(rib.MUPExtendedCommunity)
		if err != nil {
			return nil, fmt.Errorf("bgp: Type2: MUP extended community: %w", err)
		}
		attrs = append(attrs, mupAttr)
	}

	return &gobgpapi.AddPathRequest{
		TableType: gobgpapi.TableType_GLOBAL,
		Path: &gobgpapi.Path{
			Nlri:   nlri,
			Pattrs: attrs,
			Family: family,
		},
	}, nil
}

// BuildDeleteType1Route constructs a GoBGP DeletePathRequest for a Type1 route.
func BuildDeleteType1Route(rib *ir.BGPRIBInfo) (*gobgpapi.DeletePathRequest, error) {
	req, err := BuildType1Route(rib)
	if err != nil {
		return nil, err
	}
	return &gobgpapi.DeletePathRequest{
		TableType: gobgpapi.TableType_GLOBAL,
		Path:      req.Path,
	}, nil
}

// BuildDeleteType2Route constructs a GoBGP DeletePathRequest for a Type2 route.
func BuildDeleteType2Route(rib *ir.BGPRIBInfo) (*gobgpapi.DeletePathRequest, error) {
	req, err := BuildType2Route(rib)
	if err != nil {
		return nil, err
	}
	return &gobgpapi.DeletePathRequest{
		TableType: gobgpapi.TableType_GLOBAL,
		Path:      req.Path,
	}, nil
}

// --- internal helpers ---------------------------------------------------------

// parseRD parses "ASN:Admin" or "IP:Admin" into an anypb.Any RouteDistinguisher.
func parseRD(rd string) (*anypb.Any, error) {
	// Type 0: ASN2:Admin4
	var asn2, admin4 uint32
	if n, _ := fmt.Sscanf(rd, "%d:%d", &asn2, &admin4); n == 2 {
		return anypb.New(&gobgpapi.RouteDistinguisherTwoOctetASN{
			Admin:    asn2,
			Assigned: admin4,
		})
	}
	// Type 1: IP:Admin2
	var ipStr string
	var admin2 uint32
	if n, _ := fmt.Sscanf(rd, "%15s:%d", &ipStr, &admin2); n == 2 {
		ip := net.ParseIP(ipStr)
		if ip != nil && ip.To4() != nil {
			return anypb.New(&gobgpapi.RouteDistinguisherIPAddress{
				Admin:    ip.String(),
				Assigned: admin2,
			})
		}
	}
	return nil, fmt.Errorf("cannot parse RD %q", rd)
}

// addressBitLen returns 32 for IPv4, 128 for IPv6, 0 for empty addr.
func addressBitLen(addr string) (uint32, error) {
	if addr == "" {
		return 0, nil
	}
	ip := net.ParseIP(addr)
	if ip == nil {
		return 0, fmt.Errorf("invalid IP address %q", addr)
	}
	if ip.To4() != nil {
		return 32, nil
	}
	return 128, nil
}

// mupFamily returns the MUP SAFI address family for IPv4 or IPv6.
func mupFamily(addr, prefix string) *gobgpapi.Family {
	target := addr
	if target == "" {
		target = prefix
	}
	afi := gobgpapi.Family_AFI_IP
	ip := net.ParseIP(target)
	if ip != nil && ip.To4() == nil {
		afi = gobgpapi.Family_AFI_IP6
	}
	return &gobgpapi.Family{
		Afi:  afi,
		Safi: gobgpapi.Family_SAFI_MUP,
	}
}

// buildPathAttrs constructs [MpReachNLRI, ExtendedCommunities(RT)] attributes.
func buildPathAttrs(rib *ir.BGPRIBInfo, nlri *anypb.Any, family *gobgpapi.Family) ([]*anypb.Any, error) {
	mpReach, err := anypb.New(&gobgpapi.MpReachNLRIAttribute{
		Family:   family,
		NextHops: []string{rib.NexthopAddress},
		Nlris:    []*anypb.Any{nlri},
	})
	if err != nil {
		return nil, fmt.Errorf("bgp: marshal MpReachNLRI: %w", err)
	}

	attrs := []*anypb.Any{mpReach}

	if len(rib.RT) > 0 {
		var extComms []*anypb.Any
		for _, rt := range rib.RT {
			extComm, err := parseRT(rt)
			if err != nil {
				return nil, fmt.Errorf("bgp: parse RT %q: %w", rt, err)
			}
			extComms = append(extComms, extComm)
		}
		extCommAttr, err := anypb.New(&gobgpapi.ExtendedCommunitiesAttribute{
			Communities: extComms,
		})
		if err != nil {
			return nil, fmt.Errorf("bgp: marshal ExtendedCommunities: %w", err)
		}
		attrs = append(attrs, extCommAttr)
	}
	return attrs, nil
}

// parseRT parses a Route Target "ASN:LocalAdmin" into an anypb.Any.
func parseRT(rt string) (*anypb.Any, error) {
	var asn, admin uint32
	if n, _ := fmt.Sscanf(rt, "%d:%d", &asn, &admin); n == 2 {
		return anypb.New(&gobgpapi.TwoOctetAsSpecificExtended{
			IsTransitive: true,
			SubType:      0x02, // Route Target
			Asn:          asn,
			LocalAdmin:   admin,
		})
	}
	return nil, fmt.Errorf("cannot parse RT %q", rt)
}

// buildMUPExtComm wraps ir.MUPExtendedCommunity into a GoBGP MUPExtended attribute.
func buildMUPExtComm(mup *ir.MUPExtendedCommunity) (*anypb.Any, error) {
	sid2 := uint32(mup.SegmentIdentifier[0])<<8 | uint32(mup.SegmentIdentifier[1])
	sid4 := uint32(mup.SegmentIdentifier[2])<<24 |
		uint32(mup.SegmentIdentifier[3])<<16 |
		uint32(mup.SegmentIdentifier[4])<<8 |
		uint32(mup.SegmentIdentifier[5])
	return anypb.New(&gobgpapi.MUPExtended{
		SubType:    0,
		SegmentId2: sid2,
		SegmentId4: sid4,
	})
}
