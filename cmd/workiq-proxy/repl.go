package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

var (
	dimDot    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	brightDot = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
)

// startSpinner launches a goroutine that animates the bubbles Points
// spinner on stderr with per-character coloring. Call the returned
// function to stop it and clear the line.
func startSpinner() func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	// Build styled frames from bubbles spinner.Points and arrange as
	// a bounce sequence: ●∙∙ ∙●∙ ∙∙● ∙●∙ (always one dot highlighted).
	styledFrames := make([]string, 0, len(spinner.Points.Frames))
	for _, f := range spinner.Points.Frames {
		var b strings.Builder
		for _, r := range f {
			if r == '●' {
				b.WriteString(brightDot.Render(string(r)))
			} else {
				b.WriteString(dimDot.Render(string(r)))
			}
		}
		styledFrames = append(styledFrames, b.String())
	}
	// Skip the all-dim frame (index 0), bounce the rest: 1,2,3,2,1,2,3,...
	frames := []string{styledFrames[1], styledFrames[2], styledFrames[3], styledFrames[2]}
	n := len(frames)
	go func() {
		defer close(done)
		i := 0
		fmt.Fprint(os.Stderr, "\033[?25l") // hide cursor
		t := time.NewTicker(250 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stop:
				fmt.Fprint(os.Stderr, "\r\033[K\033[?25h") // clear line, show cursor
				return
			case <-t.C:
				fmt.Fprintf(os.Stderr, "\r  %s", frames[i%n])
				i++
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}

var (
	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("63")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	subtleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	cmdStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("228")).
			Bold(true)

	descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	toolNameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true).
			Width(22)

	toolDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("248"))
)

func newMarkdownRenderer() *glamour.TermRenderer {
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		return nil
	}
	return r
}

func printHelp() {
	cmds := []struct{ cmd, desc string }{
		{"/ask <question>", "Ask Microsoft 365 Copilot a question"},
		{"/emails [opts]", "Search emails (from: subject: date:)"},
		{"/docs [opts]", "Search documents (name: site: type:)"},
		{"/chats [opts]", "Search chats (with: date:)"},
		{"/channels [opts]", "Search channels (team: channel: date:)"},
		{"/meetings [opts]", "Search meetings (by: subject: date:)"},
		{"/people [opts]", "Search people (name: dept: project:)"},
		{"", ""},
		{"/accept-eula", "Accept the Work IQ EULA"},
		{"/tools", "List available MCP tools"},
		{"/help", "Show this help"},
		{"/quit", "Exit"},
	}
	fmt.Fprintln(os.Stderr, "")
	for _, c := range cmds {
		if c.cmd == "" {
			fmt.Fprintln(os.Stderr, "")
			continue
		}
		fmt.Fprintf(os.Stderr, "  %s  %s\n",
			cmdStyle.Render(fmt.Sprintf("%-20s", c.cmd)),
			descStyle.Render(c.desc))
	}
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, subtleStyle.Render("  Type / for commands, or just type a question."))
	fmt.Fprintln(os.Stderr, subtleStyle.Render("  Chain commands with &&: /emails from:bob && /meetings date:tomorrow"))
	fmt.Fprintln(os.Stderr, subtleStyle.Render("  Press Ctrl-C twice to exit."))
	fmt.Fprintln(os.Stderr, "")
}

