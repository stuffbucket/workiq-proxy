package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	workiqCmd = flag.String("workiq-cmd", "npx -y @microsoft/workiq mcp", "Command to spawn the Work IQ MCP server")
	logFile   = flag.String("log-file", "", "Override log file path (default: ~/.work-iq-cli/mcp-traffic.log)")
	noLog     = flag.Bool("no-log", false, "Disable traffic logging")
)

func stateDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".work-iq-cli")
}

func tokenCachePath() string {
	return filepath.Join(stateDir(), "msal_token_cache.dat")
}

func resolveLogFile() string {
	if *logFile != "" {
		return *logFile
	}
	return filepath.Join(stateDir(), "mcp-traffic.log")
}

type rpcMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *rpcErrorObj     `json:"error,omitempty"`
}

type rpcErrorObj struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type initResult struct {
	Capabilities map[string]json.RawMessage `json:"capabilities,omitempty"`
	ServerInfo   json.RawMessage            `json:"serverInfo,omitempty"`
}

type toolsListResult struct {
	Tools []json.RawMessage `json:"tools"`
}

type toolCallResult struct {
	Content json.RawMessage `json:"content"`
	IsError bool            `json:"isError,omitempty"`
}

type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func logMsg(direction string, data []byte) {
	if *noLog {
		return
	}
	p := resolveLogFile()
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s %s\n", time.Now().UTC().Format(time.RFC3339Nano), direction, string(data))
}

var (
	interceptPrompts   = true
	interceptResources = true
)

var (
	pending   = make(map[string]string)
	pendingMu sync.Mutex
)

func idString(raw *json.RawMessage) string {
	if raw == nil {
		return ""
	}
	return string(*raw)
}

func trackRequest(id *json.RawMessage, method string) {
	if id == nil {
		return
	}
	pendingMu.Lock()
	pending[idString(id)] = method
	pendingMu.Unlock()
}

func popRequest(id *json.RawMessage) string {
	if id == nil {
		return ""
	}
	key := idString(id)
	pendingMu.Lock()
	method := pending[key]
	delete(pending, key)
	pendingMu.Unlock()
	return method
}

func peekRequest(id *json.RawMessage) string {
	if id == nil {
		return ""
	}
	pendingMu.Lock()
	method := pending[idString(id)]
	pendingMu.Unlock()
	return method
}

func makeResult(id *json.RawMessage, result interface{}) []byte {
	r, _ := json.Marshal(result)
	msg := rpcMessage{JSONRPC: "2.0", ID: id, Result: r}
	out, _ := json.Marshal(msg)
	return out
}

func hasAuthToken() bool {
	info, err := os.Stat(tokenCachePath())
	if err != nil {
		return false
	}
	return info.Size() > 0
}

func enrichToolCallResult(raw json.RawMessage) json.RawMessage {
	var result toolCallResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return raw
	}
	contentStr := string(result.Content)
	if !strings.Contains(contentStr, "Failed to create conversation") {
		return raw
	}
	var items []contentItem
	if err := json.Unmarshal(result.Content, &items); err != nil {
		return raw
	}
	items = append(items, contentItem{
		Type: "text",
		Text: "\n\n[workiq-proxy] This usually means your auth token expired or your Copilot license is not active. Try running: workiq ask -q \"What's on my calendar?\" in your terminal to re-authenticate.",
	})
	result.Content, _ = json.Marshal(items)
	out, _ := json.Marshal(result)
	return out
}

func patchCapabilities(raw json.RawMessage) json.RawMessage {
	var result initResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return raw
	}
	if result.Capabilities == nil {
		return raw
	}
	if _, ok := result.Capabilities["prompts"]; ok {
		interceptPrompts = false
	}
	if _, ok := result.Capabilities["resources"]; ok {
		interceptResources = false
	}
	if interceptPrompts {
		result.Capabilities["prompts"] = json.RawMessage(`{}`)
	}
	if interceptResources {
		result.Capabilities["resources"] = json.RawMessage(`{}`)
	}
	out, _ := json.Marshal(result)
	return out
}

