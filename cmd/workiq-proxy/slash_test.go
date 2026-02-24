package main

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// sendKey simulates a key press and returns the updated model.
func sendKey(m promptModel, k tea.KeyType) promptModel {
	result, _ := m.Update(tea.KeyMsg{Type: k})
	return result.(promptModel)
}

// sendRunes simulates typing a string one rune at a time.
func sendRunes(m promptModel, s string) promptModel {
	for _, r := range s {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = result.(promptModel)
	}
	return m
}

func TestSlashMenuAppearsOnSlash(t *testing.T) {
	history := []string{}
	m := newPromptModel(&history)
	m = sendRunes(m, "/")

	if !m.showMenu {
		t.Fatal("expected menu to show after typing /")
	}
	if len(m.matches) != len(slashCommands) {
		t.Fatalf("expected %d matches, got %d", len(slashCommands), len(m.matches))
	}
}

func TestSlashMenuFilters(t *testing.T) {
	history := []string{}
	m := newPromptModel(&history)
	m = sendRunes(m, "/a")

	if !m.showMenu {
		t.Fatal("expected menu after /a")
	}
	// Should match /ask and /accept-eula
	if len(m.matches) != 2 {
		t.Fatalf("expected 2 matches for /a, got %d", len(m.matches))
	}

	m = sendRunes(m, "s")
	// Now "/as" → only /ask
	if len(m.matches) != 1 {
		t.Fatalf("expected 1 match for /as, got %d", len(m.matches))
	}
	if m.matches[0].name != "/ask" {
		t.Errorf("expected /ask, got %s", m.matches[0].name)
	}
}

func TestSlashMenuNoMatchHidesMenu(t *testing.T) {
	history := []string{}
	m := newPromptModel(&history)
	m = sendRunes(m, "/xyz")

	if m.showMenu {
		t.Fatal("expected menu hidden for /xyz (no matches)")
	}
}

func TestMenuNavigationUpDown(t *testing.T) {
	history := []string{}
	m := newPromptModel(&history)
	m = sendRunes(m, "/")

	if m.cursor != 0 {
		t.Fatalf("initial cursor should be 0, got %d", m.cursor)
	}

	m = sendKey(m, tea.KeyDown)
	if m.cursor != 1 {
		t.Errorf("down: cursor should be 1, got %d", m.cursor)
	}

	m = sendKey(m, tea.KeyUp)
	if m.cursor != 0 {
		t.Errorf("up: cursor should be 0, got %d", m.cursor)
	}

	// Wrap around up.
	m = sendKey(m, tea.KeyUp)
	if m.cursor != len(m.matches)-1 {
		t.Errorf("wrap up: cursor should be %d, got %d", len(m.matches)-1, m.cursor)
	}

	// Wrap around down.
	m = sendKey(m, tea.KeyDown)
	if m.cursor != 0 {
		t.Errorf("wrap down: cursor should be 0, got %d", m.cursor)
	}
}

func TestTabAcceptsCompletion(t *testing.T) {
	history := []string{}
	m := newPromptModel(&history)
	m = sendRunes(m, "/to")
	// Should have /tools selected.
	if len(m.matches) != 1 || m.matches[0].name != "/tools" {
		t.Fatalf("expected /tools match")
	}

	m = sendKey(m, tea.KeyTab)
	// /tools has no arg, so value should be filled and menu hidden.
	if m.input.Value() != "/tools" {
		t.Errorf("expected input /tools, got %q", m.input.Value())
	}
	if m.showMenu {
		t.Error("menu should be hidden after tab accept")
	}
}

func TestTabAcceptsCommandWithArg(t *testing.T) {
	history := []string{}
	m := newPromptModel(&history)
	m = sendRunes(m, "/as")
	// Should have /ask selected.
	if len(m.matches) != 1 || m.matches[0].name != "/ask" {
		t.Fatalf("expected /ask match")
	}

	m = sendKey(m, tea.KeyTab)
	// /ask has arg, so should append space and keep editing.
	if m.input.Value() != "/ask " {
		t.Errorf("expected input '/ask ', got %q", m.input.Value())
	}
	if m.done {
		t.Error("should not be done — waiting for arg")
	}
}

func TestEnterSubmitsMenuSelection(t *testing.T) {
	history := []string{}
	m := newPromptModel(&history)
	m = sendRunes(m, "/to")

	m = sendKey(m, tea.KeyEnter)
	if !m.done {
		t.Fatal("expected done after enter")
	}
	if m.value != "/tools" {
		t.Errorf("expected value /tools, got %q", m.value)
	}
}

func TestEnterSubmitsBareText(t *testing.T) {
	history := []string{}
	m := newPromptModel(&history)
	m = sendRunes(m, "what is the weather?")

	m = sendKey(m, tea.KeyEnter)
	if !m.done {
		t.Fatal("expected done")
	}
	if m.value != "what is the weather?" {
		t.Errorf("expected bare text, got %q", m.value)
	}
}

func TestEscapeDismissesMenu(t *testing.T) {
	history := []string{}
	m := newPromptModel(&history)
	m = sendRunes(m, "/")
	if !m.showMenu {
		t.Fatal("menu should be visible")
	}

	m = sendKey(m, tea.KeyEscape)
	if m.showMenu {
		t.Error("escape should dismiss menu")
	}
}

func TestCtrlDQuits(t *testing.T) {
	history := []string{}
	m := newPromptModel(&history)
	m = sendKey(m, tea.KeyCtrlD)
	if !m.quit {
		t.Error("Ctrl-D should set quit")
	}
	if !m.done {
		t.Error("Ctrl-D should set done")
	}
}

