package config_test

// Properties 1 and 27: Mode selection and CLI arguments (req 1.1, 1.4, 1.5, 11.2).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/qooqle/mup-ribgen/pkg/config"
)

// TestProperty1_ModeSelection verifies req 1.1, 1.4, 1.5:
// Mode1 and Mode2 can be independently enabled/disabled, simultaneously enabled,
// and mode settings are correctly loaded from the config file at startup.
func TestProperty1_ModeSelection(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Property 1a: Mode1 and Mode2 flags are independently settable in-memory (req 1.1).
	properties.Property("Property1a: Mode1 and Mode2 are independently configurable", prop.ForAll(
		func(mode1, mode2 bool) bool {
			cfg := &config.Config{Mode1Enabled: mode1, Mode2Enabled: mode2}
			return cfg.Mode1Enabled == mode1 && cfg.Mode2Enabled == mode2
		},
		gen.Bool(),
		gen.Bool(),
	))

	// Property 1b: Mode toggles survive a config-file load round-trip (req 1.5).
	properties.Property("Property1b: mode settings survive JSON config round-trip", prop.ForAll(
		func(mode1, mode2 bool) bool {
			dir, err := os.MkdirTemp("", "mup-prop1b-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(dir)

			f := filepath.Join(dir, "config.json")
			data, _ := json.Marshal(map[string]interface{}{
				"mode1_enabled": mode1,
				"mode2_enabled": mode2,
			})
			if err := os.WriteFile(f, data, 0o600); err != nil {
				return false
			}

			cfg, err := config.Load(f)
			if err != nil {
				return false
			}
			return cfg.Mode1Enabled == mode1 && cfg.Mode2Enabled == mode2
		},
		gen.Bool(),
		gen.Bool(),
	))

	// Property 1c: both modes can be simultaneously enabled without conflict (req 1.4).
	properties.Property("Property1c: Mode1 and Mode2 can be simultaneously enabled", prop.ForAll(
		func(mode1, mode2 bool) bool {
			dir, err := os.MkdirTemp("", "mup-prop1c-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(dir)

			f := filepath.Join(dir, "config.json")
			data, _ := json.Marshal(map[string]interface{}{
				"mode1_enabled": mode1,
				"mode2_enabled": mode2,
			})
			if err := os.WriteFile(f, data, 0o600); err != nil {
				return false
			}

			cfg, err := config.Load(f)
			if err != nil {
				return false
			}
			// No mutual exclusion: both can be true simultaneously
			return cfg.Mode1Enabled == mode1 && cfg.Mode2Enabled == mode2
		},
		gen.Bool(),
		gen.Bool(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty27_CLIArguments verifies req 11.2:
// the config file path specified as an argument is loaded correctly,
// a non-existent path produces an error, and MUP_CONFIG_PATH is honoured.
func TestProperty27_CLIArguments(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Property 27a: explicit config path yields correct settings (req 11.2).
	properties.Property("Property27a: explicit config path loads correct config", prop.ForAll(
		func(mode1 bool, logLevel string) bool {
			dir, err := os.MkdirTemp("", "mup-prop27a-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(dir)

			f := filepath.Join(dir, "config.json")
			data, _ := json.Marshal(map[string]interface{}{
				"mode1_enabled": mode1,
				"log_level":    logLevel,
			})
			if err := os.WriteFile(f, data, 0o600); err != nil {
				return false
			}

			cfg, err := config.Load(f)
			if err != nil {
				return false
			}
			return cfg.Mode1Enabled == mode1 && cfg.LogLevel == logLevel
		},
		gen.Bool(),
		gen.OneConstOf("DEBUG", "INFO", "WARN", "ERROR"),
	))

	// Property 27b: a non-existent config path always returns an error.
	properties.Property("Property27b: missing config path returns error", prop.ForAll(
		func(suffix string) bool {
			_, err := config.Load(fmt.Sprintf("/nonexistent-mup-test/%s/config.json", suffix))
			return err != nil
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property 27c: MUP_CONFIG_PATH environment variable is used when path is empty.
	properties.Property("Property27c: MUP_CONFIG_PATH env var is used when path is empty", prop.ForAll(
		func(mode1 bool) bool {
			dir, err := os.MkdirTemp("", "mup-prop27c-*")
			if err != nil {
				return false
			}
			defer func() {
				os.RemoveAll(dir)
				os.Unsetenv("MUP_CONFIG_PATH")
			}()

			f := filepath.Join(dir, "config.json")
			data, _ := json.Marshal(map[string]interface{}{"mode1_enabled": mode1})
			if err := os.WriteFile(f, data, 0o600); err != nil {
				return false
			}
			os.Setenv("MUP_CONFIG_PATH", f)

			cfg, err := config.Load("") // empty → use env var
			if err != nil {
				return false
			}
			return cfg.Mode1Enabled == mode1
		},
		gen.Bool(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
