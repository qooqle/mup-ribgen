// Package ir defines the Session Information (IR: Intermediate Representation)
// data models used across the MUP Controller.
package ir

import "time"

// SessionSource indicates where the session information originated.
type SessionSource int

const (
	Mode1PFCP   SessionSource = iota // Captured via PFCP sniffer (Mode 1)
	Mode2Plugin                      // Received from SMF plugin (Mode 2)
)

// SessionInformation is the unified session data model that abstracts over
// PFCP dialect differences and SMF implementation variations.
type SessionInformation struct {
	// Route instance identification (multi-leg support)
	RouteKey string
	FARID    uint32

	// Session identification
	SessionID string
	SEID      uint64

	// UE information
	UEIPAddress string
	UEPrefix    string // CIDR notation

	// Tunnel information
	TEID uint32
	QFI  uint8

	// Endpoint information
	EndpointAddress string

	// Network instance (key for Static Context lookup)
	NetworkInstance string

	// Metadata
	Source SessionSource
}

// MUPExtendedCommunity holds the BGP MUP-specific Extended Community
// containing a Direct-Type Segment Identifier.
type MUPExtendedCommunity struct {
	SegmentIdentifier [6]byte
}

// StaticContext holds the per-Network-Instance configuration loaded from
// the static context config file.
type StaticContext struct {
	NetworkInstance string

	// BGP route attributes
	RD string
	RT []string

	// Type 1 Session Transformed Route (optional)
	SourceAddress *string

	// Type 2 Session Transformed Route
	MUPExtendedCommunity  *MUPExtendedCommunity
	EndpointAddressLength *int

	// Common
	NexthopAddress string
}

// BGPRIBInfo is the fully synthesized BGP route information produced by the
// IR Manager from a SessionInformation and its corresponding StaticContext.
// It contains all fields required to generate MUP SAFI routes for GoBGP.
type BGPRIBInfo struct {
	// Route instance identification (multi-leg support)
	RouteKey string
	FARID    uint32

	// Session identification (from SessionInformation)
	SessionID string
	SEID      uint64

	// UE information (from SessionInformation)
	UEIPAddress string
	UEPrefix    string // CIDR notation

	// Tunnel information (from SessionInformation)
	TEID uint32
	QFI  uint8

	// Endpoint information (from SessionInformation)
	EndpointAddress string

	// Static context fields (from StaticContext)
	NetworkInstance       string
	RD                    string
	RT                    []string
	SourceAddress         *string               // Type 1 (optional)
	MUPExtendedCommunity  *MUPExtendedCommunity // Type 2
	EndpointAddressLength *int                  // Type 2
	NexthopAddress        string

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
	Source    SessionSource
}