// runRepl starts the workiq MCP server and provides a human-friendly
// interactive REPL. Commands are translated into JSON-RPC tool calls.
func runRepl(cmdParts []string) {
	cmdPath, err := exec.LookPath(cmdParts[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "workiq-proxy: cannot find %q: %v\n", cmdParts[0], err)
		os.Exit(1)
	}

	child := exec.Command(cmdPath, cmdParts[1:]...)
	child.Stderr = os.Stderr

	childIn, err := child.StdinPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "workiq-proxy: stdin pipe: %v\n", err)
		os.Exit(1)
	}
	childOut, err := child.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "workiq-proxy: stdout pipe: %v\n", err)
		os.Exit(1)
	}

	if err := child.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "workiq-proxy: failed to start %q: %v\n", strings.Join(cmdParts, " "), err)
		os.Exit(1)
	}

	// Response channel — scanner goroutine pushes parsed messages here.
	responses := make(chan rpcMessage, 16)
	var scanWg sync.WaitGroup
	scanWg.Add(1)
	go func() {
		defer scanWg.Done()
		defer close(responses)
		scanner := bufio.NewScanner(childOut)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var msg rpcMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				continue
			}
			responses <- msg
		}
	}()

	nextID := 0
	send := func(method string, params interface{}) {
		nextID++
		id := json.RawMessage(fmt.Sprintf("%d", nextID))
		msg := rpcMessage{
			JSONRPC: "2.0",
			ID:      &id,
			Method:  method,
			Params:  mustMarshal(params),
		}
		data, _ := json.Marshal(msg)
		childIn.Write(data)
		childIn.Write([]byte("\n"))
	}

	sendNotification := func(method string) {
		msg := rpcMessage{JSONRPC: "2.0", Method: method}
		data, _ := json.Marshal(msg)
		childIn.Write(data)
		childIn.Write([]byte("\n"))
	}

	waitResponse := func() (rpcMessage, bool) {
		stopSpinner := startSpinner()
		for {
			msg, ok := <-responses
			if !ok {
				stopSpinner()
				return msg, false
			}
			// Skip server-initiated notifications (no ID).
			if msg.ID == nil && msg.Method != "" {
				continue
			}
			stopSpinner()
			return msg, true
		}
	}

	// MCP handshake.
	send("initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]string{"name": "workiq-proxy", "version": "0.1.0"},
	})
	if _, ok := waitResponse(); !ok {
		fmt.Fprintln(os.Stderr, "workiq-proxy: server did not respond to initialize")
		os.Exit(1)
	}
	sendNotification("notifications/initialized")

	mdRenderer := newMarkdownRenderer()

	fmt.Fprint(os.Stderr, banner)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, subtleStyle.Render("workiq-proxy interactive mode"))
	printHelp()

	askQuestion := func(question string) {
		send("tools/call", toolCallParams{
			Name:      "ask_work_iq",
			Arguments: mustMarshal(map[string]string{"question": question}),
		})
		if resp, ok := waitResponse(); ok {
			printReplResult(resp, mdRenderer)
		}
	}

	quit := false
	dispatch := func(input string) {
		if input == "" {
			return
		}

		// Bare text (no / prefix) → treat as a question.
		if !strings.HasPrefix(input, "/") {
			askQuestion(input)
			return
		}

		cmd, arg := splitCommand(input[1:])

		// Steering commands route through buildQuestion.
		if spec, ok := steeringSpecs[cmd]; ok {
			params := parseSteeringArgs(arg, spec)
			askQuestion(buildQuestion(spec.tool, mustMarshal(params)))
			return
		}

		switch cmd {
		case "ask":
			if arg == "" {
				fmt.Fprintln(os.Stderr, errorStyle.Render("Usage: /ask <question>"))
			} else {
				askQuestion(arg)
			}

		case "accept-eula":
			send("tools/call", toolCallParams{
				Name:      "accept_eula",
				Arguments: mustMarshal(map[string]string{"eulaUrl": "https://github.com/microsoft/work-iq-mcp"}),
			})
			if resp, ok := waitResponse(); ok {
				printReplResult(resp, mdRenderer)
			}

		case "tools":
			send("tools/list", map[string]interface{}{})
			if resp, ok := waitResponse(); ok {
				if resp.Result != nil {
					resp.Result = patchToolsList(resp.Result)
				}
				printToolsList(resp)
			}

		case "help", "?":
			printHelp()

		case "quit", "exit", "q":
			quit = true

		default:
			fmt.Fprintln(os.Stderr, errorStyle.Render("Unknown command: /"+cmd+". Type /help for available commands."))
		}
	}

	for !quit {
		input, ok := readInput()
		if !ok {
			break // Ctrl-D
		}
		for _, seg := range splitQueue(input) {
			dispatch(seg)
			if quit {
				break
			}
		}
	}

	childIn.Close()
	child.Wait()
	scanWg.Wait()
}

func splitCommand(input string) (string, string) {
	parts := strings.SplitN(input, " ", 2)
	cmd := strings.ToLower(strings.TrimRight(parts[0], ":"))
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}
	return cmd, arg
}

