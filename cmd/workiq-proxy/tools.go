package main

import (
	"encoding/json"
	"strings"
)

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
	out := make([]json.RawMessage, 0, len(syntheticTools))
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
