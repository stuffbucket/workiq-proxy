package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/natefinch/lumberjack.v2"
)

// ── XDG-compliant directory helpers ─────────────────────────────────
//
// configDir → $XDG_CONFIG_HOME/workiq-proxy  (settings JSON)
// stateDir  → $XDG_STATE_HOME/workiq-proxy   (logs, runtime state)
//
// On macOS os.UserConfigDir returns ~/Library/Application Support and
// there is no stdlib UserStateDir, so we fall back sensibly.

const appName = "workiq-proxy"

func configDir() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, appName)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, appName)
}

func stateDir() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, appName)
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "state", appName)
}

// ── Disk logger (structured JSON, rotated) ──────────────────────────
//
// diskLog is a *slog.Logger that writes JSON to a rotated log file.
// It is nil until initDiskLog is called, or when logging is disabled.

var diskLog *slog.Logger

// initDiskLog sets up the JSON disk logger backed by lumberjack.
// MaxSize 5 MB × 1 backup ≈ 10 MB ceiling on disk.
func initDiskLog() {
	if cfg.NoLog {
		return
	}
	p := resolveLogFile()
	if p == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o700)

	w := &lumberjack.Logger{
		Filename:   p,
		MaxSize:    5, // megabytes per file
		MaxBackups: 1, // keep 1 rotated file → ~10 MB ceiling
		MaxAge:     0, // don't delete by age
		Compress:   false,
	}
	diskLog = slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

func resolveLogFile() string {
	if cfg.LogFile != "" {
		return cfg.LogFile
	}
	dir := stateDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "workiq-proxy.log")
}

// ── Legacy traffic logger (MCP proxy mode) ──────────────────────────
//
// logMsg is used by the MCP proxy relay to log raw JSON-RPC traffic.
// It writes to diskLog when available.

func logMsg(direction string, data []byte) {
	if cfg.NoLog || diskLog == nil {
		return
	}
	diskLog.Info("mcp-traffic",
		"dir", direction,
		"payload", string(data),
	)
}

// ── ANSI stripping ──────────────────────────────────────────────────

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

type ansiStripper struct {
	w io.Writer
}

func (s *ansiStripper) Write(p []byte) (int, error) {
	clean := ansiRe.ReplaceAll(p, nil)
	_, err := s.w.Write(clean)
	// Return original len so callers don't think it was short.
	return len(p), err
}

// stripANSI wraps w so that ANSI escape sequences are removed before writing.
func stripANSI(w io.Writer) io.Writer {
	return &ansiStripper{w: w}
}

// ── Convenience: tee a log message to disk ──────────────────────────

func diskInfo(msg string, args ...any) {
	if diskLog != nil {
		diskLog.Info(msg, args...)
	}
}

func diskWarn(msg string, args ...any) {
	if diskLog != nil {
		diskLog.Warn(msg, args...)
	}
}

func diskError(msg string, args ...any) {
	if diskLog != nil {
		diskLog.Error(msg, args...)
	}
}