func TestCtrlCClearsInputThenCancels(t *testing.T) {
	history := []string{}
	m := newPromptModel(&history)
	m = sendRunes(m, "hello")

	// First Ctrl-C with text → clears input.
	m = sendKey(m, tea.KeyCtrlC)
	if m.done {
		t.Error("first Ctrl-C with text should not be done")
	}
	if m.input.Value() != "" {
		t.Errorf("first Ctrl-C should clear input, got %q", m.input.Value())
	}

	// First Ctrl-C on empty → cancels prompt (but not quit).
	// Reset the package-level lastCtrlC so this isn't a "double".
	lastCtrlC = time.Time{}
	m = sendKey(m, tea.KeyCtrlC)
	if !m.done {
		t.Error("Ctrl-C on empty should cancel prompt")
	}
	if m.quit {
		t.Error("single Ctrl-C should not quit")
	}
}

func TestDoubleCtrlCQuits(t *testing.T) {
	history := []string{}
	m := newPromptModel(&history)

	// Simulate two rapid Ctrl-C presses.
	m = sendKey(m, tea.KeyCtrlC) // first: cancel
	// lastCtrlC is now set at package level. Immediately send another.
	m.done = false // undo the done from first press for testing
	m = sendKey(m, tea.KeyCtrlC)

	if !m.quit {
		t.Error("double Ctrl-C should quit")
	}
}

// ── History tests ───────────────────────────────────────────────────

func TestHistoryUpDown(t *testing.T) {
	history := []string{"first", "second", "third"}
	m := newPromptModel(&history)

	// histIdx starts at len(history) == 3 (new-line position).
	if m.histIdx != 3 {
		t.Fatalf("initial histIdx should be 3, got %d", m.histIdx)
	}

	// Up → "third"
	m = sendKey(m, tea.KeyUp)
	if m.input.Value() != "third" {
		t.Errorf("up once: expected 'third', got %q", m.input.Value())
	}

	// Up → "second"
	m = sendKey(m, tea.KeyUp)
	if m.input.Value() != "second" {
		t.Errorf("up twice: expected 'second', got %q", m.input.Value())
	}

	// Up → "first"
	m = sendKey(m, tea.KeyUp)
	if m.input.Value() != "first" {
		t.Errorf("up thrice: expected 'first', got %q", m.input.Value())
	}

	// Up at top → stays at "first"
	m = sendKey(m, tea.KeyUp)
	if m.input.Value() != "first" {
		t.Errorf("up at top: should stay 'first', got %q", m.input.Value())
	}

	// Down → "second"
	m = sendKey(m, tea.KeyDown)
	if m.input.Value() != "second" {
		t.Errorf("down from first: expected 'second', got %q", m.input.Value())
	}

	// Down → "third"
	m = sendKey(m, tea.KeyDown)
	if m.input.Value() != "third" {
		t.Errorf("down to third: expected 'third', got %q", m.input.Value())
	}

	// Down → back to empty new-line.
	m = sendKey(m, tea.KeyDown)
	if m.input.Value() != "" {
		t.Errorf("down past end: expected empty, got %q", m.input.Value())
	}

	// Down at bottom → stays empty.
	m = sendKey(m, tea.KeyDown)
	if m.input.Value() != "" {
		t.Errorf("down at bottom: should stay empty, got %q", m.input.Value())
	}
}

func TestHistorySavesInProgressInput(t *testing.T) {
	history := []string{"old entry"}
	m := newPromptModel(&history)

	// Type something.
	m = sendRunes(m, "in progress")

	// Up → shows "old entry", stashes "in progress".
	m = sendKey(m, tea.KeyUp)
	if m.input.Value() != "old entry" {
		t.Fatalf("up: expected 'old entry', got %q", m.input.Value())
	}

	// Down → restores "in progress".
	m = sendKey(m, tea.KeyDown)
	if m.input.Value() != "in progress" {
		t.Errorf("down: expected 'in progress', got %q", m.input.Value())
	}
}

func TestHistoryEmptyDoesNothing(t *testing.T) {
	history := []string{}
	m := newPromptModel(&history)

	// Up with no history → no change.
	m = sendKey(m, tea.KeyUp)
	if m.input.Value() != "" {
		t.Errorf("up with empty history should be no-op, got %q", m.input.Value())
	}
}

func TestHistoryUpDownWithMenuTakesPriority(t *testing.T) {
	history := []string{"old"}
	m := newPromptModel(&history)
	m = sendRunes(m, "/")

	if !m.showMenu {
		t.Fatal("menu should be showing")
	}

	// Up/Down should navigate menu, not history.
	m = sendKey(m, tea.KeyDown)
	if m.cursor != 1 {
		t.Errorf("down in menu: cursor should be 1, got %d", m.cursor)
	}

	// Input should still be "/"
	if m.input.Value() != "/" {
		t.Errorf("input should still be '/', got %q", m.input.Value())
	}
}

func TestFilterSlashCommands(t *testing.T) {
	tests := []struct {
		prefix string
		want   int
	}{
		{"/", len(slashCommands)},
		{"/a", 2},  // /ask, /accept-eula
		{"/as", 1}, // /ask
		{"/q", 1},  // /quit
		{"/t", 1},  // /tools
		{"/h", 1},  // /help
		{"/z", 0},
	}
	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			got := filterSlashCommands(tt.prefix)
			if len(got) != tt.want {
				t.Errorf("filterSlashCommands(%q) = %d matches, want %d", tt.prefix, len(got), tt.want)
			}
		})
	}
}

func TestMenuHidesAfterSpace(t *testing.T) {
	history := []string{}
	m := newPromptModel(&history)
	m = sendRunes(m, "/ask")
	if !m.showMenu {
		t.Fatal("menu should show for '/ask'")
	}

	m = sendRunes(m, " ")
	if m.showMenu {
		t.Error("menu should hide after space (entering arguments)")
	}
}