type syntheticToolDef struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	InputSchema syntheticToolInputSchema `json:"inputSchema"`
}

type syntheticToolInputSchema struct {
	Type       string                             `json:"type"`
	Properties map[string]syntheticToolPropSchema `json:"properties"`
}

type syntheticToolPropSchema struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

var syntheticTools = []syntheticToolDef{
	{
		Name:        "search_emails",
		Description: "Search your Microsoft 365 emails. Finds messages matching the given criteria using Microsoft 365 Copilot.",
		InputSchema: syntheticToolInputSchema{
			Type: "object",
			Properties: map[string]syntheticToolPropSchema{
				"from":       {Type: "string", Description: "Sender name or email address to filter by"},
				"subject":    {Type: "string", Description: "Subject line keywords to search for"},
				"keywords":   {Type: "string", Description: "Keywords to search for in the email body"},
				"date_range": {Type: "string", Description: "Time period to search (e.g. last week, yesterday, January 2026)"},
			},
		},
	},
	{
		Name:        "search_documents",
		Description: "Search your Microsoft 365 documents in SharePoint and OneDrive. Finds files matching the given criteria using Microsoft 365 Copilot.",
		InputSchema: syntheticToolInputSchema{
			Type: "object",
			Properties: map[string]syntheticToolPropSchema{
				"filename":  {Type: "string", Description: "Filename or partial filename to search for"},
				"keywords":  {Type: "string", Description: "Keywords to search for in document content"},
				"site":      {Type: "string", Description: "SharePoint site name to search within"},
				"file_type": {Type: "string", Description: "File type to filter by (e.g. docx, xlsx, pdf)"},
			},
		},
	},
	{
		Name:        "search_chats",
		Description: "Search your Microsoft Teams chat messages. Finds messages matching the given criteria using Microsoft 365 Copilot.",
		InputSchema: syntheticToolInputSchema{
			Type: "object",
			Properties: map[string]syntheticToolPropSchema{
				"person":     {Type: "string", Description: "Person name to search chats with"},
				"keywords":   {Type: "string", Description: "Keywords to search for in chat messages"},
				"date_range": {Type: "string", Description: "Time period to search (e.g. last week, yesterday)"},
			},
		},
	},
	{
		Name:        "search_channels",
		Description: "Search Microsoft Teams channel messages. Finds messages matching the given criteria using Microsoft 365 Copilot.",
		InputSchema: syntheticToolInputSchema{
			Type: "object",
			Properties: map[string]syntheticToolPropSchema{
				"channel":    {Type: "string", Description: "Channel name to search within"},
				"team":       {Type: "string", Description: "Team name to search within"},
				"keywords":   {Type: "string", Description: "Keywords to search for in channel messages"},
				"date_range": {Type: "string", Description: "Time period to search (e.g. last week, yesterday)"},
			},
		},
	},
	{
		Name:        "search_meetings",
		Description: "Search your Microsoft 365 meetings and meeting transcripts. Finds meetings matching the given criteria using Microsoft 365 Copilot.",
		InputSchema: syntheticToolInputSchema{
			Type: "object",
			Properties: map[string]syntheticToolPropSchema{
				"organizer":  {Type: "string", Description: "Meeting organizer name to filter by"},
				"subject":    {Type: "string", Description: "Meeting subject keywords to search for"},
				"date_range": {Type: "string", Description: "Time period to search (e.g. last week, tomorrow, next Monday)"},
			},
		},
	},
	{
		Name:        "search_people",
		Description: "Search for people in your Microsoft 365 organization. Finds people matching the given criteria using Microsoft 365 Copilot.",
		InputSchema: syntheticToolInputSchema{
			Type: "object",
			Properties: map[string]syntheticToolPropSchema{
				"name":       {Type: "string", Description: "Person's name to search for"},
				"department": {Type: "string", Description: "Department to search within"},
				"project":    {Type: "string", Description: "Project or topic the person is associated with"},
			},
		},
	},
	{
		Name:        "search_external",
		Description: "Search external data sources connected to your Microsoft 365 environment. Finds items matching the given criteria using Microsoft 365 Copilot.",
		InputSchema: syntheticToolInputSchema{
			Type: "object",
			Properties: map[string]syntheticToolPropSchema{
				"keywords": {Type: "string", Description: "Keywords to search for"},
				"source":   {Type: "string", Description: "External data source name to search within"},
			},
		},
	},
}

