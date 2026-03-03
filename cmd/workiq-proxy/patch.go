package main

import (
	"encoding/json"
	"strings"
)

var (
	interceptPrompts   = true
	interceptResources = true
)

func patchCapabilities(raw json.RawMessage) json.RawMessage {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	capRaw, ok := m["capabilities"]
	if !ok {
		return raw
	}
	var caps map[string]json.RawMessage
	if err := json.Unmarshal(capRaw, &caps); err != nil {
		return raw
	}
	if _, ok := caps["prompts"]; ok {
		interceptPrompts = false
	}
	if _, ok := caps["resources"]; ok {
		interceptResources = false
	}
	if interceptPrompts {
		caps["prompts"] = json.RawMessage(`{}`)
	}
	if interceptResources {
		caps["resources"] = json.RawMessage(`{}`)
	}
	m["capabilities"], _ = json.Marshal(caps)
	out, _ := json.Marshal(m)
	return out
}

func patchToolsList(raw json.RawMessage) json.RawMessage {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	toolsRaw, ok := m["tools"]
	if !ok {
		return raw
	}
	var existing []json.RawMessage
	if err := json.Unmarshal(toolsRaw, &existing); err != nil {
		return raw
	}
	tools := make([]json.RawMessage, 0, len(existing)+len(syntheticTools))
	tools = append(tools, existing...)
	tools = append(tools, syntheticToolsJSON()...)
	m["tools"], _ = json.Marshal(tools)
	out, _ := json.Marshal(m)
	return out
}

func enrichToolCallResult(raw json.RawMessage) json.RawMessage {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	contentRaw, ok := m["content"]
	if !ok {
		return raw
	}
	contentStr := string(contentRaw)

	var hint string
	contentLower := strings.ToLower(contentStr)
	switch {
	case strings.Contains(contentLower, "eula") || strings.Contains(contentLower, "license agreement") || strings.Contains(contentLower, "accept_eula"):
		hint = "\n\n[workiq-proxy] The Work IQ EULA has not been accepted. Run: workiq-proxy accept-eula (or: npx @stuffbucket/workiq-proxy accept-eula) in your terminal, or call the accept_eula tool with eulaUrl set to https://github.com/microsoft/work-iq-mcp"
	case strings.Contains(contentStr, "530084") || strings.Contains(contentStr, "token protection"):
		hint = "\n\n[workiq-proxy] Your organization requires token protection (error 530084). Ask your IT admin to exempt the Work IQ CLI app (ba081686-5d24-4bc6-a0d6-d034ecffed87) from the token protection conditional access policy."
	case strings.Contains(contentStr, "AADSTS") || strings.Contains(contentStr, "security policy"):
		hint = "\n\n[workiq-proxy] A Microsoft Entra ID conditional access policy is blocking this request. Check the error code and contact your IT admin."
	case strings.Contains(contentStr, "InteractionRequired") || strings.Contains(contentStr, "interaction_required"):
		hint = "\n\n[workiq-proxy] Authentication requires an interactive browser login. Run: workiq ask -q \"What's on my calendar?\" in your terminal to re-authenticate, then retry."
	case strings.Contains(contentStr, "Failed to create conversation"):
		hint = "\n\n[workiq-proxy] This usually means your auth token expired or your Copilot license is not active. Try running: workiq ask -q \"What's on my calendar?\" in your terminal to re-authenticate."
	default:
		return raw
	}

	var existing2 []contentItem
	if err := json.Unmarshal(contentRaw, &existing2); err != nil {
		return raw
	}
	items := make([]contentItem, 0, len(existing2)+1)
	items = append(items, existing2...)
	items = append(items, contentItem{Type: contentTypeText, Text: hint})
	m["content"], _ = json.Marshal(items)
	out, _ := json.Marshal(m)
	return out
}