// splitQueue splits input on " && " so users can chain commands.
func splitQueue(input string) []string {
	parts := strings.Split(input, " && ")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// steeringSpec maps a REPL slash command to a synthetic tool.
type steeringSpec struct {
	tool       string
	defaultKey string
	aliases    map[string]string // REPL key → tool param name
}

var steeringSpecs = map[string]steeringSpec{
	"emails": {
		tool: "search_emails", defaultKey: "keywords",
		aliases: map[string]string{"from": "from", "subject": "subject", "keywords": "keywords", "date": "date_range"},
	},
	"docs": {
		tool: "search_documents", defaultKey: "keywords",
		aliases: map[string]string{"name": "filename", "keywords": "keywords", "site": "site", "type": "file_type"},
	},
	"chats": {
		tool: "search_chats", defaultKey: "keywords",
		aliases: map[string]string{"with": "person", "keywords": "keywords", "date": "date_range"},
	},
	"channels": {
		tool: "search_channels", defaultKey: "keywords",
		aliases: map[string]string{"team": "team", "channel": "channel", "keywords": "keywords", "date": "date_range"},
	},
	"meetings": {
		tool: "search_meetings", defaultKey: "date_range",
		aliases: map[string]string{"by": "organizer", "subject": "subject", "date": "date_range"},
	},
	"people": {
		tool: "search_people", defaultKey: "name",
		aliases: map[string]string{"name": "name", "dept": "department", "project": "project"},
	},
}

// parseSteeringArgs splits an argument string into tool parameters.
// Tokens matching a known key: prefix start a new parameter; all other
// text accumulates into the current parameter (or the default key).
// Quoted strings ("...") are kept as single tokens with quotes stripped.
func parseSteeringArgs(arg string, spec steeringSpec) map[string]string {
	params := make(map[string]string)
	currentKey := ""
	var currentVal []string

	flush := func() {
		if len(currentVal) == 0 {
			return
		}
		val := strings.Join(currentVal, " ")
		if currentKey == "" {
			params[spec.defaultKey] = val
		} else if mapped, ok := spec.aliases[currentKey]; ok {
			params[mapped] = val
		}
		currentVal = nil
		currentKey = ""
	}

	for _, tok := range tokenize(arg) {
		if idx := strings.Index(tok, ":"); idx > 0 {
			key := strings.ToLower(tok[:idx])
			if _, ok := spec.aliases[key]; ok {
				flush()
				currentKey = key
				if rest := tok[idx+1:]; rest != "" {
					currentVal = append(currentVal, rest)
				}
				continue
			}
		}
		currentVal = append(currentVal, tok)
	}
	flush()

	return params
}

// tokenize splits s on whitespace but keeps quoted strings ("...") as
// single tokens with the surrounding quotes removed.
func tokenize(s string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQuote = !inQuote
		case c == ' ' && !inQuote:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func printReplResult(msg rpcMessage, r *glamour.TermRenderer) {
	if msg.Error != nil {
		detail := msg.Error.Message
		if msg.Error.Code != 0 {
			detail = fmt.Sprintf("%s (code %d)", detail, msg.Error.Code)
		}
		fmt.Fprintln(os.Stderr, errorStyle.Render("Error: "+detail))
		return
	}
	if msg.Result == nil {
		return
	}
	// Apply the same error enrichment the proxy adds.
	msg.Result = enrichToolCallResult(msg.Result)
	var result struct {
		Content []contentItem `json:"content"`
		IsError bool          `json:"isError"`
	}
	if err := json.Unmarshal(msg.Result, &result); err != nil {
		fmt.Fprintln(os.Stderr, string(msg.Result))
		return
	}
	if result.IsError {
		for _, c := range result.Content {
			if c.Type == "text" {
				fmt.Fprintln(os.Stderr, errorStyle.Render(c.Text))
			}
		}
		return
	}
	for _, c := range result.Content {
		if c.Type == "text" {
			fmt.Print(renderText(c.Text, r))
		}
	}
}

// linkRef holds a single extracted URL and its display label.
type linkRef struct {
	label string
	href  string
}

// bareURLRe matches bare URLs that are NOT already inside markdown link
// syntax ](url). It captures an optional preceding character in group 1
// and the URL in group 2.
var bareURLRe = regexp.MustCompile(`(^|[^(])(https?://[^\s)\]>]+)`)

// mdLinkRe matches markdown links [text](url).
var mdLinkRe = regexp.MustCompile(`\[([^\]]*)\]\((https?://[^)]+)\)`)

// brokenURLRe detects a URL split across lines.
var brokenURLRe = regexp.MustCompile(`(https?://\S+)\r?\n([^\s\[*#>][\w/?.=%&~+;:@!$()',*-]*)`)

// placeholderRe matches link placeholders inserted by preprocessURLs.
var placeholderRe = regexp.MustCompile(`⟦(\d+)⟧`)

// urlLabels maps URL host prefixes to friendly display labels.
var urlLabels = map[string]string{
	"teams.microsoft.com":   "Open in Teams",
	"outlook.office.com":    "Open in Outlook",
	"outlook.office365.com": "Open in Outlook",
	"outlook.live.com":      "Open in Outlook",
	"onedrive.live.com":     "Open in OneDrive",
	"sharepoint.com":        "Open in SharePoint",
	"github.com":            "Open on GitHub",
	"dev.azure.com":         "Open in Azure DevOps",
}

// urlDisplayLabel returns a friendly label for a URL based on its host.
func urlDisplayLabel(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "link"
	}
	host := strings.ToLower(u.Host)
	for suffix, label := range urlLabels {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return label
		}
	}
	return host
}

