// Package testharness – Level 3: PCAP integration tests (req 12.11–12.16).
package testharness

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/qooqle/mup-ribgen/pkg/dialect"
	"github.com/qooqle/mup-ribgen/pkg/dslruntime"
	"github.com/qooqle/mup-ribgen/pkg/ir"
	"github.com/qooqle/mup-ribgen/pkg/pfcp"
)

var rtL3 = dslruntime.RT{}

// PCAPLoader extracts PFCP messages from a PCAP file.
// The default implementation is a stub; replace with a PCAPGoLoader
// for real PCAP file parsing.
type PCAPLoader interface {
	Load(pcapFile string) ([]*pfcp.ParsedMessage, error)
}

// StubPCAPLoader always returns an empty slice (used when no PCAP is available).
type StubPCAPLoader struct{}

func (StubPCAPLoader) Load(_ string) ([]*pfcp.ParsedMessage, error) {
	return nil, nil
}

// RunLevel3 runs a PCAP integration test (req 12.11–12.16).
// It reads all PFCP messages from pcapFile using loader, processes them through
// transformer in order (establishment → modification → deletion), and compares
// the resulting SessionInformation for each establishment/modification event
// against the JSON expectations in expectedFile.
func RunLevel3(pcapFile, expectedFile string, loader PCAPLoader, transformer dialect.DialectTransformer) (*PCAPTestReport, error) {
	messages, err := loader.Load(pcapFile)
	if err != nil {
		return nil, fmt.Errorf("level3: load pcap %q: %w", pcapFile, err)
	}
	expected, err := loadExpectedJSON(expectedFile)
	if err != nil {
		return nil, fmt.Errorf("level3: load expected %q: %w", expectedFile, err)
	}

	sm := newInMemoryStateManager(transformer)
	report := &PCAPTestReport{}
	expIdx := 0

	for i, msg := range messages {
		var got *ir.SessionInformation

		switch msg.Type {
		case pfcp.MsgTypeSessionEstablishmentRequest:
			got, err = sm.HandleEstablishment(msg.ToEstablishmentRequest())
			if err != nil {
				report.TotalMessages++
				report.FailedMessages++
				report.Failures = append(report.Failures, PCAPTestFailure{
					MessageIndex: i, MessageType: "establishment",
					Diff: "error: " + err.Error(),
				})
				continue
			}
		case pfcp.MsgTypeSessionModificationRequest:
			got, err = sm.HandleModification(msg.ToModificationRequest())
			if err != nil {
				report.TotalMessages++
				report.FailedMessages++
				report.Failures = append(report.Failures, PCAPTestFailure{
					MessageIndex: i, MessageType: "modification",
					Diff: "error: " + err.Error(),
				})
				continue
			}
		case pfcp.MsgTypeSessionEstablishmentResponse:
			// Extract UP's SEID from the F-SEID IE and register it as an alias
			// for the session stored under CP's SEID (in the header).
			cpSEID := msg.SEID
			upSEIDVal := rtL3.GetField(msg.Fields, "pfcp", "f_seid", "seid")
			upSEID := rtL3.CoerceUint64(upSEIDVal)
			if upSEID != 0 && upSEID != cpSEID {
				sm.addSEIDAlias(upSEID, cpSEID)
			}
			continue
		case pfcp.MsgTypeSessionDeletionRequest:
			sm.HandleDeletion(msg.ToDeletionRequest())
			continue
		default:
			continue // skip heartbeats, association messages, etc.
		}

		report.TotalMessages++
		if expIdx >= len(expected) {
			report.FailedMessages++
			report.Failures = append(report.Failures, PCAPTestFailure{
				MessageIndex: i,
				Diff:         fmt.Sprintf("no expected entry at index %d", expIdx),
			})
			expIdx++
			continue
		}
		exp := expected[expIdx]
		expIdx++
		if !reflect.DeepEqual(got, exp) {
			report.FailedMessages++
			report.Failures = append(report.Failures, PCAPTestFailure{
				MessageIndex: i, Expected: exp, Actual: got,
				Diff: diffJSON(exp, got),
			})
		} else {
			report.PassedMessages++
		}
	}
	return report, nil
}

func loadExpectedJSON(filename string) ([]*ir.SessionInformation, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var result []*ir.SessionInformation
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
