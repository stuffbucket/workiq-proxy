package main

import (
	"encoding/json"
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
