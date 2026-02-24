package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var (
	workiqCmd = flag.String("workiq-cmd", "npx -y @microsoft/workiq", "Base command to reach the Work IQ CLI")
	logFile   = flag.String("log-file", "", "Override log file path (default: ~/.work-iq-cli/mcp-traffic.log)")
	noLog     = flag.Bool("no-log", false, "Disable traffic logging")
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

	if tty {
		switch {
		// CLI mode: TTY + trailing args → passthrough to workiq CLI.
		case len(trailing) > 0 && trailing[0] != "repl":
			cmdParts = append(cmdParts, trailing...)
			runPassthrough(cmdParts)
			return

		// REPL mode: TTY + "repl" arg → MCP proxy with interactive wrapper.
		case len(trailing) > 0 && trailing[0] == "repl":
			cmdParts = append(cmdParts, "mcp")
			fmt.Fprintln(os.Stderr, "workiq-proxy REPL — paste JSON-RPC messages, one per line")
			fmt.Fprintln(os.Stderr, "Press Ctrl-D to exit.")
			fmt.Fprintln(os.Stderr, "")
			runProxy(cmdParts, true)
			return

		// Usage: TTY + no args → show help and exit.
		default:
			fmt.Fprintln(os.Stderr, "Usage: workiq-proxy <command> [args...]")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Commands:")
			fmt.Fprintln(os.Stderr, "  mcp            Start MCP proxy server (stdio)")
			fmt.Fprintln(os.Stderr, "  accept-eula    Accept the Work IQ EULA")
			fmt.Fprintln(os.Stderr, "  ask -q \"...\"   Ask Microsoft 365 Copilot a question")
			fmt.Fprintln(os.Stderr, "  repl           Interactive MCP JSON-RPC testing mode")
			fmt.Fprintln(os.Stderr, "  version        Show workiq CLI version")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "MCP clients launch this command with piped stdio (no TTY).")
			os.Exit(0)
		}
	}

	// No TTY, no subcommand → MCP proxy mode (implicit).
	cmdParts = append(cmdParts, "mcp")
	runProxy(cmdParts, false)
}
