package ir_test

// Task 8.6: Performance benchmarks for IR Manager (req 10.3: <200ms).

import (
	"testing"

	"github.com/qooqle/mup-ribgen/pkg/ir"
)

// BenchmarkManagerHandleCreate measures the cost of creating a BGP RIB entry
// from a SessionInformation (req 10.3: IR → BGP RIB update <200ms).
func BenchmarkManagerHandleCreate(b *testing.B) {
	m := ir.NewManager(fixedSctx(), 256)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		info := &ir.SessionInformation{
			SEID:            uint64(i + 1),
			NetworkInstance: "test-nw",
		}
		_ = m.HandleCreate(info)
		// drain to prevent channel backpressure
		select {
		case <-m.Events():
		default:
		}
	}
}

// BenchmarkManagerHandleUpdate measures the cost of updating an existing
// BGP RIB entry (req 10.3).
func BenchmarkManagerHandleUpdate(b *testing.B) {
	m := ir.NewManager(fixedSctx(), 256)
	info := &ir.SessionInformation{SEID: 1, NetworkInstance: "test-nw"}
	_ = m.HandleCreate(info)
	<-m.Events() // drain create event

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.HandleUpdate(info)
		select {
		case <-m.Events():
		default:
		}
	}
}

// BenchmarkManagerConcurrent measures thread-safe concurrent access to the
// IR Manager (req 10.1: 10,000 sessions).
func BenchmarkManagerConcurrent(b *testing.B) {
	m := ir.NewManager(fixedSctx(), 1024)
	// Drain events in background to prevent channel blockage.
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-m.Events():
			case <-stop:
				return
			}
		}
	}()
	defer close(stop)

	b.RunParallel(func(pb *testing.PB) {
		seid := uint64(1)
		for pb.Next() {
			info := &ir.SessionInformation{
				SEID:            seid,
				NetworkInstance: "test-nw",
			}
			_ = m.HandleCreate(info)
			seid++
		}
	})
}
