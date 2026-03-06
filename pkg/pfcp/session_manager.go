// Package pfcp – PFCP Session State Manager (req 2.3–2.9, 5.9).
package pfcp

import (
	"fmt"
	"sync"
	"time"

	"github.com/qooqle/mup-ribgen/pkg/ir"
)

// Transformer is the dialect-specific conversion logic consumed by SessionManager.
// dialect.DialectTransformer satisfies this interface automatically (structural typing).
type Transformer interface {
	EstablishmentToState(req *PFCPEstablishmentRequest) (*PFCPSessionState, error)
	ModificationToState(req *PFCPModificationRequest) (*PFCPSessionStateDelta, error)
	StateToSessionInfo(state *PFCPSessionState) (*ir.SessionInformation, error)
}

type multiInfoTransformer interface {
	StateToSessionInfos(state *PFCPSessionState) ([]*ir.SessionInformation, error)
}

// SessionManager manages the PFCP session state lifecycle (req 2.7).
// It stores PFCPSessionState per SEID and applies a Transformer
// to convert PFCP messages into SessionInformation for the IR Manager.
//
// All public methods are safe for concurrent use.
type SessionManager struct {
	mu          sync.RWMutex
	sessions    map[uint64]*PFCPSessionState
	seidAliases map[uint64]uint64 // upSEID → cpSEID (from Establishment Response)
	transformer Transformer
}

// NewSessionManager creates a SessionManager backed by the given transformer.
func NewSessionManager(transformer Transformer) *SessionManager {
	return &SessionManager{
		sessions:    make(map[uint64]*PFCPSessionState),
		seidAliases: make(map[uint64]uint64),
		transformer: transformer,
	}
}

// RegisterSEIDAlias registers upSEID as an alias for cpSEID.
// Called when an Establishment Response is observed (passive sniffing).
// This enables subsequent Modification/Deletion requests (which carry upSEID)
// to be resolved to the session stored under cpSEID.
func (m *SessionManager) RegisterSEIDAlias(upSEID, cpSEID uint64) {
	if upSEID == 0 || upSEID == cpSEID {
		return
	}
	m.mu.Lock()
	m.seidAliases[upSEID] = cpSEID
	m.mu.Unlock()
}

// resolveSEID returns the canonical (CP) SEID for a given SEID,
// following the alias chain if necessary.
func (m *SessionManager) resolveSEID(seid uint64) uint64 {
	m.mu.RLock()
	if cpSEID, ok := m.seidAliases[seid]; ok {
		m.mu.RUnlock()
		return cpSEID
	}
	m.mu.RUnlock()
	return seid
}

// CanonicalSEID returns the canonical (CP) SEID for a given SEID,
// following the alias chain if necessary.
func (m *SessionManager) CanonicalSEID(seid uint64) uint64 {
	return m.resolveSEID(seid)
}

// HandleEstablishment processes a PFCP Session Establishment Request (req 2.3).
// Creates a new session state and returns the resulting SessionInformation.
func (m *SessionManager) HandleEstablishment(req *PFCPEstablishmentRequest) (*ir.SessionInformation, error) {
	infos, err := m.HandleEstablishmentInfos(req)
	if err != nil {
		return nil, err
	}
	if len(infos) == 0 {
		return nil, nil
	}
	return infos[0], nil
}

// HandleEstablishmentInfos processes an Establishment and returns all derived session infos.
func (m *SessionManager) HandleEstablishmentInfos(req *PFCPEstablishmentRequest) ([]*ir.SessionInformation, error) {
	state, err := m.transformer.EstablishmentToState(req)
	if err != nil {
		return nil, fmt.Errorf("session manager: establishment SEID=%d: %w", req.SEID, err)
	}
	m.mu.Lock()
	m.sessions[state.SEID] = state
	m.mu.Unlock()
	return m.stateToInfos(state)
}

// HandleModification processes a PFCP Session Modification Request (req 2.4, 2.8).
// Merges the delta into the existing session state and returns updated SessionInformation.
// Returns an error if no session state exists for the given SEID (req 2.9).
func (m *SessionManager) HandleModification(req *PFCPModificationRequest) (*ir.SessionInformation, error) {
	infos, err := m.HandleModificationInfos(req)
	if err != nil {
		return nil, err
	}
	if len(infos) == 0 {
		return nil, nil
	}
	return infos[0], nil
}

// HandleModificationInfos processes a Modification and returns all derived session infos.
func (m *SessionManager) HandleModificationInfos(req *PFCPModificationRequest) ([]*ir.SessionInformation, error) {
	canonical := m.resolveSEID(req.SEID)
	m.mu.RLock()
	state, ok := m.sessions[canonical]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("session manager: modification without establishment for SEID %d", req.SEID)
	}

	delta, err := m.transformer.ModificationToState(req)
	if err != nil {
		return nil, fmt.Errorf("session manager: modification SEID=%d: %w", req.SEID, err)
	}
	mergeStateDeltaInto(state, delta)
	state.LastModified = time.Now()

	m.mu.Lock()
	m.sessions[canonical] = state
	m.mu.Unlock()
	return m.stateToInfos(state)
}

// HandleDeletion processes a PFCP Session Deletion Request (req 2.5).
// Removes the session state for the given SEID; safe to call even if SEID is unknown.
func (m *SessionManager) HandleDeletion(req *PFCPDeletionRequest) {
	canonical := m.resolveSEID(req.SEID)
	m.mu.Lock()
	delete(m.sessions, canonical)
	m.mu.Unlock()
}

// SessionCount returns the number of currently active sessions.
func (m *SessionManager) SessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

func (m *SessionManager) stateToInfos(state *PFCPSessionState) ([]*ir.SessionInformation, error) {
	if mt, ok := m.transformer.(multiInfoTransformer); ok {
		infos, err := mt.StateToSessionInfos(state)
		if err != nil {
			return nil, err
		}
		return infos, nil
	}
	info, err := m.transformer.StateToSessionInfo(state)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}
	return []*ir.SessionInformation{info}, nil
}

// mergeStateDeltaInto applies a PFCPSessionStateDelta to an existing state in-place (req 2.8).
func mergeStateDeltaInto(state *PFCPSessionState, delta *PFCPSessionStateDelta) {
	for id, pdr := range delta.UpdatePDRs {
		state.PDRs[id] = pdr
	}
	for _, id := range delta.RemovePDRs {
		delete(state.PDRs, id)
	}
	for id, far := range delta.UpdateFARs {
		if existing, ok := state.FARs[id]; ok {
			state.FARs[id] = mergeFAR(existing, far)
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

// mergeFAR applies partial update fields onto an existing FAR.
// It specifically handles Update Forwarding Parameters IE by merging it into
// forwarding_parameters so downstream extraction can use a single path.
func mergeFAR(existing, update *FAR) *FAR {
	if existing == nil {
		return update
	}
	if update == nil {
		return existing
	}
	if existing.Fields == nil {
		existing.Fields = map[string]interface{}{}
	}
	for k, v := range update.Fields {
		existing.Fields[k] = v
	}
	ufp, _ := update.Fields["update_forwarding_parameters"].(map[string]interface{})
	if ufp == nil {
		return existing
	}
	fwd, _ := existing.Fields["forwarding_parameters"].(map[string]interface{})
	if fwd == nil {
		fwd = map[string]interface{}{}
	}
	for k, v := range ufp {
		fwd[k] = v
	}
	existing.Fields["forwarding_parameters"] = fwd
	return existing
}
