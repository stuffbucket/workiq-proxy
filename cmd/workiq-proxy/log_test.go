package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigDir(t *testing.T) {
	d := configDir()
	if d == "" {
		t.Skip("os.UserConfigDir not available")
	}
	if !strings.HasSuffix(d, filepath.Join("", appName)) {
		t.Errorf("configDir() = %q, want suffix %q", d, appName)
	}
}

func TestStateDir(t *testing.T) {
	d := stateDir()
	if d == "" {
		t.Skip("HOME not set")
	}
	if !strings.HasSuffix(d, filepath.Join("", appName)) {
		t.Errorf("stateDir() = %q, want suffix %q", d, appName)
	}
}

func TestStateDir_XDGOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	d := stateDir()
	want := filepath.Join(tmp, appName)
	if d != want {
		t.Errorf("stateDir() = %q, want %q", d, want)
	}
}

func TestResolveLogFile_Default(t *testing.T) {
	// With no --log-file override, resolveLogFile uses stateDir.
	old := cfg.LogFile
	cfg.LogFile = ""
	defer func() { cfg.LogFile = old }()

	p := resolveLogFile()
	if p == "" {
		t.Skip("stateDir not available")
	}
	if !strings.HasSuffix(p, "workiq-proxy.log") {
		t.Errorf("resolveLogFile() = %q, want suffix workiq-proxy.log", p)
	}
}

func TestResolveLogFile_Override(t *testing.T) {
	old := cfg.LogFile
	cfg.LogFile = "/tmp/custom.log"
	defer func() { cfg.LogFile = old }()

	if p := resolveLogFile(); p != "/tmp/custom.log" {
		t.Errorf("resolveLogFile() = %q, want /tmp/custom.log", p)
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no ansi", "hello world", "hello world"},
		{"bold red", "\x1b[1;31mError\x1b[0m", "Error"},
		{"multiple codes", "\x1b[32mOK\x1b[0m and \x1b[33mwarn\x1b[0m", "OK and warn"},
		{"empty", "", ""},
		{"only ansi", "\x1b[0m", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			w := stripANSI(&buf)
			n, err := w.Write([]byte(tt.input))
			if err != nil {
				t.Fatalf("Write error: %v", err)
			}
			if n != len(tt.input) {
				t.Errorf("Write returned %d, want %d", n, len(tt.input))
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInitDiskLog_WritesJSON(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "test.log")

	old := cfg
	cfg.LogFile = logPath
	cfg.NoLog = false
	defer func() {
		cfg = old
		diskLog = nil
	}()

	initDiskLog()
	if diskLog == nil {
		t.Fatal("initDiskLog did not set diskLog")
	}

	diskInfo("test-message", "key", "value")

	// Read back and verify it's valid JSON.
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("log file is empty")
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("log entry is not valid JSON: %v\nraw: %s", err, data)
	}
	if entry["msg"] != "test-message" {
		t.Errorf("msg = %v, want test-message", entry["msg"])
	}
	if entry["key"] != "value" {
		t.Errorf("key = %v, want value", entry["key"])
	}
	if entry["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", entry["level"])
	}
}

func TestInitDiskLog_NoLog(t *testing.T) {
	old := cfg
	cfg.NoLog = true
	defer func() {
		cfg = old
		diskLog = nil
	}()

	initDiskLog()
	if diskLog != nil {
		t.Error("initDiskLog should not create logger when --no-log is set")
	}
}

func TestLogMsg_UsesDiskLog(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "traffic.log")

	old := cfg
	cfg.LogFile = logPath
	cfg.NoLog = false
	defer func() {
		cfg = old
		diskLog = nil
	}()

	initDiskLog()
	logMsg(">>>", []byte(`{"method":"initialize"}`))

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if entry["msg"] != "mcp-traffic" {
		t.Errorf("msg = %v, want mcp-traffic", entry["msg"])
	}
	if entry["dir"] != ">>>" {
		t.Errorf("dir = %v, want >>>", entry["dir"])
	}
}

func TestDiskHelpers_NilLogger(t *testing.T) {
	old := diskLog
	diskLog = nil
	defer func() { diskLog = old }()

	// These should not panic when diskLog is nil.
	diskInfo("noop")
	diskWarn("noop")
	diskError("noop")
}
