package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

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

func printPrompt() {
	fmt.Fprint(os.Stderr, promptStyle.Render("wiq")+" "+subtleStyle.Render("›")+" ")
}

func printHelp() {
	cmds := []struct{ cmd, desc string }{
		{"ask <question>", "Ask Microsoft 365 Copilot a question"},
		{"accept-eula", "Accept the Work IQ EULA"},
		{"tools", "List available MCP tools"},
		{"help", "Show this help"},
		{"quit", "Exit"},
	}
	fmt.Fprintln(os.Stderr, "")
	for _, c := range cmds {
		fmt.Fprintf(os.Stderr, "  %s  %s\n",
			cmdStyle.Render(fmt.Sprintf("%-20s", c.cmd)),
			descStyle.Render(c.desc))
	}
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, subtleStyle.Render("  Bare text is treated as a question."))
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
		msg, ok := <-responses
		return msg, ok
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

	scanner := bufio.NewScanner(os.Stdin)
	printPrompt()

	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			printPrompt()
			continue
		}

		cmd, arg := splitCommand(input)

		switch cmd {
		case "ask":
			if arg == "" {
				fmt.Fprintln(os.Stderr, errorStyle.Render("Usage: ask <question>"))
			} else {
				send("tools/call", toolCallParams{
					Name:      "ask_work_iq",
					Arguments: mustMarshal(map[string]string{"question": arg}),
				})
				if resp, ok := waitResponse(); ok {
					printReplResult(resp, mdRenderer)
				}
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
				printToolsList(resp)
			}

		case "help", "?":
			printHelp()

		case "quit", "exit", "q":
			childIn.Close()
			child.Wait()
			scanWg.Wait()
			return

		default:
			// Treat bare input as a question.
			send("tools/call", toolCallParams{
				Name:      "ask_work_iq",
				Arguments: mustMarshal(map[string]string{"question": input}),
			})
			if resp, ok := waitResponse(); ok {
				printReplResult(resp, mdRenderer)
			}
		}

		printPrompt()
	}

	childIn.Close()
	child.Wait()
	scanWg.Wait()
}

func splitCommand(input string) (string, string) {
	parts := strings.SplitN(input, " ", 2)
	cmd := strings.ToLower(parts[0])
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}
	return cmd, arg
}

func printReplResult(msg rpcMessage, r *glamour.TermRenderer) {
	if msg.Error != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render("Error: "+msg.Error.Message))
		return
	}
	if msg.Result == nil {
		return
	}
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
			if r != nil {
				if rendered, err := r.Render(c.Text); err == nil {
					fmt.Print(rendered)
					continue
				}
			}
			fmt.Println(c.Text)
		}
	}
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
