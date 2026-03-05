//go:build !pcap

package pfcp

import "context"

// newSniffer returns a StubSniffer when the 'pcap' build tag is absent.
func newSniffer(_ string) Sniffer {
	return &StubSniffer{stop: make(chan struct{})}
}

// StubSniffer is a no-op Sniffer for builds without libpcap.
// It closes its output channel immediately when cancelled or stopped.
type StubSniffer struct {
	stop chan struct{}
}

func (s *StubSniffer) Start(ctx context.Context) (<-chan *RawPacket, error) {
	ch := make(chan *RawPacket)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
		case <-s.stop:
		}
	}()
	return ch, nil
}

func (s *StubSniffer) Stop() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
}
