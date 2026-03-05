//go:build pcap

// Real PFCP sniffer using gopacket/libpcap.
// Requires: brew install libpcap && go build -tags pcap

package pfcp

import (
	"context"
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

func newSniffer(iface string) Sniffer {
	return &pcapSniffer{iface: iface, stop: make(chan struct{})}
}

type pcapSniffer struct {
	iface  string
	handle *pcap.Handle
	stop   chan struct{}
}

func (s *pcapSniffer) Start(ctx context.Context) (<-chan *RawPacket, error) {
	h, err := pcap.OpenLive(s.iface, 65536, true, pcap.BlockForever)
	if err != nil {
		return nil, fmt.Errorf("pfcp sniffer: open %q: %w", s.iface, err)
	}
	if err := h.SetBPFFilter("udp port 8805"); err != nil {
		h.Close()
		return nil, fmt.Errorf("pfcp sniffer: bpf filter: %w", err)
	}
	s.handle = h
	ch := make(chan *RawPacket, 256)
	src := gopacket.NewPacketSource(h, h.LinkType())
	src.DecodeOptions = gopacket.DecodeOptions{Lazy: true, NoCopy: true}
	go func() {
		defer close(ch)
		defer h.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stop:
				return
			case pkt, ok := <-src.Packets():
				if !ok {
					return
				}
				raw := extractUDP(pkt)
				if raw == nil {
					continue
				}
				select {
				case ch <- raw:
				case <-ctx.Done():
					return
				case <-s.stop:
					return
				}
			}
		}
	}()
	return ch, nil
}

func (s *pcapSniffer) Stop() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	if s.handle != nil {
		s.handle.Close()
	}
}

func extractUDP(pkt gopacket.Packet) *RawPacket {
	net := pkt.NetworkLayer()
	trans := pkt.TransportLayer()
	if net == nil || trans == nil {
		return nil
	}
	udp, ok := trans.(*layers.UDP)
	if !ok || len(udp.Payload) == 0 {
		return nil
	}
	data := make([]byte, len(udp.Payload))
	copy(data, udp.Payload)
	return &RawPacket{
		Data:      data,
		Timestamp: pkt.Metadata().Timestamp,
		SrcIP:     net.NetworkFlow().Src().String(),
		DstIP:     net.NetworkFlow().Dst().String(),
		SrcPort:   uint16(udp.SrcPort),
		DstPort:   uint16(udp.DstPort),
	}
}
