// Package ir defines the Session Information data models and the IR Manager.
// This file implements the IR Manager (req 4.6, 4.7).
package ir

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// BGPEventType describes the kind of BGP RIB change.
type BGPEventType string

const (
	BGPEventCreate BGPEventType = "create"
	BGPEventUpdate BGPEventType = "update"
	BGPEventDelete BGPEventType = "delete"
)

// BGPEvent is emitted by the Manager whenever BGP RIB Info changes.
type BGPEvent struct {
	// Type is "create", "update", or "delete".
	Type BGPEventType
	// Info is the new/updated BGPRIBInfo; nil for delete events.
	Info *BGPRIBInfo
	// SEID is always set (even for delete events where Info is nil).
	SEID uint64
}

// StaticContextProvider is implemented by staticctx.Manager.
type StaticContextProvider interface {
	GetContext(networkInstance string) (*StaticContext, error)
}

// Manager synthesizes SessionInformation + StaticContext → BGPRIBInfo and
// maintains an in-memory store keyed by SEID (req 4.6, 4.7).
// It emits BGPEvents on a buffered channel for downstream consumers.
type Manager struct {
	mu     sync.RWMutex
	ribs   map[uint64]*BGPRIBInfo
	sctx   StaticContextProvider
	events chan *BGPEvent
}

// NewManager creates an IR Manager with the given StaticContextProvider.
// bufSize controls the BGPEvent channel buffer; defaults to 256 if ≤ 0.
func NewManager(sctx StaticContextProvider, bufSize int) *Manager {
	if bufSize <= 0 {
		bufSize = 256
	}
	return &Manager{
		ribs:   make(map[uint64]*BGPRIBInfo),
		sctx:   sctx,
		events: make(chan *BGPEvent, bufSize),
	}
}

// Events returns the channel on which BGPEvent pointers are sent.
// The caller must drain this channel to prevent the Manager from blocking.
func (m *Manager) Events() <-chan *BGPEvent { return m.events }

// HandleCreate creates a new BGPRIBInfo from info and emits a "create" event.
// Returns an error if the StaticContext for info.NetworkInstance is not found.
func (m *Manager) HandleCreate(info *SessionInformation) error {
	rib, err := m.synthesize(info, time.Now())
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.ribs[info.SEID] = rib
	m.mu.Unlock()

	m.emit(&BGPEvent{Type: BGPEventCreate, Info: rib, SEID: info.SEID})
	return nil
}

// HandleUpdate updates the BGPRIBInfo for info.SEID and emits an "update" event.
// Preserves the original CreatedAt timestamp if an existing entry is found.
// Returns an error if the StaticContext for info.NetworkInstance is not found.
func (m *Manager) HandleUpdate(info *SessionInformation) error {
	m.mu.RLock()
	existing, hasExisting := m.ribs[info.SEID]
	m.mu.RUnlock()

	rib, err := m.synthesize(info, time.Now())
	if err != nil {
		return err
	}
	if hasExisting {
		rib.CreatedAt = existing.CreatedAt
	}

	m.mu.Lock()
	m.ribs[info.SEID] = rib
	m.mu.Unlock()

	m.emit(&BGPEvent{Type: BGPEventUpdate, Info: rib, SEID: info.SEID})
	return nil
}

// HandleDelete removes the BGPRIBInfo for seid and emits a "delete" event.
// No-op (no event emitted) if seid is not found.
func (m *Manager) HandleDelete(seid uint64) {
	m.mu.Lock()
	rib, ok := m.ribs[seid]
	delete(m.ribs, seid)
	m.mu.Unlock()

	if ok {
		m.emit(&BGPEvent{Type: BGPEventDelete, Info: rib, SEID: seid})
	}
}

// Get returns the BGPRIBInfo for seid.
func (m *Manager) Get(seid uint64) (*BGPRIBInfo, bool) {
	m.mu.RLock()
	rib, ok := m.ribs[seid]
	m.mu.RUnlock()
	return rib, ok
}

// Len returns the number of stored BGPRIBInfo entries.
func (m *Manager) Len() int {
	m.mu.RLock()
	n := len(m.ribs)
	m.mu.RUnlock()
	return n
}

// synthesize creates a BGPRIBInfo by merging SessionInformation and StaticContext.
func (m *Manager) synthesize(info *SessionInformation, now time.Time) (*BGPRIBInfo, error) {
	if info.NetworkInstance == "" {
		// After a bare Establishment Request, the session state has no PDRs/FARs yet
		// so NetworkInstance is not yet known. The caller should retry on the first
		// Modification event, which will carry the actual network instance.
		return nil, fmt.Errorf("ir: NetworkInstance not yet known for SEID=%d (will populate on modification)", info.SEID)
	}

	ctx, err := m.sctx.GetContext(info.NetworkInstance)
	if err != nil {
		slog.Warn("ir: static context not found",
			"network_instance", info.NetworkInstance, "err", err)
		return nil, fmt.Errorf("ir: static context: %w", err)
	}

	return &BGPRIBInfo{
		SessionID:             info.SessionID,
		SEID:                  info.SEID,
		UEIPAddress:           info.UEIPAddress,
		UEPrefix:              info.UEPrefix,
		TEID:                  info.TEID,
		QFI:                   info.QFI,
		EndpointAddress:       info.EndpointAddress,
		NetworkInstance:       info.NetworkInstance,
		RD:                    ctx.RD,
		RT:                    ctx.RT,
		SourceAddress:         ctx.SourceAddress,
		MUPExtendedCommunity:  ctx.MUPExtendedCommunity,
		EndpointAddressLength: ctx.EndpointAddressLength,
		NexthopAddress:        ctx.NexthopAddress,
		CreatedAt:             now,
		UpdatedAt:             now,
		Source:                info.Source,
	}, nil
}

// emit sends a BGPEvent without blocking. Drops the event if the channel is full.
func (m *Manager) emit(ev *BGPEvent) {
	select {
	case m.events <- ev:
	default:
		slog.Warn("ir: event channel full, dropping BGP event",
			"seid", ev.SEID, "type", ev.Type)
	}
}
