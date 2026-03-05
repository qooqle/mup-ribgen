package staticctx_test

// Property 18: Static Context loading (req 7.1–7.7)
// Property 19: JSON Schema validation (req 7.8, 7.9)

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/qooqle/mup-ribgen/pkg/staticctx"
)

// --- helpers ----------------------------------------------------------------

type niEntry struct {
	Name    string
	RD      string
	RT      []string
	Nexthop string
}

// genNetworkInstanceName generates valid Network Instance names (alphanumeric + - _).
func genNetworkInstanceName() gopter.Gen {
	return gen.RegexMatch(`[a-zA-Z][a-zA-Z0-9_-]{1,15}`)
}

// genASN generates a simple AS:NN string like "65000:1".
func genASN() gopter.Gen {
	return gopter.CombineGens(
		gen.UInt16Range(1, 65535),
		gen.UInt16Range(1, 65535),
	).Map(func(vals []interface{}) string {
		return fmt.Sprintf("%d:%d", vals[0].(uint16), vals[1].(uint16))
	})
}

// writeConfig writes a minimal valid config JSON to a temp file and returns
// the path.
func writeConfig(t *testing.T, entries []niEntry) string {
	t.Helper()
	ni := map[string]map[string]interface{}{}
	for _, e := range entries {
		ni[e.Name] = map[string]interface{}{
			"rd":      e.RD,
			"rt":      e.RT,
			"nexthop": e.Nexthop,
		}
	}
	cfg := map[string]interface{}{"network_instances": ni}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("writeConfig: marshal: %v", err)
	}
	f := filepath.Join(t.TempDir(), "static-context.json")
	if err := os.WriteFile(f, data, 0o600); err != nil {
		t.Fatalf("writeConfig: write: %v", err)
	}
	return f
}

// --- Property 18: Static Context loading ------------------------------------