// preprocessURLs reassembles broken URLs, extracts all links (markdown
// and bare) into a ref slice, and replaces them with short placeholders
// like ⟦0⟧ that glamour can render without mangling.
func preprocessURLs(text string) (string, []linkRef) {
	// Rejoin URLs that were split across lines.
	for brokenURLRe.MatchString(text) {
		text = brokenURLRe.ReplaceAllString(text, "${1}${2}")
	}

	var refs []linkRef

	// Extract markdown links [text](url) → placeholder.
	text = mdLinkRe.ReplaceAllStringFunc(text, func(match string) string {
		groups := mdLinkRe.FindStringSubmatch(match)
		label := groups[1]
		href := groups[2]
		if len(label) <= 3 {
			label = urlDisplayLabel(href)
		}
		idx := len(refs)
		refs = append(refs, linkRef{label: label, href: href})
		return fmt.Sprintf("⟦%d⟧", idx)
	})

	// Extract bare URLs → placeholder.
	text = bareURLRe.ReplaceAllStringFunc(text, func(match string) string {
		groups := bareURLRe.FindStringSubmatch(match)
		prefix := groups[1]
		rawURL := groups[2]
		label := urlDisplayLabel(rawURL)
		idx := len(refs)
		refs = append(refs, linkRef{label: label, href: rawURL})
		return fmt.Sprintf("%s⟦%d⟧", prefix, idx)
	})

	return text, refs
}

// postprocessURLs replaces ⟦N⟧ placeholders in rendered output with
// OSC 8 terminal hyperlinks.
func postprocessURLs(text string, refs []linkRef) string {
	return placeholderRe.ReplaceAllStringFunc(text, func(match string) string {
		groups := placeholderRe.FindStringSubmatch(match)
		idx := 0
		fmt.Sscanf(groups[1], "%d", &idx)
		if idx < 0 || idx >= len(refs) {
			return match
		}
		ref := refs[idx]
		return "\033]8;;" + ref.href + "\033\\" + ref.label + "\033]8;;\033\\"
	})
}

// renderText is the full pipeline: preprocess URLs out of the markdown,
// render through glamour, then inject OSC 8 hyperlinks into the output.
func renderText(text string, r *glamour.TermRenderer) string {
	cleaned, refs := preprocessURLs(text)
	if r != nil {
		if rendered, err := r.Render(cleaned); err == nil {
			return postprocessURLs(rendered, refs)
		}
	}
	return postprocessURLs(cleaned, refs)
}

func printToolsList(msg rpcMessage) {
	if msg.Error != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render("Error: "+msg.Error.Message))
		return
	}
	if msg.Result == nil {
		return
	}
	var result struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(msg.Result, &result); err != nil {
		fmt.Fprintln(os.Stderr, string(msg.Result))
		return
	}
	fmt.Fprintln(os.Stderr, "")
	for _, t := range result.Tools {
		fmt.Fprintf(os.Stderr, "  %s %s\n",
			toolNameStyle.Render(t.Name),
			toolDescStyle.Render(t.Description))
	}
	fmt.Fprintln(os.Stderr, "")
}
