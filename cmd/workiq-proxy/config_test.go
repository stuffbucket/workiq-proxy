package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigFilePath(t *testing.T) {
	p := configFilePath()
	if p == "" {
		t.Skip("os.UserConfigDir not available")
	}
	if !strings.HasSuffix(p, filepath.Join(appName, "config.json")) {
		t.Errorf("configFilePath() = %q, want suffix %s/config.json", p, appName)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	old := cfg
	defer func() { cfg = old }()

	// Point to empty dir so no config file is found.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loadConfig()

	if cfg.WorkiqCmd != DefaultWorkiqCmd {
		t.Errorf("WorkiqCmd = %q, want default %q", cfg.WorkiqCmd, DefaultWorkiqCmd)
	}
	if cfg.Port != DefaultPort {
		t.Errorf("Port = %d, want %d", cfg.Port, DefaultPort)
	}
	if cfg.NoLog {
		t.Error("NoLog should default to false")
	}
	if cfg.LogFile != "" {
		t.Errorf("LogFile = %q, want empty", cfg.LogFile)
	}
}

func TestLoadConfig_FromDisk(t *testing.T) {
	old := cfg
	defer func() { cfg = old }()

	// Write a config file.
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	dir := filepath.Join(tmp, appName)
	_ = os.MkdirAll(dir, 0o700)

	diskCfg := Config{
		WorkiqCmd: "/usr/local/bin/workiq",
		Port:      9999,
		NoLog:     true,
		LogFile:   "/tmp/custom.log",
	}
	data, _ := json.MarshalIndent(diskCfg, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "config.json"), data, 0o600)

	loadConfig()

	if cfg.WorkiqCmd != "/usr/local/bin/workiq" {
		t.Errorf("WorkiqCmd = %q, want /usr/local/bin/workiq", cfg.WorkiqCmd)
	}
	if cfg.Port != 9999 {
		t.Errorf("Port = %d, want 9999", cfg.Port)
	}
	if !cfg.NoLog {
		t.Error("NoLog should be true from disk")
	}
	if cfg.LogFile != "/tmp/custom.log" {
		t.Errorf("LogFile = %q, want /tmp/custom.log", cfg.LogFile)
	}
}

func TestLoadConfig_PartialDisk(t *testing.T) {
	old := cfg
	defer func() { cfg = old }()

	// Only set port on disk; other fields should get defaults.
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	dir := filepath.Join(tmp, appName)
	_ = os.MkdirAll(dir, 0o700)
	_ = os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"port": 7777}`), 0o600)

	loadConfig()

	if cfg.WorkiqCmd != DefaultWorkiqCmd {
		t.Errorf("WorkiqCmd = %q, want default %q", cfg.WorkiqCmd, DefaultWorkiqCmd)
	}
	if cfg.Port != 7777 {
		t.Errorf("Port = %d, want 7777", cfg.Port)
	}
}

func TestLoadConfig_BadJSON(t *testing.T) {
	old := cfg
	defer func() { cfg = old }()

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	dir := filepath.Join(tmp, appName)
	_ = os.MkdirAll(dir, 0o700)
	_ = os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{invalid json`), 0o600)

	// Should not panic; falls back to defaults.
	loadConfig()

	if cfg.WorkiqCmd != DefaultWorkiqCmd {
		t.Errorf("WorkiqCmd = %q, want default %q after bad JSON", cfg.WorkiqCmd, DefaultWorkiqCmd)
	}
}

func TestSaveConfig(t *testing.T) {
	old := cfg
	defer func() { cfg = old }()

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg = Config{
		WorkiqCmd: "custom-cmd",
		Port:      8080,
		NoLog:     true,
		LogFile:   "/var/log/test.log",
	}

	if err := saveConfig(); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}

	// Read back.
	data, err := os.ReadFile(filepath.Join(tmp, appName, "config.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if loaded.WorkiqCmd != "custom-cmd" {
		t.Errorf("WorkiqCmd = %q, want custom-cmd", loaded.WorkiqCmd)
	}
	if loaded.Port != 8080 {
		t.Errorf("Port = %d, want 8080", loaded.Port)
	}
}

func TestSaveConfig_RoundTrip(t *testing.T) {
	old := cfg
	defer func() { cfg = old }()

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg = Config{
		WorkiqCmd: "roundtrip-cmd",
		Port:      5555,
	}
	if err := saveConfig(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Load it back.
	loadConfig()

	if cfg.WorkiqCmd != "roundtrip-cmd" {
		t.Errorf("after round-trip: WorkiqCmd = %q", cfg.WorkiqCmd)
	}
	if cfg.Port != 5555 {
		t.Errorf("after round-trip: Port = %d", cfg.Port)
	}
}
