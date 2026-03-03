package main

import (
	"fmt"
	"os"
	"strings"
)

// userError holds a structured error message following the
// what / so-what / what-next pattern.
//
//	What:     What happened (plain statement of fact).
//	Why:      Why it matters — what the user can't do now.
//	Fix:      What to do next to recover.
//	Detail:   Optional technical detail (e.g. wrapped error).
//
// Construct with uerr(), then deliver via .Print(), .Die(), or .ANSI().
type userError struct {
	What   string
	Why    string
	Fix    string
	Detail string
}

// uerr creates a userError. All fields are optional except What.
func uerr(what string) userError {
	return userError{What: what}
}

func (e userError) because(why string) userError { e.Why = why; return e }
func (e userError) fix(fix string) userError     { e.Fix = fix; return e }
func (e userError) detail(err error) userError   { e.Detail = err.Error(); return e }
func (e userError) detailf(f string, a ...any) userError {
	e.Detail = fmt.Sprintf(f, a...)
	return e
}

// ── Delivery methods ────────────────────────────────────────────────

// Print writes the formatted message to stderr, prefixed with "workiq-proxy:".
func (e userError) Print() {
	fmt.Fprintf(os.Stderr, "workiq-proxy: %s\n", e.What)
	if e.Why != "" {
		fmt.Fprintf(os.Stderr, "  %s\n", e.Why)
	}
	if e.Detail != "" {
		fmt.Fprintf(os.Stderr, "  Detail: %s\n", e.Detail)
	}
	if e.Fix != "" {
		fmt.Fprintf(os.Stderr, "  %s\n", e.Fix)
	}
}

// Die prints the message to stderr and exits with code 1.
func (e userError) Die() {
	e.Print()
	os.Exit(1)
}

// ANSI returns the message formatted with ANSI escape codes for
// display in a terminal emulator (e.g. xterm.js via WebSocket).
// Uses \r\n line endings for raw terminal output.
func (e userError) ANSI() string {
	var b strings.Builder
	b.WriteString("\x1b[1;31m") // bold red
	b.WriteString(e.What)
	b.WriteString("\x1b[0m\r\n")
	if e.Why != "" {
		b.WriteString("  ")
		b.WriteString(e.Why)
		b.WriteString("\r\n")
	}
	if e.Detail != "" {
		b.WriteString("  Detail: ")
		b.WriteString(e.Detail)
		b.WriteString("\r\n")
	}
	if e.Fix != "" {
		b.WriteString("  ")
		b.WriteString(e.Fix)
		b.WriteString("\r\n")
	}
	return b.String()
}

// ── Pre-built errors ────────────────────────────────────────────────
// Reusable error constructors for common failure modes. Call .detail()
// to attach the underlying error, then .Die() or .Print() to deliver.

func errCLINotFound(name string) userError {
	return uerr(fmt.Sprintf("Could not find the Work IQ CLI (%s).", name)).
		because("Without it, workiq-proxy has nothing to connect to.").
		fix("Install it with: npm install -g @microsoft/workiq")
}

func errBackendStart() userError {
	return uerr("Could not start the backend process.").
		because("Without the backend, workiq-proxy cannot answer queries.").
		fix("Make sure the Work IQ CLI is installed: npm install -g @microsoft/workiq")
}

func errBackendNoResponse() userError {
	return uerr("The backend did not respond during startup.").
		because("This usually means the Work IQ CLI is installed but not working.").
		fix("Try reinstalling: npm install -g @microsoft/workiq")
}

func errBackendDied(err error) userError {
	e := uerr("The backend process stopped unexpectedly.").
		because("Without it, workiq-proxy can no longer process requests.").
		fix("Restart the server to reconnect.")
	if err != nil {
		e = e.detail(err)
	}
	return e
}

func errPortUnavailable() userError {
	return uerr("Could not find an available port.").
		because("The server needs a port to listen on, and the default range is occupied.").
		fix("Close other programs or try a different port: workiq-proxy serve --port 12000")
}

func errServerStopped() userError {
	return uerr("The server stopped unexpectedly.").
		because("Any connected clients have been disconnected.").
		fix("Check the error above and restart: workiq-proxy serve")
}

// errorCatalog lists all pre-built errors for doc generation.
// Each entry calls the constructor with a placeholder argument so the
// catalog stays in sync with the actual error text.
var errorCatalog = []userError{
	errCLINotFound("{name}"),
	errBackendStart(),
	errBackendNoResponse(),
	errBackendDied(nil),
	errPortUnavailable(),
	errServerStopped(),
}
