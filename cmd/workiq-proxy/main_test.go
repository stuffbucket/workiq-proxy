package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func TestBuildQuestion(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		args     map[string]string
		want     string
	}{
		{
			name:     "emails with all params",
			toolName: "search_emails",
			args:     map[string]string{"from": "Sarah", "subject": "budget", "keywords": "Q4", "date_range": "last week"},
			want:     "Find my emails from Sarah with subject about budget about Q4 from last week",
		},
		{
			name:     "emails partial",
			toolName: "search_emails",
			args:     map[string]string{"from": "Sarah"},
			want:     "Find my emails from Sarah",
		},
		{
			name:     "emails no params",
			toolName: "search_emails",
			args:     map[string]string{},
			want:     "Find my emails recent items",
		},
		{
			name:     "documents",
			toolName: "search_documents",
			args:     map[string]string{"filename": "report", "file_type": "xlsx"},
			want:     "Find documents named report of type xlsx",
		},
		{
			name:     "chats",
			toolName: "search_chats",
			args:     map[string]string{"person": "Alice", "keywords": "project update"},
			want:     "Find Teams chat messages with Alice about project update",
		},
		{
			name:     "channels",
			toolName: "search_channels",
			args:     map[string]string{"team": "Engineering", "channel": "general", "keywords": "deployment"},
			want:     "Find Teams channel messages in team Engineering in channel general about deployment",
		},
		{
			name:     "meetings",
			toolName: "search_meetings",
			args:     map[string]string{"organizer": "Bob", "date_range": "tomorrow"},
			want:     "Find my meetings organized by Bob from tomorrow",
		},
		{
			name:     "people",
			toolName: "search_people",
			args:     map[string]string{"name": "Jane", "department": "Finance"},
			want:     "Find people named Jane in Finance department",
		},
		{
			name:     "external",
			toolName: "search_external",
			args:     map[string]string{"keywords": "policy", "source": "Confluence"},
			want:     "Search external data in Confluence for policy",
		},
		{
			name:     "nil args",
			toolName: "search_emails",
			want:     "Find my emails recent items",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args json.RawMessage
			if tt.args != nil {
				args, _ = json.Marshal(tt.args)
			}
			got := buildQuestion(tt.toolName, args)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPatchCapabilities(t *testing.T) {
	t.Run("adds missing prompts and resources", func(t *testing.T) {
		interceptPrompts = true
		interceptResources = true
		input := `{"protocolVersion":"2024-11-05","capabilities":{"logging":{},"tools":{"listChanged":true}},"serverInfo":{"name":"WorkIQ"}}`
		result := patchCapabilities(json.RawMessage(input))
		var parsed initResult
		if err := json.Unmarshal(result, &parsed); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if parsed.ProtocolVersion != "2024-11-05" {
			t.Errorf("protocolVersion lost: got %q", parsed.ProtocolVersion)
		}
		if _, ok := parsed.Capabilities["prompts"]; !ok {
			t.Error("prompts not added")
		}
		if _, ok := parsed.Capabilities["resources"]; !ok {
			t.Error("resources not added")
		}
		if _, ok := parsed.Capabilities["logging"]; !ok {
			t.Error("logging lost")
		}
		if _, ok := parsed.Capabilities["tools"]; !ok {
			t.Error("tools lost")
		}
	})

	t.Run("self-disables on server prompts", func(t *testing.T) {
		interceptPrompts = true
		interceptResources = true
		input := `{"capabilities":{"logging":{},"tools":{},"prompts":{"listChanged":true}}}`
		patchCapabilities(json.RawMessage(input))
		if interceptPrompts {
			t.Error("interceptPrompts should be false")
		}
		if !interceptResources {
			t.Error("interceptResources should still be true")
		}
		interceptPrompts = true
	})

	t.Run("self-disables on server resources", func(t *testing.T) {
		interceptPrompts = true
		interceptResources = true
		input := `{"capabilities":{"logging":{},"tools":{},"resources":{}}}`
		patchCapabilities(json.RawMessage(input))
		if !interceptPrompts {
			t.Error("interceptPrompts should still be true")
		}
		if interceptResources {
			t.Error("interceptResources should be false")
		}
		interceptResources = true
	})

	t.Run("no capabilities passthrough", func(t *testing.T) {
		interceptPrompts = true
		interceptResources = true
		input := `{"serverInfo":{"name":"WorkIQ"}}`
		result := patchCapabilities(json.RawMessage(input))
		if string(result) != input {
			t.Errorf("expected passthrough, got %s", string(result))
		}
	})

	t.Run("preserves unknown fields", func(t *testing.T) {
		interceptPrompts = true
		interceptResources = true
		input := `{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"serverInfo":{"name":"WorkIQ"},"instructions":"Be helpful","futureField":42}`
		result := patchCapabilities(json.RawMessage(input))
		var m map[string]json.RawMessage
		json.Unmarshal(result, &m)
		if string(m["protocolVersion"]) != `"2024-11-05"` {
			t.Errorf("protocolVersion lost: got %s", string(m["protocolVersion"]))
		}
		if string(m["instructions"]) != `"Be helpful"` {
			t.Errorf("instructions lost: got %s", string(m["instructions"]))
		}
		if string(m["futureField"]) != `42` {
			t.Errorf("futureField lost: got %s", string(m["futureField"]))
		}
	})
}

func TestPatchToolsList(t *testing.T) {
	input := `{"tools":[{"name":"accept_eula"},{"name":"ask_work_iq"}]}`
	result := patchToolsList(json.RawMessage(input))
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var tools []json.RawMessage
	json.Unmarshal(parsed["tools"], &tools)
	if len(tools) != 9 {
		t.Errorf("expected 9 tools, got %d", len(tools))
	}
	names := make(map[string]bool)
	for _, raw := range tools {
		var tool struct{ Name string }
		json.Unmarshal(raw, &tool)
		names[tool.Name] = true
	}
	for _, exp := range []string{"search_emails", "search_documents", "search_chats", "search_channels", "search_meetings", "search_people", "search_external"} {
		if !names[exp] {
			t.Errorf("missing: %s", exp)
		}
	}

	t.Run("preserves unknown fields", func(t *testing.T) {
		input := `{"tools":[{"name":"ask_work_iq"}],"nextCursor":"abc123"}`
		result := patchToolsList(json.RawMessage(input))
		var m map[string]json.RawMessage
		json.Unmarshal(result, &m)
		if string(m["nextCursor"]) != `"abc123"` {
			t.Errorf("nextCursor lost: got %s", string(m["nextCursor"]))
		}
	})
}

func TestEnrichToolCallResult(t *testing.T) {
	t.Run("enriches conversation error", func(t *testing.T) {
		content, _ := json.Marshal([]contentItem{{Type: "text", Text: "Failed to create conversation: error"}})
		input, _ := json.Marshal(map[string]interface{}{"content": json.RawMessage(content), "isError": true})
		result := enrichToolCallResult(json.RawMessage(input))
		var parsed map[string]json.RawMessage
		json.Unmarshal(result, &parsed)
		var items []contentItem
		json.Unmarshal(parsed["content"], &items)
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
		if !strings.Contains(items[1].Text, "auth token expired") {
			t.Error("expected auth token hint")
		}
	})

	t.Run("enriches token protection error", func(t *testing.T) {
		content, _ := json.Marshal([]contentItem{{Type: "text", Text: "Error 530084: token protection policy blocked access"}})
		input, _ := json.Marshal(map[string]interface{}{"content": json.RawMessage(content), "isError": true})
		result := enrichToolCallResult(json.RawMessage(input))
		var parsed map[string]json.RawMessage
		json.Unmarshal(result, &parsed)
		var items []contentItem
		json.Unmarshal(parsed["content"], &items)
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
		if !strings.Contains(items[1].Text, "token protection") {
			t.Error("expected token protection hint")
		}
	})

	t.Run("enriches conditional access error", func(t *testing.T) {
		content, _ := json.Marshal([]contentItem{{Type: "text", Text: "AADSTS50076: some conditional access error"}})
		input, _ := json.Marshal(map[string]interface{}{"content": json.RawMessage(content), "isError": true})
		result := enrichToolCallResult(json.RawMessage(input))
		var parsed map[string]json.RawMessage
		json.Unmarshal(result, &parsed)
		var items []contentItem
		json.Unmarshal(parsed["content"], &items)
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
		if !strings.Contains(items[1].Text, "conditional access") {
			t.Error("expected conditional access hint")
		}
	})

	t.Run("preserves unknown fields", func(t *testing.T) {
		content, _ := json.Marshal([]contentItem{{Type: "text", Text: "Failed to create conversation: error"}})
		input, _ := json.Marshal(map[string]interface{}{"content": json.RawMessage(content), "isError": true, "_meta": map[string]string{"traceId": "xyz"}})
		result := enrichToolCallResult(json.RawMessage(input))
		var parsed map[string]json.RawMessage
		json.Unmarshal(result, &parsed)
		if _, ok := parsed["_meta"]; !ok {
			t.Error("_meta field lost during enrichment")
		}
	})

	t.Run("enriches eula error", func(t *testing.T) {
		content, _ := json.Marshal([]contentItem{{Type: "text", Text: "You must accept the EULA before using this tool"}})
		input, _ := json.Marshal(map[string]interface{}{"content": json.RawMessage(content), "isError": true})
		result := enrichToolCallResult(json.RawMessage(input))
		var parsed map[string]json.RawMessage
		json.Unmarshal(result, &parsed)
		var items []contentItem
		json.Unmarshal(parsed["content"], &items)
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
		if !strings.Contains(items[1].Text, "accept_eula") {
			t.Error("expected EULA acceptance hint")
		}
	})

	t.Run("enriches interaction required error", func(t *testing.T) {
		content, _ := json.Marshal([]contentItem{{Type: "text", Text: "MsalUiRequiredException: InteractionRequired - claims challenge"}})
		input, _ := json.Marshal(map[string]interface{}{"content": json.RawMessage(content), "isError": true})
		result := enrichToolCallResult(json.RawMessage(input))
		var parsed map[string]json.RawMessage
		json.Unmarshal(result, &parsed)
		var items []contentItem
		json.Unmarshal(parsed["content"], &items)
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
		if !strings.Contains(items[1].Text, "interactive browser") {
			t.Error("expected interactive auth hint")
		}
	})

	t.Run("passes through normal", func(t *testing.T) {
		content, _ := json.Marshal([]contentItem{{Type: "text", Text: "Here are your emails"}})
		input, _ := json.Marshal(map[string]interface{}{"content": json.RawMessage(content)})
		result := enrichToolCallResult(json.RawMessage(input))
		if string(result) != string(input) {
			t.Error("expected passthrough")
		}
	})
}

func TestMakeResult(t *testing.T) {
	id := json.RawMessage(`1`)
	result := makeResult(&id, map[string]interface{}{"prompts": []interface{}{}})
	var msg rpcMessage
	if err := json.Unmarshal(result, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.JSONRPC != "2.0" {
		t.Errorf("jsonrpc: got %s", msg.JSONRPC)
	}
	if string(*msg.ID) != "1" {
		t.Errorf("id: got %s", string(*msg.ID))
	}
}

func TestSyntheticToolNames(t *testing.T) {
	for _, name := range []string{"search_emails", "search_documents", "search_chats", "search_channels", "search_meetings", "search_people", "search_external"} {
		if !syntheticToolNames[name] {
			t.Errorf("missing: %s", name)
		}
	}
	if syntheticToolNames["ask_work_iq"] {
		t.Error("ask_work_iq should not be synthetic")
	}
}

func TestRenderWithURLs(t *testing.T) {
	r := newMarkdownRenderer()
	if r == nil {
		t.Fatal("failed to create markdown renderer")
	}

	osc := func(label, href string) string {
		return "\033]8;;" + href + "\033\\" + label + "\033]8;;\033\\"
	}

	// stripOSC8 removes OSC 8 escape sequences, leaving only visible text.
	osc8Re := regexp.MustCompile(`\033\]8;;[^\033]*\033\\`)
	stripOSC8 := func(s string) string {
		return osc8Re.ReplaceAllString(s, "")
	}

	longTeamsURL := "https://teams.microsoft.com/l/meeting/details?eventId=AAMkADkwNzEwMmNkLWFmNGQtNDFlZS04YjJhLTZhZDI5NmU1NmIxZAFRAAgI3nQA0bBAAEYAAAAA2NHrSsPBXU6PC_oSYMmssgcAr7CiMCsTiUu65ueNDQ53KwAAAHljZAAAcbrkZ7o-DEm7jz5iZCjevgAN83CBRAAAEA%3d%3d"

	tests := []struct {
		name  string
		input string
		check func(string) error
	}{
		{
			name:  "broken Teams URL reassembled and rendered cleanly",
			input: "Note: This meeting conflicts with another. [1](https://teams.microsoft.\ncom/l/meeting/details?eventId=AAAA)\n\nDetails",
			check: func(got string) error {
				visible := stripOSC8(got)
				if strings.Contains(visible, "ttps://") {
					return fmt.Errorf("raw URL fragment visible in display text: %q", visible)
				}
				if strings.Contains(visible, "]8;;") {
					return fmt.Errorf("raw OSC 8 escape visible: %q", visible)
				}
				want := osc("Open in Teams", "https://teams.microsoft.com/l/meeting/details?eventId=AAAA")
				if !strings.Contains(got, want) {
					return fmt.Errorf("missing OSC 8 hyperlink in output: %q", got)
				}
				return nil
			},
		},
		{
			name:  "long Teams URL from markdown link not broken across lines",
			input: "Meeting result [1](" + longTeamsURL + ") here",
			check: func(got string) error {
				visible := stripOSC8(got)
				// The raw URL must NOT appear in visible display text.
				if strings.Contains(visible, "eventId=") {
					return fmt.Errorf("raw URL leaked into visible text: %q", visible)
				}
				want := osc("Open in Teams", longTeamsURL)
				if !strings.Contains(got, want) {
					return fmt.Errorf("missing OSC 8 hyperlink: %q", got)
				}
				return nil
			},
		},
		{
			name:  "bare URL on its own line",
			input: "See link below\nhttps://github.com/foo/bar\n\nDone",
			check: func(got string) error {
				if strings.Contains(got, "https://github.com") && !strings.Contains(got, "\033]8;;") {
					return fmt.Errorf("bare URL not converted to OSC 8: %q", got)
				}
				want := osc("Open on GitHub", "https://github.com/foo/bar")
				if !strings.Contains(got, want) {
					return fmt.Errorf("missing OSC 8 hyperlink: %q", got)
				}
				return nil
			},
		},
		{
			name:  "markdown link with descriptive text preserved",
			input: "Click [see the docs](https://example.com/help) for more info",
			check: func(got string) error {
				want := osc("see the docs", "https://example.com/help")
				if !strings.Contains(got, want) {
					return fmt.Errorf("missing OSC 8 hyperlink: %q", got)
				}
				return nil
			},
		},
		{
			name: "multi-line broken bare URL reassembled",
			input: "Check https://teams.microsoft.\ncom/l/meeting/details?eventId=AAMkADkw\n\nDetails",
			check: func(got string) error {
				if strings.Contains(got, "\ncom/l/") {
					return fmt.Errorf("URL still broken across lines: %q", got)
				}
				want := osc("Open in Teams", "https://teams.microsoft.com/l/meeting/details?eventId=AAMkADkw")
				if !strings.Contains(got, want) {
					return fmt.Errorf("missing OSC 8 hyperlink: %q", got)
				}
				return nil
			},
		},
		{
			name:  "no mangled escape sequences in output",
			input: "Result [1](" + longTeamsURL + ")\n\n* Meeting: Test\n* Time: 9:00 AM",
			check: func(got string) error {
				// Check for common mangling patterns.
				if strings.Contains(got, "0m") && strings.Contains(got, "ttps://") {
					return fmt.Errorf("ANSI escape mangling detected: %q", got)
				}
				if strings.Contains(got, "]8;;") && !strings.Contains(got, "\033]8;;") {
					return fmt.Errorf("broken OSC 8 sequence (missing ESC): %q", got)
				}
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderText(tt.input, r)
			if err := tt.check(got); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestPreprocessURLs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(string, []linkRef) error
	}{
		{
			name:  "replaces markdown link with placeholder",
			input: "Click [here](https://example.com/page) now",
			check: func(got string, refs []linkRef) error {
				if strings.Contains(got, "https://") {
					return fmt.Errorf("URL still in text: %q", got)
				}
				if len(refs) != 1 {
					return fmt.Errorf("expected 1 ref, got %d", len(refs))
				}
				if refs[0].href != "https://example.com/page" {
					return fmt.Errorf("wrong href: %s", refs[0].href)
				}
				if refs[0].label != "here" {
					return fmt.Errorf("wrong label: %s", refs[0].label)
				}
				return nil
			},
		},
		{
			name:  "reassembles broken URL then replaces",
			input: "Go to https://teams.microsoft.\ncom/l/meeting?id=X\n\nDone",
			check: func(got string, refs []linkRef) error {
				if strings.Contains(got, "\ncom/l/") {
					return fmt.Errorf("URL still broken: %q", got)
				}
				if len(refs) != 1 {
					return fmt.Errorf("expected 1 ref, got %d", len(refs))
				}
				if refs[0].href != "https://teams.microsoft.com/l/meeting?id=X" {
					return fmt.Errorf("wrong href: %s", refs[0].href)
				}
				return nil
			},
		},
		{
			name:  "numeric link text gets friendly label",
			input: "result [1](https://teams.microsoft.com/foo) here",
			check: func(got string, refs []linkRef) error {
				if len(refs) != 1 {
					return fmt.Errorf("expected 1 ref, got %d", len(refs))
				}
				if refs[0].label != "Open in Teams" {
					return fmt.Errorf("expected friendly label, got: %s", refs[0].label)
				}
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, refs := preprocessURLs(tt.input)
			if err := tt.check(got, refs); err != nil {
				t.Error(err)
			}
		})
	}
}
