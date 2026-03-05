package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/qooqle/mup-ribgen/pkg/config"
)

func writeJSON(t *testing.T, dir string, v interface{}) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	f := filepath.Join(dir, "config.json")
	if err := os.WriteFile(f, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return f
}

// TestLoad_ExplicitPath verifies that an explicit path is used when provided.
func TestLoad_ExplicitPath(t *testing.T) {
	dir := t.TempDir()
	f := writeJSON(t, dir, map[string]interface{}{
		"mode1_enabled": true,
		"log_level":     "DEBUG",
	})
	cfg, err := config.Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Mode1Enabled {
		t.Error("expected Mode1Enabled=true")
	}
	if cfg.LogLevel != "DEBUG" {
		t.Errorf("expected LogLevel=DEBUG, got %q", cfg.LogLevel)
	}
}

// TestLoad_EnvPath verifies MUP_CONFIG_PATH env override.
func TestLoad_EnvPath(t *testing.T) {
	dir := t.TempDir()
	f := writeJSON(t, dir, map[string]interface{}{"mode2_enabled": true})
	t.Setenv("MUP_CONFIG_PATH", f)

	cfg, err := config.Load("") // empty path → use env
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Mode2Enabled {
		t.Error("expected Mode2Enabled=true")
	}
}

// TestLoad_DefaultPath verifies that ./config.json is tried when no path given.
func TestLoad_DefaultPath(t *testing.T) {
	// Change cwd to a temp dir so ./config.json resolves there.
	dir := t.TempDir()
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Skip("cannot chdir:", err)
	}
	writeJSON(t, dir, map[string]interface{}{"gobgp_address": "localhost:50051"})

	t.Setenv("MUP_CONFIG_PATH", "") // ensure env doesn't interfere
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GoBGPAddress != "localhost:50051" {
		t.Errorf("got %q", cfg.GoBGPAddress)
	}
}

// TestLoad_EnvOverride_LogLevel verifies MUP_LOG_LEVEL overrides the file value.
func TestLoad_EnvOverride_LogLevel(t *testing.T) {
	dir := t.TempDir()
	f := writeJSON(t, dir, map[string]interface{}{"log_level": "INFO"})
	t.Setenv("MUP_LOG_LEVEL", "ERROR")

	cfg, err := config.Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LogLevel != "ERROR" {
		t.Errorf("expected ERROR, got %q", cfg.LogLevel)
	}
}

// TestLoad_EnvOverride_GoBGP verifies MUP_GOBGP_ADDR overrides the file value.
func TestLoad_EnvOverride_GoBGP(t *testing.T) {
	dir := t.TempDir()
	f := writeJSON(t, dir, map[string]interface{}{"gobgp_address": "10.0.0.1:50051"})
	t.Setenv("MUP_GOBGP_ADDR", "192.168.1.1:50051")

	cfg, err := config.Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GoBGPAddress != "192.168.1.1:50051" {
		t.Errorf("got %q", cfg.GoBGPAddress)
	}
}

// TestLoad_MissingFile verifies that a missing file returns an error.
func TestLoad_MissingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/path/config.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
