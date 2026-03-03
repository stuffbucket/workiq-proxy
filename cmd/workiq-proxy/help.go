package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// allCLICommands is the single source of truth for all CLI subcommands.
// printUsage (--help) shows all of these; printHelp (help) shows a subset.
var allCLICommands = [][2]string{
	{"install", "Register with an MCP client (claude, copilot)"},
	{"accept-eula", "Accept the Work IQ EULA"},
	{`ask -q "..."`, "Ask Microsoft 365 Copilot a question"},
	{"mcp", "Start MCP proxy server (stdio)"},
	{"serve", "Start OpenAI-compatible HTTP API (localhost)"},
	{"web", "Open browser-based terminal UI"},
	{"json", "Interactive JSON-RPC testing mode"},
	{"version", "Show component versions"},
	{"licenses", "Print license and third-party disclosures"},
	{"help", "Show help"},
}

// friendlyCLICommands is shown by the beginner-friendly "help" command.
var friendlyCLICommands = [][2]string{
	{"workiq-proxy", "Start an interactive chat session"},
	{"workiq-proxy install claude", "Register with Claude Code"},
	{"workiq-proxy install copilot", "Register with GitHub Copilot"},
	{"workiq-proxy accept-eula", "Accept the Work IQ EULA"},
	{`workiq-proxy ask -q "..."`, "Ask a question directly"},
}

// writeTable writes padded two-column rows to w.
// A row with both fields empty produces a blank separator line.
func writeTable(w io.Writer, rows [][2]string) {
	width := 0
	for _, r := range rows {
		if n := len(r[0]); n > width {
			width = n
		}
	}
	for _, r := range rows {
		if r[0] == "" && r[1] == "" {
			fmt.Fprintln(w)
			continue
		}
		fmt.Fprintf(w, "  %-*s  %s\n", width, r[0], r[1])
	}
}

func printUsage() {
	printBanner()
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Usage: workiq-proxy [command] [args...]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	writeTable(os.Stderr, allCLICommands)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Options:")
	flag.CommandLine.SetOutput(os.Stderr)
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Run with no arguments in a terminal for interactive REPL.")
	fmt.Fprintln(os.Stderr, "MCP clients launch this command with piped stdio (no TTY).")
}

func printHelp() {
	printBanner()
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "workiq-proxy — Work IQ for your terminal")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  Just run workiq-proxy to start chatting with Microsoft 365 Copilot.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Getting started:")
	writeTable(os.Stderr, friendlyCLICommands)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Run workiq-proxy --help for all commands and options.")
}
