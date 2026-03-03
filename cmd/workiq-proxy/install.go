package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ── Shared helpers ──────────────────────────────────────────────────

var stdinScanner *bufio.Scanner

func readLine() string {
	if stdinScanner == nil {
		stdinScanner = bufio.NewScanner(os.Stdin)
	}
	if stdinScanner.Scan() {
		return strings.TrimSpace(stdinScanner.Text())
	}
	return ""
}

func confirmYes() bool {
	answer := strings.ToLower(readLine())
	return answer == "y" || answer == "yes"
}

func selfExePath() string {
	p, err := os.Executable()
	if err != nil {
		uerr("Could not determine where workiq-proxy is installed.").
			because("The install command needs to register the full binary path.").
			fix("Run workiq-proxy from an absolute path, or check file permissions.").
			detail(err).Die()
	}
	// Resolve symlinks so the registered path survives link changes.
	resolved, err := filepath.EvalSymlinks(p)
	if err == nil {
		p = resolved
	}
	return p
}

// ── Claude Code ─────────────────────────────────────────────────────

func runInstallClaudeCode() {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		errCLINotFound("claude").Die()
	}

	selfPath := selfExePath()

	// Check for existing "workiq" MCP server.
	existing := claudeMCPGet(claudePath, "workiq")
	if existing != "" {
		fmt.Fprintln(os.Stderr, "Found an existing workiq MCP server in Claude Code:")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, existing)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprint(os.Stderr, "Replace it with workiq-proxy? [y/N] ")
		if !confirmYes() {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return
		}
		removeCmd := exec.Command(claudePath, "mcp", "remove", "workiq")
		removeCmd.Stderr = os.Stderr
		if err := removeCmd.Run(); err != nil {
			uerr("Could not remove existing MCP server registration.").
				because("The old entry must be removed before adding the new one.").
				fix("Try: claude mcp remove workiq").
				detail(err).Die()
		}
	}

	scope := promptClaudeScope()

	addCmd := exec.Command(claudePath, "mcp", "add",
		"-s", scope, "workiq", "--", selfPath, "mcp",
	)
	addCmd.Stdout = os.Stdout
	addCmd.Stderr = os.Stderr
	if err := addCmd.Run(); err != nil {
		uerr("Claude MCP registration failed.").
			because("workiq-proxy was not registered as an MCP server.").
			fix("Try: claude mcp add workiq -- workiq-proxy mcp").
			detail(err).Die()
	}

	// Verify the registration actually took effect.
	verify := claudeMCPGet(claudePath, "workiq")
	if verify == "" {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "⚠ Registration command succeeded but verification failed.")
		fmt.Fprintln(os.Stderr, "  Run 'claude mcp list' to check.")
		os.Exit(1)
	}
	if !strings.Contains(verify, selfPath) {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "⚠ Registered, but command path doesn't match this binary:")
		fmt.Fprintln(os.Stderr, verify)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintf(os.Stderr, "✓ workiq-proxy registered in Claude Code (scope: %s)\n", scope)
}

