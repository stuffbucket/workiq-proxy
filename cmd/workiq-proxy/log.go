package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func stateDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".work-iq-cli")
}

func resolveLogFile() string {
	if *logFile != "" {
		return *logFile
	}
	dir := stateDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "mcp-traffic.log")
}

func logMsg(direction string, data []byte) {
	if *noLog {
		return
	}
	p := resolveLogFile()
	if p == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s %s\n", time.Now().UTC().Format(time.RFC3339Nano), direction, string(data))
}
