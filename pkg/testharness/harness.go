package testharness

import (
	"fmt"

	"github.com/qooqle/mup-ribgen/pkg/dialect"
)

// Harness is the unified DSL Test Harness that orchestrates all three levels.
type Harness struct {
	transformer dialect.DialectTransformer
	loader      PCAPLoader

	unitFiles      []string
	lifecycleFiles []string
	pcapPairs      []pcapPair // pcapFile + expectedJSONFile
}

type pcapPair struct{ pcap, expected string }

// New creates a Harness for the given transformer.
func New(transformer dialect.DialectTransformer) *Harness {
	return &Harness{transformer: transformer, loader: StubPCAPLoader{}}
}

// WithPCAPLoader sets a custom PCAP loader (replace stub with real implementation).
func (h *Harness) WithPCAPLoader(l PCAPLoader) *Harness { h.loader = l; return h }

// AddUnitFile registers a Level-1 JSON test file.
func (h *Harness) AddUnitFile(f string) *Harness {
	h.unitFiles = append(h.unitFiles, f)
	return h
}

// AddLifecycleFile registers a Level-2 JSON test file.
func (h *Harness) AddLifecycleFile(f string) *Harness {
	h.lifecycleFiles = append(h.lifecycleFiles, f)
	return h
}

// AddPCAPTest registers a Level-3 PCAP+expected-JSON pair.
func (h *Harness) AddPCAPTest(pcapFile, expectedFile string) *Harness {
	h.pcapPairs = append(h.pcapPairs, pcapPair{pcapFile, expectedFile})
	return h
}

// RunAll executes all registered tests and returns a combined report.
func (h *Harness) RunAll() (*AllTestsReport, error) {
	report := &AllTestsReport{}

	// Level 1
	unitReport := &UnitTestReport{}
	for _, f := range h.unitFiles {
		r, err := RunLevel1File(f, h.transformer)
		if err != nil {
			return nil, fmt.Errorf("harness: level1 %q: %w", f, err)
		}
		unitReport.TotalTests += r.TotalTests
		unitReport.PassedTests += r.PassedTests
		unitReport.FailedTests += r.FailedTests
		unitReport.Failures = append(unitReport.Failures, r.Failures...)
	}
	report.UnitReport = unitReport

	// Level 2
	lifecycleReport := &LifecycleTestReport{}
	for _, f := range h.lifecycleFiles {
		r, err := RunLevel2File(f, h.transformer)
		if err != nil {
			return nil, fmt.Errorf("harness: level2 %q: %w", f, err)
		}
		lifecycleReport.TotalSteps += r.TotalSteps
		lifecycleReport.PassedSteps += r.PassedSteps
		lifecycleReport.FailedSteps += r.FailedSteps
		lifecycleReport.Failures = append(lifecycleReport.Failures, r.Failures...)
	}
	report.LifecycleReport = lifecycleReport

	// Level 3
	pcapReport := &PCAPTestReport{}
	for _, pair := range h.pcapPairs {
		r, err := RunLevel3(pair.pcap, pair.expected, h.loader, h.transformer)
		if err != nil {
			return nil, fmt.Errorf("harness: level3 %q: %w", pair.pcap, err)
		}
		pcapReport.TotalMessages += r.TotalMessages
		pcapReport.PassedMessages += r.PassedMessages
		pcapReport.FailedMessages += r.FailedMessages
		pcapReport.Failures = append(pcapReport.Failures, r.Failures...)
	}
	report.PCAPReport = pcapReport

	report.OverallPassed = unitReport.FailedTests == 0 &&
		lifecycleReport.FailedSteps == 0 &&
		pcapReport.FailedMessages == 0

	return report, nil
}
