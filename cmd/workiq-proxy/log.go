package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func stateDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".work-iq-cli")
}

func resolveLogFile() string {
	if *logFile != "" {
		return *logFile
	}
	return filepath.Join(stateDir(), "mcp-traffic.log")
}

func logMsg(direction string, data []byte) {
	if *noLog {
		return
	}
	p := resolveLogFile()
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s %s\n", time.Now().UTC().Format(time.RFC3339Nano), direction, string(data))
}
