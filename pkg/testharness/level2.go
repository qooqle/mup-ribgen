package testharness

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/qooqle/mup-ribgen/pkg/dialect"
	"github.com/qooqle/mup-ribgen/pkg/ir"
	"github.com/qooqle/mup-ribgen/pkg/pfcp"
)

// RunLevel2File loads a LifecycleTestCase JSON file and runs it (req 12.6–12.10).
func RunLevel2File(filename string, transformer dialect.DialectTransformer) (*LifecycleTestReport, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("level2: read %q: %w", filename, err)
	}
	var tc LifecycleTestCase
	if err := json.Unmarshal(data, &tc); err != nil {
		return nil, fmt.Errorf("level2: parse %q: %w", filename, err)
	}
	return RunLevel2Case(tc, transformer), nil
}

// RunLevel2Case runs a single lifecycle test against transformer.
func RunLevel2Case(tc LifecycleTestCase, transformer dialect.DialectTransformer) *LifecycleTestReport {
	report := &LifecycleTestReport{TotalSteps: len(tc.Steps)}
	sm := newInMemoryStateManager(transformer)

	for i, step := range tc.Steps {
		var got *ir.SessionInformation
		var err error

		switch step.Type {
		case "establishment":
			if step.Establishment != nil {
				got, err = sm.HandleEstablishment(step.Establishment)
			}
		case "modification":
			if step.Modification != nil {
				got, err = sm.HandleModification(step.Modification)
			}
		case "deletion":
			if step.Deletion != nil {
				err = sm.HandleDeletion(step.Deletion)
			}
			report.PassedSteps++
			continue
		}

		if err != nil {
			report.FailedSteps++
			report.Failures = append(report.Failures, LifecycleTestFailure{
				StepIndex: i, StepType: step.Type,
				Diff: "error: " + err.Error(),
			})
			continue
		}
		if step.ExpectedSessionInfo != nil && !reflect.DeepEqual(got, step.ExpectedSessionInfo) {
			report.FailedSteps++
			report.Failures = append(report.Failures, LifecycleTestFailure{
				StepIndex: i, StepType: step.Type,
				Expected: step.ExpectedSessionInfo,
				Actual:   got,
				Diff:     diffJSON(step.ExpectedSessionInfo, got),
			})
		} else {
			report.PassedSteps++
		}
	}
	return report
}

// --- In-memory session state manager (used for Level 2 & 3 tests) -----------

type inMemoryStateManager struct {
	mu           sync.RWMutex
	sessions     map[uint64]*pfcp.PFCPSessionState
	seidAliases  map[uint64]uint64 // upSEID → cpSEID (for PFCP protocol SEID remapping)
	transformer  dialect.DialectTransformer
}

func newInMemoryStateManager(t dialect.DialectTransformer) *inMemoryStateManager {
	return &inMemoryStateManager{
		sessions:    make(map[uint64]*pfcp.PFCPSessionState),
		seidAliases: make(map[uint64]uint64),
		transformer: t,
	}
}

// addSEIDAlias records that upSEID is an alias for an existing session stored under cpSEID.
func (m *inMemoryStateManager) addSEIDAlias(upSEID, cpSEID uint64) {
	m.mu.Lock()
	m.seidAliases[upSEID] = cpSEID
	m.mu.Unlock()
}

// resolveSEID returns the canonical session SEID for a given SEID,
// following alias chains if necessary.
func (m *inMemoryStateManager) resolveSEID(seid uint64) uint64 {
	if canonical, ok := m.seidAliases[seid]; ok {
		return canonical
	}
	return seid
}

func (m *inMemoryStateManager) HandleEstablishment(req *pfcp.PFCPEstablishmentRequest) (*ir.SessionInformation, error) {
	state, err := m.transformer.EstablishmentToState(req)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.sessions[state.SEID] = state
	m.mu.Unlock()
	return m.transformer.StateToSessionInfo(state)
}

func (m *inMemoryStateManager) HandleModification(req *pfcp.PFCPModificationRequest) (*ir.SessionInformation, error) {
	canonical := m.resolveSEID(req.SEID)
	m.mu.Lock()
	state, ok := m.sessions[canonical]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("level2: modification without establishment for SEID %d", req.SEID)
	}
	delta, err := m.transformer.ModificationToState(req)
	if err != nil {
		return nil, err
	}
	// Merge delta into existing state (req 2.8)
	mergeStateDelta(state, delta)
	state.LastModified = time.Now()

	m.mu.Lock()
	m.sessions[req.SEID] = state
	m.mu.Unlock()
	return m.transformer.StateToSessionInfo(state)
}

func (m *inMemoryStateManager) HandleDeletion(req *pfcp.PFCPDeletionRequest) error {
	canonical := m.resolveSEID(req.SEID)
	m.mu.Lock()
	delete(m.sessions, canonical)
	delete(m.seidAliases, req.SEID)
	m.mu.Unlock()
	return nil
}

// mergeStateDelta applies a PFCPSessionStateDelta to an existing state in-place.
func mergeStateDelta(state *pfcp.PFCPSessionState, delta *pfcp.PFCPSessionStateDelta) {
	for id, pdr := range delta.UpdatePDRs {
		state.PDRs[id] = pdr
	}
	for _, id := range delta.RemovePDRs {
		delete(state.PDRs, id)
	}
	for id, far := range delta.UpdateFARs {
		if existing, ok := state.FARs[id]; ok {
			if existing.Fields == nil {
				existing.Fields = map[string]interface{}{}
			}
			for k, v := range far.Fields {
				existing.Fields[k] = v
			}
			if ufp, _ := far.Fields["update_forwarding_parameters"].(map[string]interface{}); ufp != nil {
				fwd, _ := existing.Fields["forwarding_parameters"].(map[string]interface{})
				if fwd == nil {
					fwd = map[string]interface{}{}
				}
				for k, v := range ufp {
					fwd[k] = v
				}
				existing.Fields["forwarding_parameters"] = fwd
			}
			state.FARs[id] = existing
			continue
		}
		state.FARs[id] = far
	}
	for _, id := range delta.RemoveFARs {
		delete(state.FARs, id)
	}
	for id, qer := range delta.UpdateQERs {
		state.QERs[id] = qer
	}
	for _, id := range delta.RemoveQERs {
		delete(state.QERs, id)
	}
}
