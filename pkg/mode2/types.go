// Package mode2 implements the Mode 2 Session Information receiver.
// It exposes a gRPC server that accepts session events from SMF plugins
// (e.g., the free5GC SMF integration plugin) and forwards them to the IR Manager.
//
// Wire protocol: gRPC with JSON encoding (content-type: application/grpc+json).
// See mode2.proto for the service definition.
package mode2

// EventType indicates whether a session was created, updated, or deleted.
type EventType int32

const (
	EventCreate EventType = 0
	EventUpdate EventType = 1
	EventDelete EventType = 2
)

// SessionInfo carries the session data extracted from a free5GC SMF context.
type SessionInfo struct {
	SessionID       string `json:"session_id"`
	SEID            uint64 `json:"seid"`
	UEIPAddress     string `json:"ue_ip_address"`
	UEPrefix        string `json:"ue_prefix"`     // CIDR notation
	TEID            uint32 `json:"teid"`
	QFI             uint8  `json:"qfi"`           // 0 for 4G sessions
	EndpointAddress string `json:"endpoint_address"`
	NetworkInstance string `json:"network_instance"` // DNN / APN
}

// SessionEvent is sent from the SMF plugin to mup-ribgen.
type SessionEvent struct {
	Type    EventType   `json:"type"`
	Session SessionInfo `json:"session"`
}

// EventResponse is the reply from mup-ribgen to the SMF plugin.
type EventResponse struct {
	Accepted     bool   `json:"accepted"`
	ErrorMessage string `json:"error_message,omitempty"`
}