var syntheticToolNames = func() map[string]bool {
	m := make(map[string]bool)
	for _, t := range syntheticTools {
		m[t.Name] = true
	}
	return m
}()

func syntheticToolsJSON() []json.RawMessage {
	var out []json.RawMessage
	for _, t := range syntheticTools {
		b, _ := json.Marshal(t)
		out = append(out, b)
	}
	return out
}

func buildQuestion(toolName string, args json.RawMessage) string {
	var params map[string]string
	if err := json.Unmarshal(args, &params); err != nil {
		params = map[string]string{}
	}
	var parts []string
	switch toolName {
	case "search_emails":
		parts = append(parts, "Find my emails")
		if v := params["from"]; v != "" {
			parts = append(parts, "from "+v)
		}
		if v := params["subject"]; v != "" {
			parts = append(parts, "with subject about "+v)
		}
		if v := params["keywords"]; v != "" {
			parts = append(parts, "about "+v)
		}
		if v := params["date_range"]; v != "" {
			parts = append(parts, "from "+v)
		}
	case "search_documents":
		parts = append(parts, "Find documents")
		if v := params["filename"]; v != "" {
			parts = append(parts, "named "+v)
		}
		if v := params["keywords"]; v != "" {
			parts = append(parts, "about "+v)
		}
		if v := params["site"]; v != "" {
			parts = append(parts, "on site "+v)
		}
		if v := params["file_type"]; v != "" {
			parts = append(parts, "of type "+v)
		}
	case "search_chats":
		parts = append(parts, "Find Teams chat messages")
		if v := params["person"]; v != "" {
			parts = append(parts, "with "+v)
		}
		if v := params["keywords"]; v != "" {
			parts = append(parts, "about "+v)
		}
		if v := params["date_range"]; v != "" {
			parts = append(parts, "from "+v)
		}
	case "search_channels":
		parts = append(parts, "Find Teams channel messages")
		if v := params["team"]; v != "" {
			parts = append(parts, "in team "+v)
		}
		if v := params["channel"]; v != "" {
			parts = append(parts, "in channel "+v)
		}
		if v := params["keywords"]; v != "" {
			parts = append(parts, "about "+v)
		}
		if v := params["date_range"]; v != "" {
			parts = append(parts, "from "+v)
		}
	case "search_meetings":
		parts = append(parts, "Find my meetings")
		if v := params["organizer"]; v != "" {
			parts = append(parts, "organized by "+v)
		}
		if v := params["subject"]; v != "" {
			parts = append(parts, "about "+v)
		}
		if v := params["date_range"]; v != "" {
			parts = append(parts, "from "+v)
		}
	case "search_people":
		parts = append(parts, "Find people")
		if v := params["name"]; v != "" {
			parts = append(parts, "named "+v)
		}
		if v := params["department"]; v != "" {
			parts = append(parts, "in "+v+" department")
		}
		if v := params["project"]; v != "" {
			parts = append(parts, "working on "+v)
		}
	case "search_external":
		parts = append(parts, "Search external data")
		if v := params["source"]; v != "" {
			parts = append(parts, "in "+v)
		}
		if v := params["keywords"]; v != "" {
			parts = append(parts, "for "+v)
		}
	}
	if len(parts) == 1 {
		parts = append(parts, "recent items")
	}
	return strings.Join(parts, " ")
}

func patchToolsList(raw json.RawMessage) json.RawMessage {
	var result toolsListResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return raw
	}
	result.Tools = append(result.Tools, syntheticToolsJSON()...)
	out, _ := json.Marshal(result)
	return out
}

