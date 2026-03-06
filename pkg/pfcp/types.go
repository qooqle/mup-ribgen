// Package pfcp defines PFCP (Packet Forwarding Control Protocol) data structures
// used in Mode-1 passive sniffer operation.
package pfcp

import "time"

// PFCPMessageType identifies the type of a PFCP session message.
type PFCPMessageType int

const (
	SessionEstablishmentRequest PFCPMessageType = iota
	SessionModificationRequest
	SessionDeletionRequest
)

// PDR represents a Packet Detection Rule.
type PDR struct {
	PDRID uint16
	// Additional IE fields populated by dialect-specific parsing
	Fields map[string]interface{}
}

// FAR represents a Forwarding Action Rule.
type FAR struct {
	FARID uint32
	// Additional IE fields populated by dialect-specific parsing
	Fields map[string]interface{}
}

// QER represents a QoS Enforcement Rule.
type QER struct {
	QERID uint32
	// Additional IE fields populated by dialect-specific parsing
	Fields map[string]interface{}
}

// PFCPSessionState holds the current accumulated state of a PFCP session,
// keyed by the session's SEID.
type PFCPSessionState struct {
	SEID         uint64
	PDRs         map[uint16]*PDR
	FARs         map[uint32]*FAR
	QERs         map[uint32]*QER
	LastModified time.Time
}

// PFCPSessionStateDelta represents the incremental changes from a
// Session Modification Request.
type PFCPSessionStateDelta struct {
	SEID       uint64
	UpdatePDRs map[uint16]*PDR
	RemovePDRs []uint16
	UpdateFARs map[uint32]*FAR
	RemoveFARs []uint32
	UpdateQERs map[uint32]*QER
	RemoveQERs []uint32
}

// PFCPEstablishmentRequest is the parsed representation of a PFCP
// Session Establishment Request message.
type PFCPEstablishmentRequest struct {
	SEID   uint64
	Fields map[string]interface{}
}

// PFCPModificationRequest is the parsed representation of a PFCP
// Session Modification Request message.
type PFCPModificationRequest struct {
	SEID   uint64
	Fields map[string]interface{}
}

// PFCPDeletionRequest is the parsed representation of a PFCP
// Session Deletion Request message.
type PFCPDeletionRequest struct {
	SEID uint64
}
