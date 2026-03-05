package testharness

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/qooqle/mup-ribgen/pkg/dialect"
	"github.com/qooqle/mup-ribgen/pkg/pfcp"
)

// RunLevel1File loads a JSON file of UnitTestCase entries and runs all of them
// against transformer (req 12.1–12.5).
func RunLevel1File(filename string, transformer dialect.DialectTransformer) (*UnitTestReport, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("level1: read %q: %w", filename, err)
	}
	var cases []UnitTestCase
	if err := json.Unmarshal(data, &cases); err != nil {
		return nil, fmt.Errorf("level1: parse %q: %w", filename, err)
	}
	return RunLevel1Cases(cases, transformer), nil
}

// RunLevel1Cases executes a slice of UnitTestCase against transformer.
func RunLevel1Cases(cases []UnitTestCase, transformer dialect.DialectTransformer) *UnitTestReport {
	report := &UnitTestReport{TotalTests: len(cases)}
	for _, tc := range cases {
		failure := runUnitCase(tc, transformer)
		if failure != nil {
			report.FailedTests++
			report.Failures = append(report.Failures, *failure)
		} else {
			report.PassedTests++
		}
	}
	return report
}

func runUnitCase(tc UnitTestCase, tr dialect.DialectTransformer) *UnitTestFailure {
	switch tc.Type {
	case "establishment":
		if tc.Establishment == nil || tc.ExpectedState == nil {
			return &UnitTestFailure{TestName: tc.Name, Diff: "missing establishment or expected_state"}
		}
		got, err := tr.EstablishmentToState(tc.Establishment)
		if err != nil {
			return &UnitTestFailure{TestName: tc.Name, Diff: "EstablishmentToState error: " + err.Error()}
		}
		if !pfcpStatesEqual(got, tc.ExpectedState) {
			return &UnitTestFailure{
				TestName: tc.Name,
				Expected: tc.ExpectedState,
				Actual:   got,
				Diff:     diffJSON(tc.ExpectedState, got),
			}
		}
	case "modification":
		if tc.Modification == nil || tc.ExpectedDelta == nil {
			return &UnitTestFailure{TestName: tc.Name, Diff: "missing modification or expected_delta"}
		}
		got, err := tr.ModificationToState(tc.Modification)
		if err != nil {
			return &UnitTestFailure{TestName: tc.Name, Diff: "ModificationToState error: " + err.Error()}
		}
		if !reflect.DeepEqual(got, tc.ExpectedDelta) {
			return &UnitTestFailure{
				TestName: tc.Name,
				Expected: tc.ExpectedDelta,
				Actual:   got,
				Diff:     diffJSON(tc.ExpectedDelta, got),
			}
		}
	case "state_to_session_info":
		if tc.InputState == nil || tc.ExpectedSessionInfo == nil {
			return &UnitTestFailure{TestName: tc.Name, Diff: "missing input_state or expected_session_info"}
		}
		got, err := tr.StateToSessionInfo(tc.InputState)
		if err != nil {
			return &UnitTestFailure{TestName: tc.Name, Diff: "StateToSessionInfo error: " + err.Error()}
		}
		if !reflect.DeepEqual(got, tc.ExpectedSessionInfo) {
			return &UnitTestFailure{
				TestName: tc.Name,
				Expected: tc.ExpectedSessionInfo,
				Actual:   got,
				Diff:     diffJSON(tc.ExpectedSessionInfo, got),
			}
		}
	default:
		return &UnitTestFailure{TestName: tc.Name, Diff: "unknown test type: " + tc.Type}
	}
	return nil
}

// pfcpStatesEqual compares two PFCPSessionState values, ignoring LastModified.
func pfcpStatesEqual(a, b *pfcp.PFCPSessionState) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	aCopy := *a
	bCopy := *b
	aCopy.LastModified = time.Time{}
	bCopy.LastModified = time.Time{}
	return reflect.DeepEqual(&aCopy, &bCopy)
}

// diffJSON returns a simple textual diff of two values serialised as JSON.
func diffJSON(expected, actual interface{}) string {
	expBytes, _ := json.MarshalIndent(expected, "", "  ")
	actBytes, _ := json.MarshalIndent(actual, "", "  ")
	return fmt.Sprintf("expected:\n%s\nactual:\n%s", expBytes, actBytes)
}
