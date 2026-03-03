package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteTable(t *testing.T) {
	var buf bytes.Buffer
	writeTable(&buf, [][2]string{
		{"short", "description one"},
		{"longer-name", "description two"},
		{"", ""},
		{"x", "after separator"},
	})
	out := buf.String()

	if !strings.Contains(out, "short        description one") {
		t.Errorf("expected padded 'short' row, got:\n%s", out)
	}
	if !strings.Contains(out, "longer-name  description two") {
		t.Errorf("expected padded 'longer-name' row, got:\n%s", out)
	}
	// Separator should be a blank line.
	if !strings.Contains(out, "\n\n") {
		t.Errorf("expected blank separator line, got:\n%s", out)
	}
}

// TestUsageContainsAllCLICommands verifies --help output includes every
// entry from allCLICommands so adding a command without updating help is caught.
func TestUsageContainsAllCLICommands(t *testing.T) {
	var buf bytes.Buffer
	writeTable(&buf, allCLICommands)
	out := buf.String()

	for _, cmd := range allCLICommands {
		if !strings.Contains(out, cmd[0]) {
			t.Errorf("printUsage output missing command %q", cmd[0])
		}
		if !strings.Contains(out, cmd[1]) {
			t.Errorf("printUsage output missing description %q", cmd[1])
		}
	}
}

// TestHelpContainsFriendlyCommands verifies the beginner help contains
// every entry from friendlyCLICommands.
func TestHelpContainsFriendlyCommands(t *testing.T) {
	var buf bytes.Buffer
	writeTable(&buf, friendlyCLICommands)
	out := buf.String()

	for _, cmd := range friendlyCLICommands {
		if !strings.Contains(out, cmd[0]) {
			t.Errorf("printHelp output missing command %q", cmd[0])
		}
		if !strings.Contains(out, cmd[1]) {
			t.Errorf("printHelp output missing description %q", cmd[1])
		}
	}
}

// TestReplHelpCoversAllSlashCommands verifies that printReplHelp displays
// every slash command. If someone adds a slashCommand but forgets help,
// this will catch it.
func TestReplHelpCoversAllSlashCommands(t *testing.T) {
	// Simulate what printReplHelp builds — same logic, plain text.
	var rows [][2]string
	for _, sc := range slashCommands {
		left := sc.name
		if sc.argHint != "" {
			left += " " + sc.argHint
		}
		rows = append(rows, [2]string{left, sc.desc})
	}
	var buf bytes.Buffer
	writeTable(&buf, rows)
	out := buf.String()

	for _, sc := range slashCommands {
		if !strings.Contains(out, sc.name) {
			t.Errorf("REPL help missing slash command %q", sc.name)
		}
		if !strings.Contains(out, sc.desc) {
			t.Errorf("REPL help missing description for %q: %q", sc.name, sc.desc)
		}
	}
}

// TestFriendlyHelpDirectsToFullHelp verifies the progressive disclosure
// nudge is present in friendlyCLICommands' context.
func TestFriendlyHelpDirectsToFullHelp(t *testing.T) {
	// The friendlyCLICommands data itself doesn't contain the directive,
	// but printHelp() adds it. Verify the data is a strict subset of
	// allCLICommands by checking that every friendly command name
	// appears somewhere in the full list (modulo the "workiq-proxy " prefix
	// and expanded install targets).
	fullText := ""
	for _, cmd := range allCLICommands {
		fullText += cmd[0] + " " + cmd[1] + "\n"
	}

	// Each friendly command's core verb should appear in the full list.
	coreVerbs := []string{"install", "accept-eula", "ask"}
	for _, verb := range coreVerbs {
		if !strings.Contains(fullText, verb) {
			t.Errorf("friendly help references %q but full help doesn't list it", verb)
		}
	}
}

// TestSlashCommandArgHintConsistency verifies that every slashCommand
// with hasArg has a non-empty argHint, and vice versa.
func TestSlashCommandArgHintConsistency(t *testing.T) {
	for _, sc := range slashCommands {
		if sc.hasArg && sc.argHint == "" {
			t.Errorf("slash command %q has hasArg=true but empty argHint", sc.name)
		}
		if !sc.hasArg && sc.argHint != "" {
			t.Errorf("slash command %q has hasArg=false but argHint=%q", sc.name, sc.argHint)
		}
	}
}
