// Package testharness provides the three-level DSL Test Harness infrastructure
// for validating compiled Dialect Transformers (req 12.1–12.16).
package testharness

import (
	"github.com/qooqle/mup-ribgen/pkg/ir"
	"github.com/qooqle/mup-ribgen/pkg/pfcp"
)

// --- Level 1: Unit test data types ------------------------------------------

// UnitTestCase is a single Level-1 test case loaded from JSON.
type UnitTestCase struct {
	Name              string                     `json:"name"`
	Type              string                     `json:"type"` // "establishment" | "modification" | "state_to_session_info"
	Establishment     *pfcp.PFCPEstablishmentRequest `json:"establishment,omitempty"`
	Modification      *pfcp.PFCPModificationRequest  `json:"modification,omitempty"`
	InputState        *pfcp.PFCPSessionState         `json:"input_state,omitempty"`
	ExpectedState     *pfcp.PFCPSessionState         `json:"expected_state,omitempty"`
	ExpectedDelta     *pfcp.PFCPSessionStateDelta    `json:"expected_delta,omitempty"`
	ExpectedSessionInfo *ir.SessionInformation       `json:"expected_session_info,omitempty"`
}

// UnitTestReport summarises the result of a Level-1 run.
type UnitTestReport struct {
	TotalTests  int
	PassedTests int
	FailedTests int
	Failures    []UnitTestFailure
}

// UnitTestFailure describes a single failing Level-1 test case.
type UnitTestFailure struct {
	TestName string
	Expected interface{}
	Actual   interface{}
	Diff     string
}

// --- Level 2: Lifecycle test data types -------------------------------------

// LifecycleStep is one step in a session lifecycle test.
type LifecycleStep struct {
	Type                string                         `json:"type"` // "establishment" | "modification" | "deletion"
	Establishment       *pfcp.PFCPEstablishmentRequest `json:"establishment,omitempty"`
	Modification        *pfcp.PFCPModificationRequest  `json:"modification,omitempty"`
	Deletion            *pfcp.PFCPDeletionRequest       `json:"deletion,omitempty"`
	ExpectedSessionInfo *ir.SessionInformation          `json:"expected_session_info,omitempty"`
}

// LifecycleTestCase is a Level-2 test loaded from JSON.
type LifecycleTestCase struct {
	Name  string          `json:"name"`
	Steps []LifecycleStep `json:"steps"`
}

// LifecycleTestReport summarises a Level-2 run.
type LifecycleTestReport struct {
	TotalSteps  int
	PassedSteps int
	FailedSteps int
	Failures    []LifecycleTestFailure
}

// LifecycleTestFailure describes a single failing lifecycle step.
type LifecycleTestFailure struct {
	StepIndex int
	StepType  string
	Expected  *ir.SessionInformation
	Actual    *ir.SessionInformation
	Diff      string
}

// --- Level 3: PCAP integration data types -----------------------------------

// PCAPTestReport summarises a Level-3 PCAP integration run.
type PCAPTestReport struct {
	TotalMessages  int
	PassedMessages int
	FailedMessages int
	Failures       []PCAPTestFailure
}

// PCAPTestFailure describes a single failing PCAP message comparison.
type PCAPTestFailure struct {
	MessageIndex int
	MessageType  string
	Expected     *ir.SessionInformation
	Actual       *ir.SessionInformation
	Diff         string
}

// --- Unified report ---------------------------------------------------------

// AllTestsReport aggregates results from all three levels.
type AllTestsReport struct {
	UnitReport      *UnitTestReport
	LifecycleReport *LifecycleTestReport
	PCAPReport      *PCAPTestReport
	OverallPassed   bool
}