func main() {
	flag.Parse()

	cmdParts := strings.Fields(*workiqCmd)
	if len(cmdParts) == 0 {
		fmt.Fprintln(os.Stderr, "workiq-proxy: --workiq-cmd cannot be empty")
		os.Exit(1)
	}

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
		fmt.Fprintf(os.Stderr, "workiq-proxy: failed to start %q: %v\n", *workiqCmd, err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigCh {
			child.Process.Signal(sig)
		}
	}()

	var outMu sync.Mutex
	writeOut := func(data []byte) {
		outMu.Lock()
		os.Stdout.Write(data)
		os.Stdout.Write([]byte("\n"))
		outMu.Unlock()
	}

	authFailContent := mustMarshal([]contentItem{{
		Type: "text",
		Text: "No cached auth token found. Run this in your terminal first:\n\n  workiq ask -q \"What's on my calendar?\"\n\nThis triggers browser-based authentication and caches the token for MCP use.",
	}})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(childOut)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			trimmed := strings.TrimSpace(string(line))
			if trimmed == "" {
				continue
			}
			var msg rpcMessage
			if err := json.Unmarshal([]byte(trimmed), &msg); err != nil {
				logMsg("<<<", line)
				writeOut(line)
				continue
			}
			method := peekRequest(msg.ID)
			if method == "initialize" && msg.Result != nil {
				msg.Result = patchCapabilities(msg.Result)
			}
			if method == "tools/list" && msg.Result != nil {
				msg.Result = patchToolsList(msg.Result)
			}
			if method == "tools/call" && msg.Result != nil {
				msg.Result = enrichToolCallResult(msg.Result)
			}
			if msg.ID != nil {
				popRequest(msg.ID)
			}
			out, _ := json.Marshal(msg)
			logMsg("<<<", out)
			writeOut(out)
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			childIn.Write([]byte(line + "\n"))
			continue
		}
		logMsg(">>>", []byte(line))
		if msg.Method == "prompts/list" && interceptPrompts {
			resp := makeResult(msg.ID, map[string]interface{}{"prompts": []interface{}{}})
			logMsg("<<<", resp)
			writeOut(resp)
			continue
		}
		if msg.Method == "resources/list" && interceptResources {
			resp := makeResult(msg.ID, map[string]interface{}{"resources": []interface{}{}})
			logMsg("<<<", resp)
			writeOut(resp)
			continue
		}
		if msg.Method == "resources/templates/list" && interceptResources {
			resp := makeResult(msg.ID, map[string]interface{}{"resourceTemplates": []interface{}{}})
			logMsg("<<<", resp)
			writeOut(resp)
			continue
		}
		if msg.Method == "tools/call" && msg.Params != nil {
			var params toolCallParams
			if err := json.Unmarshal(msg.Params, &params); err == nil {
				if params.Name == "ask_work_iq" && !hasAuthToken() {
					resp := makeResult(msg.ID, toolCallResult{Content: authFailContent, IsError: true})
					logMsg("<<<", resp)
					writeOut(resp)
					continue
				}
				if syntheticToolNames[params.Name] {
					if !hasAuthToken() {
						resp := makeResult(msg.ID, toolCallResult{Content: authFailContent, IsError: true})
						logMsg("<<<", resp)
						writeOut(resp)
						continue
					}
					question := buildQuestion(params.Name, params.Arguments)
					newParams := toolCallParams{
						Name:      "ask_work_iq",
						Arguments: mustMarshal(map[string]string{"question": question}),
					}
					msg.Params = mustMarshal(newParams)
					trackRequest(msg.ID, "tools/call")
					rewritten, _ := json.Marshal(msg)
					logMsg(">>>", rewritten)
					childIn.Write(rewritten)
					childIn.Write([]byte("\n"))
					continue
				}
			}
		}
		if msg.ID != nil && msg.Method != "" {
			trackRequest(msg.ID, msg.Method)
		}
		childIn.Write([]byte(line + "\n"))
	}

	childIn.Close()
	wg.Wait()

	exitCode := 1
	if err := child.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	} else {
		exitCode = 0
	}

	logMsg("---", mustMarshal(map[string]interface{}{"event": "child-exit", "code": exitCode}))
	os.Exit(exitCode)
}

func mustMarshal(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
