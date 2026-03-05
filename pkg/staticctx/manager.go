// Package staticctx implements the Static Context Manager.
// It loads and validates per-Network-Instance configuration from a JSON file
// and provides lookup by Network Instance name.
package staticctx

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/qooqle/mup-ribgen/pkg/ir"
	"github.com/xeipuuv/gojsonschema"
)

//go:embed schema.json
var schemaJSON []byte

// Manager loads and serves StaticContext entries keyed by Network Instance.
type Manager struct {
	mu       sync.RWMutex
	contexts map[string]*ir.StaticContext
}

// New creates an empty Manager.
func New() *Manager {
	return &Manager{contexts: make(map[string]*ir.StaticContext)}
}

// Load validates and loads the static context configuration file at configFile.
// Returns an error if the file is missing, not valid JSON, or fails schema
// validation. On success it replaces the currently held contexts atomically.
func (m *Manager) Load(configFile string) error {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("staticctx: read config: %w", err)
	}
	if err := validate(data); err != nil {
		return err
	}
	contexts, err := parse(data)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.contexts = contexts
	m.mu.Unlock()
	return nil
}

// Validate runs JSON Schema validation on configFile without updating state.
func (m *Manager) Validate(configFile string) error {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("staticctx: read config: %w", err)
	}
	return validate(data)
}

// GetContext returns the StaticContext for the given networkInstance.
// Returns an error if no entry is found.
func (m *Manager) GetContext(networkInstance string) (*ir.StaticContext, error) {
	m.mu.RLock()
	ctx, ok := m.contexts[networkInstance]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("staticctx: network instance %q not found", networkInstance)
	}
	return ctx, nil
}

// Reload re-reads the configuration from the same path used in the last Load
// call. This method is a stub for future use; callers should call Load again.
func (m *Manager) Reload(configFile string) error {
	return m.Load(configFile)
}

// --- internal helpers -------------------------------------------------------

// rawConfig mirrors the JSON structure of the config file for unmarshalling.
type rawConfig struct {
	NetworkInstances map[string]rawNetworkInstance `json:"network_instances"`
}

type rawNetworkInstance struct {
	RD                    string                 `json:"rd"`
	RT                    []string               `json:"rt"`
	SourceAddress         *string                `json:"source_address,omitempty"`
	MUPExtendedCommunity  *rawMUPExtComm         `json:"mup_extended_community,omitempty"`
	EndpointAddressLength *int                   `json:"endpoint_address_length,omitempty"`
	Nexthop               string                 `json:"nexthop"`
}

type rawMUPExtComm struct {
	SegmentIdentifier string `json:"segment_identifier"`
}

func validate(data []byte) error {
	schemaLoader := gojsonschema.NewBytesLoader(schemaJSON)
	docLoader := gojsonschema.NewBytesLoader(data)

	result, err := gojsonschema.Validate(schemaLoader, docLoader)
	if err != nil {
		return fmt.Errorf("staticctx: schema validation error: %w", err)
	}
	if !result.Valid() {
		msg := "staticctx: config file failed JSON Schema validation:"
		for _, e := range result.Errors() {
			msg += "\n  - " + e.String()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func parse(data []byte) (map[string]*ir.StaticContext, error) {
	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("staticctx: unmarshal: %w", err)
	}

	contexts := make(map[string]*ir.StaticContext, len(raw.NetworkInstances))
	for name, ni := range raw.NetworkInstances {
		ctx := &ir.StaticContext{
			NetworkInstance:       name,
			RD:                    ni.RD,
			RT:                    ni.RT,
			SourceAddress:         ni.SourceAddress,
			EndpointAddressLength: ni.EndpointAddressLength,
			NexthopAddress:        ni.Nexthop,
		}
		if ni.MUPExtendedCommunity != nil {
			b, err := hex.DecodeString(ni.MUPExtendedCommunity.SegmentIdentifier)
			if err != nil || len(b) != 6 {
				return nil, fmt.Errorf("staticctx: invalid segment_identifier for %q", name)
			}
			comm := &ir.MUPExtendedCommunity{}
			copy(comm.SegmentIdentifier[:], b)
			ctx.MUPExtendedCommunity = comm
		}
		contexts[name] = ctx
	}
	return contexts, nil
}
