// Package config manages the main application configuration file (req 11.2, 11.3).
// It supports loading from an explicit path, falling back to ./config.json,
// and environment-variable overrides.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

const defaultConfigPath = "./config.json"

// Config holds the top-level application configuration.
type Config struct {
	// Mode toggles
	Mode1Enabled bool `json:"mode1_enabled"`
	Mode2Enabled bool `json:"mode2_enabled"`

	// Dialect used in Mode 1 (name of a compiled Dialect Transformer)
	Dialect string `json:"dialect,omitempty"`

	// Log level: DEBUG, INFO, WARN, ERROR
	LogLevel string `json:"log_level,omitempty"`

	// Path to the static context config file
	StaticContextFile string `json:"static_context_file,omitempty"`

	// GoBGP address (host:port)
	GoBGPAddress string `json:"gobgp_address,omitempty"`

	// PFCP capture interface (Mode 1)
	PFCPInterface string `json:"pfcp_interface,omitempty"`

	// Listening address for Mode 2 gRPC / HTTP API
	ListenAddress string `json:"listen_address,omitempty"`
}

// Load reads the configuration from path. If path is empty it tries the
// MUP_CONFIG_PATH environment variable, then falls back to defaultConfigPath.
// After loading from disk, environment variables may override individual fields:
//
//	MUP_LOG_LEVEL   overrides Config.LogLevel
//	MUP_GOBGP_ADDR  overrides Config.GoBGPAddress
func Load(path string) (*Config, error) {
	resolved := resolvePath(path)
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("config: read %q: %w", resolved, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %q: %w", resolved, err)
	}
	applyEnvOverrides(&cfg)
	return &cfg, nil
}

// resolvePath picks the effective config file path.
func resolvePath(path string) string {
	if path != "" {
		return path
	}
	if env := os.Getenv("MUP_CONFIG_PATH"); env != "" {
		return env
	}
	return defaultConfigPath
}

// applyEnvOverrides overrides selected fields with environment variables.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("MUP_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("MUP_GOBGP_ADDR"); v != "" {
		cfg.GoBGPAddress = v
	}
}
