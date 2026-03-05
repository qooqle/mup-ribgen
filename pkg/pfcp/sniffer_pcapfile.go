package pfcp

// PCAPFileSniffer reads a PCAP file and emits PFCP RawPackets.
// It implements the Sniffer interface without requiring libpcap (uses pcapgo).

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

// NewPCAPFileSniffer creates a Sniffer that reads packets from a PCAP file.
// Useful for replaying captured PFCP traffic in Mode 1 without a live interface.
func NewPCAPFileSniffer(pcapFile string) Sniffer {
	return &pcapFileSniffer{path: pcapFile}
}

type pcapFileSniffer struct {
	path string
}

// Start opens the PCAP file and sends all PFCP RawPackets to the returned channel.
// The channel is closed when all packets have been sent or ctx is cancelled.
func (s *pcapFileSniffer) Start(ctx context.Context) (<-chan *RawPacket, error) {
	f, err := os.Open(s.path)
	if err != nil {
		return nil, fmt.Errorf("pcapfile: open %q: %w", s.path, err)
	}
	r, err := pcapgo.NewReader(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("pcapfile: read header: %w", err)
	}

	ch := make(chan *RawPacket, 256)
	go func() {
		defer close(ch)
		defer f.Close()
		count := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			data, ci, err := r.ReadPacketData()
			if err == io.EOF {
				break
			}
			if err != nil {
				slog.Debug("pcapfile: skip unreadable packet", "err", err)
				continue
			}
			payload := extractPFCPPayload(data, r.LinkType())
			if len(payload) == 0 {
				continue
			}
			ts := ci.Timestamp
			if ts.IsZero() {
				ts = time.Now()
			}
			select {
			case <-ctx.Done():
				return
			case ch <- &RawPacket{Data: payload, Timestamp: ts}:
				count++
			}
		}
		slog.Info("pcapfile: done", "path", s.path, "pfcp_packets", count)
	}()
	return ch, nil
}

func (s *pcapFileSniffer) Stop() {}

// extractPFCPPayload extracts the UDP payload from a raw Ethernet/IPv4/UDP
// packet if the source or destination port is 8805 (PFCP).
func extractPFCPPayload(data []byte, linkType layers.LinkType) []byte {
	if linkType != layers.LinkTypeEthernet {
		return nil
	}
	if len(data) < 14 {
		return nil
	}
	etherType := binary.BigEndian.Uint16(data[12:14])

	ipStart := 0
	switch etherType {
	case 0x0800:
		ipStart = 14
	case 0x8100: // VLAN
		if len(data) < 18 {
			return nil
		}
		if binary.BigEndian.Uint16(data[16:18]) != 0x0800 {
			return nil
		}
		ipStart = 18
	default:
		return nil
	}

	if len(data) < ipStart+20 || data[ipStart]>>4 != 4 {
		return nil // not IPv4
	}
	ihl := int(data[ipStart]&0x0F) * 4
	if data[ipStart+9] != 17 { // not UDP
		return nil
	}
	udpStart := ipStart + ihl
	if len(data) < udpStart+8 {
		return nil
	}
	src := binary.BigEndian.Uint16(data[udpStart : udpStart+2])
	dst := binary.BigEndian.Uint16(data[udpStart+2 : udpStart+4])
	if src != PFCPPort && dst != PFCPPort {
		return nil
	}
	payload := data[udpStart+8:]
	if len(payload) == 0 {
		return nil
	}
	return payload
}
