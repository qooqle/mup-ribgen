package pfcp_test

// Task 8.6: Performance benchmarks for PFCP processing (req 10.2: <100ms).

import (
	"testing"

	"github.com/qooqle/mup-ribgen/pkg/pfcp"
)

// BenchmarkSessionManagerEstablishment measures the cost of creating a new
// PFCP session state (req 10.2: PFCP → IR conversion <100ms).
func BenchmarkSessionManagerEstablishment(b *testing.B) {
	sm := pfcp.NewSessionManager(&echoTransformer{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := &pfcp.PFCPEstablishmentRequest{
			SEID:   uint64(i + 1),
			Fields: map[string]interface{}{},
		}
		_, _ = sm.HandleEstablishment(req)
	}
}

// BenchmarkSessionManagerModification measures the cost of updating an
// existing PFCP session state (req 10.2).
func BenchmarkSessionManagerModification(b *testing.B) {
	sm := pfcp.NewSessionManager(&echoTransformer{})
	// Pre-create one session to update repeatedly.
	_, _ = sm.HandleEstablishment(&pfcp.PFCPEstablishmentRequest{
		SEID:   1,
		Fields: map[string]interface{}{},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sm.HandleModification(&pfcp.PFCPModificationRequest{
			SEID:   1,
			Fields: map[string]interface{}{},
		})
	}
}

// BenchmarkParseMessage measures raw PFCP byte parsing (req 10.2).
// Uses a well-formed but minimal Establishment Request without IEs.
func BenchmarkParseMessage(b *testing.B) {
	// Minimal PFCP Session Establishment Request:
	// - 0x21: version=1, S=1 (SEID present in header)
	// - 0x32: msg type 50 (Session Establishment Request)
	// - 0x00, 0x0c: length = 12 (4-byte header-after-length already counted)
	// - 8 bytes SEID
	// - 3 bytes seq + 1 spare
	data := []byte{
		0x21, 0x32, 0x00, 0x0c,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // SEID=1
		0x00, 0x00, 0x01, 0x00, // seq=1, spare=0
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pfcp.ParseMessage(data)
	}
}
