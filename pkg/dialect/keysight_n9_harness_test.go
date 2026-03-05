package dialect_test

// Task 4.10 – Keysight_N9 dialect: DSL Test Harness (Level 1, 2, 3).
// Req: 5.8, 12.17, 12.18

import (
	"testing"

	"github.com/qooqle/mup-ribgen/pkg/dialect"
	"github.com/qooqle/mup-ribgen/pkg/testharness"
)

func mustTransformer(t *testing.T) dialect.DialectTransformer {
	t.Helper()
	tr, err := dialect.Lookup("Keysight_N9")
	if err != nil {
		t.Fatalf("Keysight_N9 transformer not registered: %v", err)
	}
	return tr
}

// TestKeysightN9_Level1_Establishment verifies EstablishmentToState via the
// JSON unit test fixture (req 12.1–12.5).
func TestKeysightN9_Level1_Establishment(t *testing.T) {
	tr := mustTransformer(t)
	report, err := testharness.RunLevel1File(
		"../../test_data/keysight_n9/unit/establishment.json", tr)
	if err != nil {
		t.Fatalf("RunLevel1File: %v", err)
	}
	if report.FailedTests > 0 {
		for _, f := range report.Failures {
			t.Errorf("FAIL [%s]:\n%s", f.TestName, f.Diff)
		}
	}
	t.Logf("Level1/establishment: %d/%d passed", report.PassedTests, report.TotalTests)
}

// TestKeysightN9_Level1_Modification verifies ModificationToState via the
// JSON unit test fixture (req 12.1–12.5).
func TestKeysightN9_Level1_Modification(t *testing.T) {
	tr := mustTransformer(t)
	report, err := testharness.RunLevel1File(
		"../../test_data/keysight_n9/unit/modification.json", tr)
	if err != nil {
		t.Fatalf("RunLevel1File: %v", err)
	}
	if report.FailedTests > 0 {
		for _, f := range report.Failures {
			t.Errorf("FAIL [%s]:\n%s", f.TestName, f.Diff)
		}
	}
	t.Logf("Level1/modification: %d/%d passed", report.PassedTests, report.TotalTests)
}

// TestKeysightN9_Level2_Lifecycle verifies the full session lifecycle
// (Establishment → Modification → Deletion) via the JSON test fixture (req 12.6–12.10).
func TestKeysightN9_Level2_Lifecycle(t *testing.T) {
	tr := mustTransformer(t)
	report, err := testharness.RunLevel2File(
		"../../test_data/keysight_n9/lifecycle/n9_session.json", tr)
	if err != nil {
		t.Fatalf("RunLevel2File: %v", err)
	}
	if report.FailedSteps > 0 {
		for _, f := range report.Failures {
			t.Errorf("FAIL step[%d/%s]:\n%s", f.StepIndex, f.StepType, f.Diff)
		}
	}
	t.Logf("Level2/lifecycle: %d/%d steps passed", report.PassedSteps, report.TotalSteps)
}

// TestKeysightN9_Level3_PCAP verifies the full PCAP integration pipeline against
// the sample Keysight N9 PCAP capture (req 12.11–12.16).
func TestKeysightN9_Level3_PCAP(t *testing.T) {
	tr := mustTransformer(t)
	report, err := testharness.RunLevel3(
		"../../sample/Keysight/pfcp-n9.pcap",
		"../../sample/Keysight/pfcp-n9-expected.json",
		testharness.PCAPGoLoader{},
		tr,
	)
	if err != nil {
		t.Fatalf("RunLevel3: %v", err)
	}
	if report.FailedMessages > 0 {
		for _, f := range report.Failures {
			t.Errorf("FAIL msg[%d/%s]:\n%s", f.MessageIndex, f.MessageType, f.Diff)
		}
	}
	t.Logf("Level3/pcap: %d/%d messages passed", report.PassedMessages, report.TotalMessages)
}
