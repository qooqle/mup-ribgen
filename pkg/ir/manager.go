// Package ir defines the Session Information data models and the IR Manager.
// This file implements the IR Manager (req 4.6, 4.7).
package ir

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
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
	// RouteKey identifies the route instance.
	RouteKey string
	// SEID is always set (even for delete events where Info is nil).
	SEID uint64
}

// StaticContextProvider is implemented by staticctx.Manager.
type StaticContextProvider interface {
	GetContext(networkInstance string) (*StaticContext, error)
}

// Manager synthesizes SessionInformation + StaticContext → BGPRIBInfo and
// maintains an in-memory store keyed by RouteKey (req 4.6, 4.7).
// It emits BGPEvents on a buffered channel for downstream consumers.
type Manager struct {
	mu        sync.RWMutex
	ribs      map[string]*BGPRIBInfo
	seidIndex map[uint64]map[string]struct{}
	sctx      StaticContextProvider
	events    chan *BGPEvent

	pendingDelete map[string]time.Time
	pendingTTL    time.Duration
}

// NewManager creates an IR Manager with the given StaticContextProvider.
// bufSize controls the BGPEvent channel buffer; defaults to 256 if ≤ 0.
func NewManager(sctx StaticContextProvider, bufSize int) *Manager {
	if bufSize <= 0 {
		bufSize = 256
	}
	return &Manager{
		ribs:          make(map[string]*BGPRIBInfo),
		seidIndex:     make(map[uint64]map[string]struct{}),
		sctx:          sctx,
		events:        make(chan *BGPEvent, bufSize),
		pendingDelete: make(map[string]time.Time),
		pendingTTL:    5 * time.Second,
	}
}

// StartPendingDeleteWatcher starts a goroutine that deletes expired pending entries.
// It should be called once during startup.
func (m *Manager) StartPendingDeleteWatcher(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				m.expirePendingDeletes(now)
			}
		}
	}()
}

func (m *Manager) expirePendingDeletes(now time.Time) {
	var expired []string
	m.mu.RLock()
	for routeKey, deadline := range m.pendingDelete {
		if !deadline.IsZero() && now.After(deadline) {
			expired = append(expired, routeKey)
		}
	}
	m.mu.RUnlock()
	for _, routeKey := range expired {
		slog.Info("ir: pending delete expired", "route_key", routeKey)
		m.handleDeleteRouteKey(routeKey)
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

	routeKey := routeKeyOf(rib)
	m.mu.Lock()
	m.ribs[routeKey] = rib
	m.indexSEIDLocked(rib.SEID, routeKey)
	m.mu.Unlock()

	m.emit(&BGPEvent{Type: BGPEventCreate, Info: rib, RouteKey: routeKey, SEID: rib.SEID})
	return nil
}

// HandleUpdate updates the BGPRIBInfo for a route instance and emits an "update" event.
// Preserves the original CreatedAt timestamp if an existing entry is found.
// Returns an error if the StaticContext for info.NetworkInstance is not found.
func (m *Manager) HandleUpdate(info *SessionInformation) error {
	routeKey := routeKeyOfInfo(info)
	m.mu.RLock()
	existing, hasExisting := m.ribs[routeKey]
	m.mu.RUnlock()

	rib, err := m.synthesize(info, time.Now())
	if err != nil {
		return err
	}
	// If forwarding info is missing, mark pending delete and skip update.
	if rib.EndpointAddress == "" || rib.TEID == 0 {
		m.mu.Lock()
		m.pendingDelete[routeKey] = time.Now().Add(m.pendingTTL)
		m.mu.Unlock()
		return nil
	}
	// Forwarding info restored: clear pending delete if any.
	m.mu.Lock()
	delete(m.pendingDelete, routeKey)
	m.mu.Unlock()
	if hasExisting {
		rib.CreatedAt = existing.CreatedAt
		m.mu.Lock()
		m.ribs[routeKey] = rib
		m.indexSEIDLocked(rib.SEID, routeKey)
		m.mu.Unlock()

		m.emit(&BGPEvent{Type: BGPEventUpdate, Info: rib, RouteKey: routeKey, SEID: rib.SEID})
		return nil
	}

	// If we receive a Modification before a usable Establishment (e.g. missing NetworkInstance),
	// treat this as a create to avoid dropping the first usable RIB entry.
	m.mu.Lock()
	m.ribs[routeKey] = rib
	m.indexSEIDLocked(rib.SEID, routeKey)
	m.mu.Unlock()

	m.emit(&BGPEvent{Type: BGPEventCreate, Info: rib, RouteKey: routeKey, SEID: rib.SEID})
	return nil
}

// HandleDelete removes all BGPRIBInfo entries for seid and emits delete events.
// No-op (no event emitted) if seid is not found.
func (m *Manager) HandleDelete(seid uint64) {
	var keys []string
	m.mu.RLock()
	for k := range m.seidIndex[seid] {
		keys = append(keys, k)
	}
	m.mu.RUnlock()
	sort.Strings(keys)
	for _, k := range keys {
		m.handleDeleteRouteKey(k)
	}
}

func (m *Manager) handleDeleteRouteKey(routeKey string) {
	m.mu.Lock()
	rib, ok := m.ribs[routeKey]
	delete(m.ribs, routeKey)
	delete(m.pendingDelete, routeKey)
	if ok {
		m.deindexSEIDLocked(rib.SEID, routeKey)
	}
	m.mu.Unlock()

	if ok {
		m.emit(&BGPEvent{Type: BGPEventDelete, Info: rib, RouteKey: routeKey, SEID: rib.SEID})
	}
}

// Get returns one BGPRIBInfo for seid (deterministically the smallest RouteKey).
func (m *Manager) Get(seid uint64) (*BGPRIBInfo, bool) {
	var keys []string
	m.mu.RLock()
	for k := range m.seidIndex[seid] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var (
		rib *BGPRIBInfo
		ok  bool
	)
	if len(keys) > 0 {
		rib, ok = m.ribs[keys[0]]
	}
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
		RouteKey:             routeKeyOfInfo(info),
		FARID:                info.FARID,
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

func (m *Manager) indexSEIDLocked(seid uint64, routeKey string) {
	if m.seidIndex[seid] == nil {
		m.seidIndex[seid] = make(map[string]struct{})
	}
	m.seidIndex[seid][routeKey] = struct{}{}
}

func (m *Manager) deindexSEIDLocked(seid uint64, routeKey string) {
	keys := m.seidIndex[seid]
	if keys == nil {
		return
	}
	delete(keys, routeKey)
	if len(keys) == 0 {
		delete(m.seidIndex, seid)
	}
}

func routeKeyOfInfo(info *SessionInformation) string {
	if info.RouteKey != "" {
		return info.RouteKey
	}
	if info.FARID != 0 {
		return fmt.Sprintf("%d:%d", info.SEID, info.FARID)
	}
	return fmt.Sprintf("%d", info.SEID)
}

func routeKeyOf(rib *BGPRIBInfo) string {
	if rib.RouteKey != "" {
		return rib.RouteKey
	}
	if rib.FARID != 0 {
		return fmt.Sprintf("%d:%d", rib.SEID, rib.FARID)
	}
	return fmt.Sprintf("%d", rib.SEID)
}
