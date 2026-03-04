package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strconv"
)

// Compile-time constants — single source of truth for identity strings
// used in MCP handshakes and user-facing output.
const (
	Version          = "0.3.4"
	MCPProtocol      = "2025-11-25"
	DefaultWorkiqCmd = "npx -y @microsoft/workiq"
	DefaultPort      = 11435
)

// Config holds all workiq-proxy settings. Values are loaded in this
// order, with later sources winning:
//
//  1. Compiled defaults (struct zero values + field defaults below)
//  2. JSON config file  ($XDG_CONFIG_HOME/workiq-proxy/config.json)
//  3. Command-line flags (only those explicitly passed)
type Config struct {
	WorkiqCmd string `json:"workiq_cmd"` // Base command to reach the Work IQ CLI
	LogFile   string `json:"log_file"`   // Override log file path ("" = default XDG state dir)
	NoLog     bool   `json:"no_log"`     // Disable traffic logging
	Port      int    `json:"port"`       // Port for serve/web mode
}

// cfg is the global config instance, available after loadConfig().
var cfg Config

// configFilePath returns the path to the JSON config file.
func configFilePath() string {
	d := configDir()
	if d == "" {
		return ""
	}
	return filepath.Join(d, "config.json")
}

// loadConfig reads the config file (if it exists), applies compiled
// defaults for any missing fields, then overlays any CLI flags that
// were explicitly set by the user.
func loadConfig() {
	// 1. Compiled defaults.
	cfg = Config{
		WorkiqCmd: DefaultWorkiqCmd,
		Port:      DefaultPort,
	}

	// 2. JSON file on disk — silently skip if missing or unreadable.
	if p := configFilePath(); p != "" {
		if data, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(data, &cfg)
		}
	}

	// 3. CLI flag overrides — only apply flags the user actually passed.
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "workiq-cmd":
			cfg.WorkiqCmd = f.Value.String()
		case "log-file":
			cfg.LogFile = f.Value.String()
		case "no-log":
			cfg.NoLog = f.Value.String() == "true"
		case "port":
			if n, err := strconv.Atoi(f.Value.String()); err == nil && n > 0 {
				cfg.Port = n
			}
		}
	})
}

// saveConfig writes the current config to the JSON file, creating
// the config directory if needed.
func saveConfig() error {
	p := configFilePath()
	if p == "" {
		return nil
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(p, data, 0o600)
}
