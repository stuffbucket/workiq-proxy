package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"strings"
	"unicode"

	"golang.org/x/term"
)

//go:embed banner.txt
var banner string

//go:embed banner-short.txt
var bannerShort string

//go:embed banner-shorter.txt
var bannerShorter string

const bannerWidth = 125       // visible columns in banner.txt
const bannerShortWidth = 95   // visible columns in banner-short.txt
const bannerShorterWidth = 32 // visible columns in banner-shorter.txt

// Flag variables exist so flag.Parse() can capture CLI values.
// Runtime code reads from cfg (populated by loadConfig).
var (
	_ = flag.String("workiq-cmd", DefaultWorkiqCmd, "Base command to reach the Work IQ CLI")
	_ = flag.String("log-file", "", "Override log file path (default: $XDG_STATE_HOME/workiq-proxy/workiq-proxy.log)")
	_ = flag.Bool("no-log", false, "Disable traffic logging")
	_ = flag.Int("port", DefaultPort, "Port for serve mode (seeks next available if busy)")
)

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func main() {
	flag.Usage = func() {
		printUsage()
		os.Exit(0)
	}
	flag.Parse()
	loadConfig()

	tty := isTerminal()

	cmdParts := splitFields(cfg.WorkiqCmd)
	if len(cmdParts) == 0 {
		uerr("The --workiq-cmd flag cannot be empty.").
			because("workiq-proxy needs a command to reach the Work IQ CLI.").
			fix("Use the default or provide the path: --workiq-cmd /path/to/workiq").
			Die()
	}

	trailing := flag.Args()

	// Explicit "mcp" subcommand → MCP proxy mode regardless of TTY.
	if len(trailing) > 0 && trailing[0] == "mcp" {
		cmdParts = append(cmdParts, "mcp")
		runProxy(cmdParts, false)
		return
	}

	// accept-eula: drive the MCP accept_eula tool call from the CLI.
	if len(trailing) > 0 && trailing[0] == "accept-eula" {
		cmdParts = append(cmdParts, "mcp")
		runAcceptEula(cmdParts)
		return
	}

	// serve: OpenAI-compatible HTTP API.
	if len(trailing) > 0 && trailing[0] == "serve" {
		cmdParts = append(cmdParts, "mcp")
		runServe(cmdParts, cfg.Port)
		return
	}

	// web: serve + browser-based xterm UI.
	if len(trailing) > 0 && trailing[0] == "web" {
		cmdParts = append(cmdParts, "mcp")
		runWeb(cmdParts, cfg.Port)
		return
	}

	// install: set up integrations.
	if len(trailing) > 0 && trailing[0] == "install" {
		if len(trailing) < 2 {
			uerr("Missing install target.").
				because("The install command needs to know which tool to register with.").
				fix("Usage: workiq-proxy install <claude|copilot>").
				Die()
		}
		switch trailing[1] {
		case "claude", "claude-code":
			runInstallClaudeCode()
		case "copilot", "copilot-cli":
			runInstallCopilot()
		default:
			uerr("Unknown install target: " + trailing[1]).
				because("workiq-proxy only supports specific install targets.").
				fix("Available targets: claude, copilot").
				Die()
		}
		return
	}

	// version: show component versions.
	if len(trailing) > 0 && trailing[0] == "version" {
		printVersions()
		return
	}

	// licenses: print project license and third-party disclosures.
	if len(trailing) > 0 && trailing[0] == "licenses" {
		printLicenses()
		return
	}

	// help: show usage and exit.
	if len(trailing) > 0 && trailing[0] == "help" {
		printHelp()
		return
	}

	if tty {
		switch {
		// CLI mode: TTY + trailing args → passthrough to workiq CLI.
		case len(trailing) > 0 && trailing[0] != "json":
			cmdParts = append(cmdParts, trailing...)
			runPassthrough(cmdParts)
			return

		// JSON mode: TTY + "json" arg → MCP proxy with interactive JSON-RPC.
		case len(trailing) > 0 && trailing[0] == "json":
			cmdParts = append(cmdParts, "mcp")
			fmt.Fprintln(os.Stderr, "workiq-proxy JSON-RPC mode — paste JSON-RPC messages, one per line")
			fmt.Fprintln(os.Stderr, "Press Ctrl-D to exit.")
			fmt.Fprintln(os.Stderr, "")
			runProxy(cmdParts, true)
			return

		// REPL: TTY + no args → interactive REPL.
		default:
			cmdParts = append(cmdParts, "mcp")
			runRepl(cmdParts)
			return
		}
	}

	// No TTY, no subcommand → MCP proxy mode (implicit).
	cmdParts = append(cmdParts, "mcp")
	runProxy(cmdParts, false)
}

func printBanner() {
	w, _, err := term.GetSize(int(os.Stderr.Fd()))
	switch {
	case err != nil || w >= bannerWidth:
		fmt.Fprint(os.Stderr, banner)
	case w >= bannerShortWidth:
		fmt.Fprint(os.Stderr, bannerShort)
	case w >= bannerShorterWidth:
		fmt.Fprint(os.Stderr, bannerShorter)
	default:
		fmt.Fprintln(os.Stderr, "workiq-proxy")
	}
}

// splitFields splits s on whitespace like strings.Fields but respects
// double-quoted segments. This handles Windows paths with spaces
// (e.g. `"C:\Program Files\nodejs\node.exe" "C:\path\to\workiq.js"`).
func splitFields(s string) []string {
	var parts []string
	var buf strings.Builder
	inQuote := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
		case !inQuote && unicode.IsSpace(r):
			if buf.Len() > 0 {
				parts = append(parts, buf.String())
				buf.Reset()
			}
		default:
			buf.WriteRune(r)
		}
	}
	if buf.Len() > 0 {
		parts = append(parts, buf.String())
	}
	return parts
}
