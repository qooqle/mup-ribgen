// Package pfcp – PFCP sniffer interface and types (req 2.1, 2.6).
package pfcp

import (
	"context"
	"time"
)

// PFCPPort is the IANA-registered UDP port for PFCP (3GPP TS 29.244).
const PFCPPort = 8805

// RawPacket is a raw UDP payload captured from the network.
// The controller DOES NOT modify this data; it only observes a copy
// (passthrough semantics, req 2.6).
type RawPacket struct {
	Data      []byte
	Timestamp time.Time
	SrcIP     string
	DstIP     string
	SrcPort   uint16
	DstPort   uint16
}

// Sniffer captures PFCP packets on a network interface.
// The controller observes a copy of each packet and never modifies it,
// ensuring passthrough of SMF-UPF traffic (req 2.6).
type Sniffer interface {
	// Start begins capture and sends packets to the returned channel.
	// The channel is closed when ctx is cancelled or Stop is called.
	Start(ctx context.Context) (<-chan *RawPacket, error)

	// Stop terminates capture gracefully.
	Stop()
}

// NewSniffer creates a Sniffer for the given network interface.
// Without the 'pcap' build tag, returns a StubSniffer (no libpcap required).
// Build with -tags pcap and install libpcap to enable live capture:
//
//	brew install libpcap
//	go build -tags pcap ./...
func NewSniffer(iface string) Sniffer {
	return newSniffer(iface)
}
