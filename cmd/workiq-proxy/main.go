package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"strings"
)

//go:embed banner.txt
var banner string

var (
	workiqCmd = flag.String("workiq-cmd", "npx -y @microsoft/workiq", "Base command to reach the Work IQ CLI")
	logFile   = flag.String("log-file", "", "Override log file path (default: ~/.work-iq-cli/mcp-traffic.log)")
	noLog     = flag.Bool("no-log", false, "Disable traffic logging")
	servePort = flag.Int("port", 11435, "Port for serve mode (seeks next available if busy)")
)

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func main() {
	flag.Parse()

	tty := isTerminal()

	cmdParts := strings.Fields(*workiqCmd)
	if len(cmdParts) == 0 {
		fmt.Fprintln(os.Stderr, "workiq-proxy: --workiq-cmd cannot be empty")
		os.Exit(1)
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
		runServe(cmdParts, *servePort)
		return
	}

	// help: show usage and exit.
	if len(trailing) > 0 && trailing[0] == "help" {
		printUsage()
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

func printUsage() {
	fmt.Fprint(os.Stderr, banner)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Usage: workiq-proxy [command] [args...]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  accept-eula    Accept the Work IQ EULA")
	fmt.Fprintln(os.Stderr, "  ask -q \"...\"   Ask Microsoft 365 Copilot a question")
	fmt.Fprintln(os.Stderr, "  mcp            Start MCP proxy server (stdio)")
	fmt.Fprintln(os.Stderr, "  serve          Start OpenAI-compatible HTTP API (localhost)")
	fmt.Fprintln(os.Stderr, "  json           Interactive JSON-RPC testing mode")
	fmt.Fprintln(os.Stderr, "  version        Show workiq CLI version")
	fmt.Fprintln(os.Stderr, "  help           Show this help")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Run with no arguments in a terminal for interactive REPL.")
	fmt.Fprintln(os.Stderr, "MCP clients launch this command with piped stdio (no TTY).")
}