func claudeMCPGet(claudePath, name string) string {
	cmd := exec.Command(claudePath, "mcp", "get", name)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func promptClaudeScope() string {
	fmt.Fprintln(os.Stderr, "Choose scope:")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  1. user      Available in all projects (recommended)")
	fmt.Fprintln(os.Stderr, "  2. local     This machine only, current project")
	fmt.Fprintln(os.Stderr, "  3. project   Shared via .mcp.json in the repo")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprint(os.Stderr, "Scope [1]: ")
	switch readLine() {
	case "2", "local":
		return "local"
	case "3", "project":
		return "project"
	default:
		return "user"
	}
}

// ── Copilot CLI ─────────────────────────────────────────────────────

// mcpConfig is the structure of ~/.copilot/mcp-config.json.
type mcpConfig struct {
	Servers map[string]mcpServerEntry `json:"mcpServers"`
}

type mcpServerEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func runInstallCopilot() {
	// Find copilot binary.
	copilotPath := findCopilot()
	if copilotPath == "" {
		errCLINotFound("copilot").Die()
	}
	_ = copilotPath // used only for discovery

	selfPath := selfExePath()

	// Copilot CLI always uses ~/.copilot/ as its config directory.
	// The --additional-mcp-config help explicitly references ~/.copilot/mcp-config.json.
	home, err := os.UserHomeDir()
	if err != nil {
		uerr("Cannot determine home directory.").
			because("The Copilot config file path cannot be resolved.").
			fix("Set the HOME environment variable.").
			detail(err).Die()
	}
	configDir := filepath.Join(home, ".copilot")
	configFile := filepath.Join(configDir, "mcp-config.json")

	// Probe: the ~/.copilot/ directory should already exist if Copilot has been used.
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "workiq-proxy: Copilot config directory %s does not exist.\n", configDir)
		fmt.Fprintln(os.Stderr, "  Has Copilot CLI been run at least once?")
		fmt.Fprint(os.Stderr, "  Create it and continue? [y/N] ")
		if !confirmYes() {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return
		}
	}

	// Read existing config if the file exists.
	cfg, fileExisted := readMCPConfig(configFile)

	// Check for existing entry.
	if entry, ok := cfg.Servers["workiq"]; ok {
		fmt.Fprintln(os.Stderr, "Found an existing workiq MCP server in Copilot CLI config:")
		fmt.Fprintf(os.Stderr, "  Command: %s\n", entry.Command)
		if len(entry.Args) > 0 {
			fmt.Fprintf(os.Stderr, "  Args:    %s\n", strings.Join(entry.Args, " "))
		}
		fmt.Fprintf(os.Stderr, "  File:    %s\n", configFile)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprint(os.Stderr, "Replace it with workiq-proxy? [y/N] ")
		if !confirmYes() {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return
		}
	}

	// Set the new entry.
	cfg.Servers["workiq"] = mcpServerEntry{
		Command: selfPath,
		Args:    []string{"mcp"},
	}

	// Back up the existing file before writing.
	backupFile := ""
	if fileExisted {
		backupFile = configFile + ".bak"
		if err := copyFile(configFile, backupFile); err != nil {
			uerr("Could not back up the existing config file.").
				because("Without a backup, we won't risk overwriting your config.").
				fix("Check permissions on " + filepath.Dir(configFile)).
				detail(err).Die()
		}
		fmt.Fprintf(os.Stderr, "Backed up existing config to %s\n", backupFile)
	}

	// Write the config.
	if err := writeMCPConfig(configFile, cfg); err != nil {
		if backupFile != "" {
			restoreBackup(backupFile, configFile)
		}
		uerr("Failed to write Copilot MCP config.").
			because("workiq-proxy was not registered.").
			fix("Check permissions on " + configFile).
			detail(err).Die()
	}

	// Verify: re-read the file and check our entry is present.
	verifyCfg, _ := readMCPConfig(configFile)
	entry, ok := verifyCfg.Servers["workiq"]
	if !ok || entry.Command != selfPath {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "⚠ Wrote config but verification failed — the entry doesn't match.")
		if backupFile != "" {
			restoreBackup(backupFile, configFile)
		}
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintf(os.Stderr, "✓ workiq-proxy registered in Copilot CLI (%s)\n", configFile)
}

// findCopilot locates the copilot binary, checking several common names.
func findCopilot() string {
	for _, name := range []string{"copilot", "gh"} {
		p, err := exec.LookPath(name)
		if err == nil {
			return p
		}
	}
	return ""
}

// readMCPConfig reads and parses the MCP config file.
// Returns an empty config with initialized map if the file doesn't exist.
// The bool indicates whether the file existed on disk.
func readMCPConfig(path string) (mcpConfig, bool) {
	cfg := mcpConfig{Servers: make(map[string]mcpServerEntry)}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, false
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		uerr("Could not parse " + path).
			because("The file exists but is not valid JSON — we will not overwrite it.").
			fix("Fix the file manually or delete it, then try again.").
			detail(err).Die()
	}
	if cfg.Servers == nil {
		cfg.Servers = make(map[string]mcpServerEntry)
	}
	return cfg, true
}

// writeMCPConfig writes the config as indented JSON. Creates parent dirs
// if needed.
func writeMCPConfig(path string, cfg mcpConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// copyFile copies src to dst, preserving permissions.
func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// restoreBackup moves the backup file back to the original path.
func restoreBackup(backupFile, originalFile string) {
	if err := os.Rename(backupFile, originalFile); err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠ Could not restore backup: %v\n", err)
		fmt.Fprintf(os.Stderr, "  Your backup is at: %s\n", backupFile)
	} else {
		fmt.Fprintf(os.Stderr, "  Restored original config from %s\n", backupFile)
	}
}
