package testharness

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/qooqle/mup-ribgen/pkg/pfcp"
)

const pfcpPort = 8805

// PCAPGoLoader reads a PCAP file (pure Go, no libpcap required) and returns
// all PFCP messages found in UDP packets on port 8805.
type PCAPGoLoader struct{}

// Load opens pcapFile and extracts PFCP ParsedMessages from UDP/8805 traffic.
func (PCAPGoLoader) Load(pcapFile string) ([]*pfcp.ParsedMessage, error) {
	f, err := os.Open(pcapFile)
	if err != nil {
		return nil, fmt.Errorf("pcap_loader: open %q: %w", pcapFile, err)
	}
	defer f.Close()

	r, err := pcapgo.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("pcap_loader: new reader: %w", err)
	}

	var msgs []*pfcp.ParsedMessage
	for {
		data, _, err := r.ReadPacketData()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip unreadable packets
		}
		payload := extractUDPPayload(data, r.LinkType())
		if len(payload) == 0 {
			continue
		}
		msg, err := pfcp.ParseMessage(payload)
		if err != nil {
			continue // skip malformed PFCP messages
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

// extractUDPPayload extracts the UDP payload from a raw packet if the destination
// or source port is pfcpPort (8805). Returns nil if the packet is not UDP/8805.
func extractUDPPayload(data []byte, linkType layers.LinkType) []byte {
	if linkType != layers.LinkTypeEthernet {
		return nil
	}
	if len(data) < 14 {
		return nil
	}
	// Ethernet header: 14 bytes
	etherType := binary.BigEndian.Uint16(data[12:14])

	var ipStart int
	switch etherType {
	case 0x0800: // IPv4
		ipStart = 14
	case 0x8100: // VLAN-tagged
		if len(data) < 18 {
			return nil
		}
		inner := binary.BigEndian.Uint16(data[16:18])
		if inner != 0x0800 {
			return nil
		}
		ipStart = 18
	default:
		return nil
	}

	if len(data) < ipStart+20 {
		return nil
	}
	ipVersion := data[ipStart] >> 4
	if ipVersion != 4 {
		return nil // only IPv4 supported
	}
	ihl := int(data[ipStart]&0x0F) * 4
	protocol := data[ipStart+9]
	if protocol != 17 { // UDP
		return nil
	}
	udpStart := ipStart + ihl
	if len(data) < udpStart+8 {
		return nil
	}
	srcPort := binary.BigEndian.Uint16(data[udpStart : udpStart+2])
	dstPort := binary.BigEndian.Uint16(data[udpStart+2 : udpStart+4])
	if srcPort != pfcpPort && dstPort != pfcpPort {
		return nil
	}
	payload := data[udpStart+8:]
	if len(payload) == 0 {
		return nil
	}
	return payload
}
