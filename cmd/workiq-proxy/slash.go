package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// slashCommand describes a single REPL slash command.
type slashCommand struct {
	name   string
	desc   string
	hasArg bool // true if the command expects a trailing argument
}

var slashCommands = []slashCommand{
	{name: "/ask", desc: "Ask Microsoft 365 Copilot a question", hasArg: true},
	{name: "/emails", desc: "Search emails (from: subject: date:)", hasArg: true},
	{name: "/docs", desc: "Search documents (name: site: type:)", hasArg: true},
	{name: "/chats", desc: "Search chats (with: date:)", hasArg: true},
	{name: "/channels", desc: "Search channels (team: channel: date:)", hasArg: true},
	{name: "/meetings", desc: "Search meetings (by: subject: date:)", hasArg: true},
	{name: "/people", desc: "Search people (name: dept: project:)", hasArg: true},
	{name: "/accept-eula", desc: "Accept the Work IQ EULA"},
	{name: "/tools", desc: "List available MCP tools"},
	{name: "/help", desc: "Show available commands"},
	{name: "/quit", desc: "Exit"},
}

// ── Menu styles ──────────────────────────────────────────────────────

var (
	menuCursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("63"))

	menuActiveCmdStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("228")).
				Bold(true).
				Width(18)

	menuActiveDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	menuDimCmdStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")).
			Width(18)

	menuDimDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("239"))
)

// ── Bubble Tea model ────────────────────────────────────────────────

type promptModel struct {
	input    textinput.Model
	matches  []slashCommand // filtered slash commands
	cursor   int            // selected index inside matches
	showMenu bool
	done     bool
	quit     bool // Ctrl-D or double Ctrl-C
	value    string

	// Session history (shared across prompts via pointer).
	history  *[]string
	histIdx  int    // current position in history; len(history) == "new line"
	savedBuf string // stash the in-progress input when browsing history
}

func newPromptModel(history *[]string) promptModel {
	ti := textinput.New()
	ti.Prompt = promptStyle.Render("wiq") + " " + subtleStyle.Render("›") + " "
	ti.Focus()
	ti.CharLimit = 500
	ti.ShowSuggestions = false // we render our own dropdown
	return promptModel{
		input:   ti,
		history: history,
		histIdx: len(*history),
	}
}

func (m promptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlD:
			m.quit = true
			m.done = true
			return m, tea.Quit

		case tea.KeyCtrlC:
			now := time.Now()
			if now.Sub(lastCtrlC) < 500*time.Millisecond {
				// Double Ctrl-C → quit the REPL entirely.
				m.quit = true
				m.done = true
				return m, tea.Quit
			}
			lastCtrlC = now
			if m.input.Value() != "" {
				// First Ctrl-C with text → clear the input.
				m.input.Reset()
				m.showMenu = false
				m.matches = nil
				return m, nil
			}
			// First Ctrl-C on empty input → cancel this prompt.
			m.value = ""
			m.done = true
			return m, tea.Quit

		case tea.KeyEnter:
			if m.showMenu && len(m.matches) > 0 {
				sel := m.matches[m.cursor]
				if sel.hasArg {
					// Fill in the command, keep editing so user can add args.
					m.input.SetValue(sel.name + " ")
					m.input.CursorEnd()
					m.showMenu = false
					return m, nil
				}
				m.value = sel.name
			} else {
				m.value = m.input.Value()
			}
			m.done = true
			return m, tea.Quit

		case tea.KeyTab:
			if m.showMenu && len(m.matches) > 0 {
				sel := m.matches[m.cursor]
				suffix := ""
				if sel.hasArg {
					suffix = " "
				}
				m.input.SetValue(sel.name + suffix)
				m.input.CursorEnd()
				m.showMenu = false
				return m, nil
			}
			return m, nil // swallow tab

		case tea.KeyUp:
			if m.showMenu && len(m.matches) > 0 {
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(m.matches) - 1
				}
				return m, nil
			}
			// History: move backward.
			if m.history != nil && len(*m.history) > 0 && m.histIdx > 0 {
				if m.histIdx == len(*m.history) {
					m.savedBuf = m.input.Value()
				}
				m.histIdx--
				m.input.SetValue((*m.history)[m.histIdx])
				m.input.CursorEnd()
			}
			return m, nil

		case tea.KeyDown:
			if m.showMenu && len(m.matches) > 0 {
				m.cursor++
				if m.cursor >= len(m.matches) {
					m.cursor = 0
				}
				return m, nil
			}
			// History: move forward.
			if m.history != nil && m.histIdx < len(*m.history) {
				m.histIdx++
				if m.histIdx == len(*m.history) {
					m.input.SetValue(m.savedBuf)
				} else {
					m.input.SetValue((*m.history)[m.histIdx])
				}
				m.input.CursorEnd()
			}
			return m, nil

		case tea.KeyEscape:
			if m.showMenu {
				m.showMenu = false
				return m, nil
			}
		}
	}

	// Forward everything else to the text input.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	// Refresh the slash-command menu whenever the value changes.
	val := m.input.Value()
	if strings.HasPrefix(val, "/") && !strings.Contains(val, " ") {
		m.matches = filterSlashCommands(val)
		m.showMenu = len(m.matches) > 0
		if m.cursor >= len(m.matches) {
			m.cursor = 0
		}
	} else {
		m.showMenu = false
		m.matches = nil
	}

	return m, cmd
}

func (m promptModel) View() string {
	if m.done {
		prompt := promptStyle.Render("wiq") + " " + subtleStyle.Render("›")
		if m.value != "" {
			prompt += " " + m.value
		}
		return prompt + "\n"
	}

	var b strings.Builder
	b.WriteString(m.input.View())

	if m.showMenu && len(m.matches) > 0 {
		b.WriteByte('\n')
		for i, c := range m.matches {
			if i > 0 {
				b.WriteByte('\n')
			}
			if i == m.cursor {
				b.WriteString(fmt.Sprintf("  %s %s %s",
					menuCursorStyle.Render("▸"),
					menuActiveCmdStyle.Render(c.name),
					menuActiveDescStyle.Render(c.desc)))
			} else {
				b.WriteString(fmt.Sprintf("    %s %s",
					menuDimCmdStyle.Render(c.name),
					menuDimDescStyle.Render(c.desc)))
			}
		}
	}

	return b.String()
}

// ── Helpers ─────────────────────────────────────────────────────────

func filterSlashCommands(prefix string) []slashCommand {
	prefix = strings.ToLower(prefix)
	var out []slashCommand
	for _, c := range slashCommands {
		if strings.HasPrefix(c.name, prefix) {
			out = append(out, c)
		}
	}
	return out
}

// readInput runs a Bubble Tea program to collect one line of REPL input
// with slash-command completion and session history.
// Returns the input string and true on success, or ("", false) when the
// user presses Ctrl-D to quit.
func readInput() (string, bool) {
	p := tea.NewProgram(newPromptModel(&replHistory), tea.WithOutput(os.Stderr))
	result, err := p.Run()
	if err != nil {
		return "", false
	}
	final := result.(promptModel)
	if final.quit {
		return "", false
	}
	if final.value != "" {
		replHistory = append(replHistory, final.value)
	}
	return final.value, true
}

// replHistory stores all non-empty inputs for the current REPL session.
var replHistory []string

// lastCtrlC tracks the timestamp of the last Ctrl-C across prompts
// so that two rapid presses (even across prompt boundaries) trigger quit.
var lastCtrlC time.Time