// Property 18: For any valid set of Network Instance entries written to a
// config file, Load must succeed and GetContext must return the correct
// RD, RT, and Nexthop values (req 7.1–7.7).
func TestProperty18_StaticContextLoading(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Req 7.1: config is loaded from JSON file
	// Req 7.2: RD mapping is preserved
	// Req 7.3: RT mapping is preserved
	// Req 7.7: Nexthop is preserved
	properties.Property("req7.1-7.3,7.7: Load preserves RD, RT, Nexthop", prop.ForAll(
		func(name, rd, nexthop string, rt1, rt2 uint16) bool {
			rd = fmt.Sprintf("%d:%d", rt1, rt2)
			rt := []string{fmt.Sprintf("%d:%d", rt1, rt2)}
			nexthop = "2001:db8::1"
			entry := niEntry{Name: sanitizeName(name), RD: rd, RT: rt, Nexthop: nexthop}
			if entry.Name == "" {
				return true // skip degenerate inputs
			}
			f := writeConfig(t, []niEntry{entry})
			m := staticctx.New()
			if err := m.Load(f); err != nil {
				return false
			}
			ctx, err := m.GetContext(entry.Name)
			if err != nil {
				return false
			}
			return ctx.RD == rd &&
				len(ctx.RT) == 1 && ctx.RT[0] == rt[0] &&
				ctx.NexthopAddress == nexthop &&
				ctx.NetworkInstance == entry.Name
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) >= 2 }),
		gen.AlphaString(),
		gen.AlphaString(),
		gen.UInt16Range(1, 65535),
		gen.UInt16Range(1, 65535),
	))

	// Req 7.4: SourceAddress (optional) is correctly loaded when present
	properties.Property("req7.4: optional SourceAddress is loaded", prop.ForAll(
		func(n uint16) bool {
			rd := fmt.Sprintf("%d:1", n)
			ni := map[string]interface{}{
				"rd": rd, "rt": []string{rd},
				"nexthop":        "2001:db8::1",
				"source_address": "2001:db8:1::1",
			}
			cfg := map[string]interface{}{"network_instances": map[string]interface{}{"vrf-a": ni}}
			data, _ := json.Marshal(cfg)
			f := filepath.Join(t.TempDir(), "cfg.json")
			_ = os.WriteFile(f, data, 0o600)
			m := staticctx.New()
			if err := m.Load(f); err != nil {
				return false
			}
			ctx, err := m.GetContext("vrf-a")
			if err != nil {
				return false
			}
			return ctx.SourceAddress != nil && *ctx.SourceAddress == "2001:db8:1::1"
		},
		gen.UInt16Range(1, 65535),
	))

	// Req 7.5 & 7.6: MUPExtendedCommunity and EndpointAddressLength are loaded
	properties.Property("req7.5,7.6: MUPExtendedCommunity and EndpointAddressLength loaded", prop.ForAll(
		func(n uint16, addrLen int) bool {
			if addrLen < 32 || addrLen > 160 {
				addrLen = 64
			}
			rd := fmt.Sprintf("%d:1", n)
			ni := map[string]interface{}{
				"rd": rd, "rt": []string{rd},
				"nexthop": "2001:db8::1",
				"mup_extended_community": map[string]interface{}{
					"segment_identifier": "000000000001",
				},
				"endpoint_address_length": addrLen,
			}
			cfg := map[string]interface{}{"network_instances": map[string]interface{}{"vrf-b": ni}}
			data, _ := json.Marshal(cfg)
			f := filepath.Join(t.TempDir(), "cfg.json")
			_ = os.WriteFile(f, data, 0o600)
			m := staticctx.New()
			if err := m.Load(f); err != nil {
				return false
			}
			ctx, err := m.GetContext("vrf-b")
			if err != nil {
				return false
			}
			return ctx.MUPExtendedCommunity != nil &&
				ctx.EndpointAddressLength != nil &&
				*ctx.EndpointAddressLength == addrLen
		},
		gen.UInt16Range(1, 65535),
		gen.IntRange(32, 160),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// --- Property 19: JSON Schema validation ------------------------------------

// Property 19: Files missing required fields must fail validation.
// Files with all required fields must pass validation (req 7.8, 7.9).
func TestProperty19_JSONSchemaValidation(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Req 7.8 & 7.9: valid config passes, missing required field fails
	properties.Property("req7.8: valid config passes schema validation", prop.ForAll(
		func(n uint16) bool {
			rd := fmt.Sprintf("%d:1", n)
			ni := map[string]interface{}{
				"rd": rd, "rt": []string{rd},
				"nexthop": "2001:db8::1",
			}
			cfg := map[string]interface{}{"network_instances": map[string]interface{}{"vrf-valid": ni}}
			data, _ := json.Marshal(cfg)
			f := filepath.Join(t.TempDir(), "cfg.json")
			_ = os.WriteFile(f, data, 0o600)
			m := staticctx.New()
			return m.Validate(f) == nil
		},
		gen.UInt16Range(1, 65535),
	))

	properties.Property("req7.9: missing 'rd' fails schema validation", prop.ForAll(
		func(n uint16) bool {
			rd := fmt.Sprintf("%d:1", n)
			ni := map[string]interface{}{
				// rd is intentionally omitted
				"rt":      []string{rd},
				"nexthop": "2001:db8::1",
			}
			cfg := map[string]interface{}{"network_instances": map[string]interface{}{"vrf-bad": ni}}
			data, _ := json.Marshal(cfg)
			f := filepath.Join(t.TempDir(), "cfg.json")
			_ = os.WriteFile(f, data, 0o600)
			m := staticctx.New()
			return m.Validate(f) != nil // must error
		},
		gen.UInt16Range(1, 65535),
	))

	properties.Property("req7.9: missing 'rt' fails schema validation", prop.ForAll(
		func(n uint16) bool {
			rd := fmt.Sprintf("%d:1", n)
			ni := map[string]interface{}{
				"rd":      rd,
				"nexthop": "2001:db8::1",
				// rt intentionally omitted
			}
			cfg := map[string]interface{}{"network_instances": map[string]interface{}{"vrf-bad": ni}}
			data, _ := json.Marshal(cfg)
			f := filepath.Join(t.TempDir(), "cfg.json")
			_ = os.WriteFile(f, data, 0o600)
			m := staticctx.New()
			return m.Validate(f) != nil
		},
		gen.UInt16Range(1, 65535),
	))

	properties.Property("req7.9: missing 'nexthop' fails schema validation", prop.ForAll(
		func(n uint16) bool {
			rd := fmt.Sprintf("%d:1", n)
			ni := map[string]interface{}{
				"rd": rd,
				"rt": []string{rd},
				// nexthop intentionally omitted
			}
			cfg := map[string]interface{}{"network_instances": map[string]interface{}{"vrf-bad": ni}}
			data, _ := json.Marshal(cfg)
			f := filepath.Join(t.TempDir(), "cfg.json")
			_ = os.WriteFile(f, data, 0o600)
			m := staticctx.New()
			return m.Validate(f) != nil
		},
		gen.UInt16Range(1, 65535),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// sanitizeName ensures the name matches the schema pattern [a-zA-Z0-9_-]+.
func sanitizeName(s string) string {
	out := make([]byte, 0, len(s))
	for _, c := range []byte(s) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return ""
	}
	return string(out)
}
