package config_test

// Task 8.4: Fatal error handling unit tests (req 9.6).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/qooqle/mup-ribgen/pkg/config"
)

// TestLoad_InvalidJSON verifies that malformed JSON causes a fatal load error (req 9.6).
func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.json")
	if err := os.WriteFile(f, []byte("{invalid json}"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := config.Load(f)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestLoad_EmptyFile verifies that an empty JSON file causes a load error.
func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.json")
	if err := os.WriteFile(f, []byte(""), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := config.Load(f)
	if err == nil {
		t.Error("expected error for empty file")
	}
}

// TestLoad_UnknownFieldsIgnored verifies that unknown JSON fields are silently ignored.
func TestLoad_UnknownFieldsIgnored(t *testing.T) {
	dir := t.TempDir()
	f := writeJSON(t, dir, map[string]interface{}{
		"mode1_enabled":   true,
		"unknown_field_x": "value",
	})
	cfg, err := config.Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Mode1Enabled {
		t.Error("expected Mode1Enabled=true")
	}
}
