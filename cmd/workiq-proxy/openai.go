package main

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
)

// ── OpenAI-compatible types ─────────────────────────────────────────

// ChatCompletionRequest mirrors the OpenAI POST /v1/chat/completions body.
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletion is the non-streaming response.
type ChatCompletion struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   ChatUsage    `json:"usage"`
}

type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionChunk is one SSE frame for streaming.
type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
}

type ChunkChoice struct {
	Index        int         `json:"index"`
	Delta        ChatMessage `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

// ModelObject is one entry in the /v1/models list.
type ModelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type ModelList struct {
	Object string        `json:"object"`
	Data   []ModelObject `json:"data"`
}

// OpenAIError follows the OpenAI error envelope.
type OpenAIError struct {
	Error OpenAIErrorDetail `json:"error"`
}

type OpenAIErrorDetail struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Code    *string `json:"code"`
}

// contentTypeText is the MCP content item type for plain text.
const contentTypeText = "text"

// unwrapResponseText handles the case where the MCP backend returns
// a JSON envelope {"response":"...","conversationId":"..."} inside
// a text content item. If the text is such an envelope, the inner
// response string is returned; otherwise the text is returned as-is.
func unwrapResponseText(text string) string {
	if len(text) < 2 || text[0] != '{' {
		return text
	}
	var envelope struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal([]byte(text), &envelope); err != nil || envelope.Response == "" {
		return text
	}
	return envelope.Response
}

// finishReasonStop is the OpenAI finish_reason for a completed response.
const finishReasonStop = "stop"

// ── Builders ────────────────────────────────────────────────────────

var chatIDCounter atomic.Int64

func nextChatID() string {
	return fmt.Sprintf("chatcmpl-%d", chatIDCounter.Add(1))
}

func buildChatCompletion(model, content string) ChatCompletion {
	return ChatCompletion{
		ID:      nextChatID(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []ChatChoice{{
			Index:        0,
			Message:      ChatMessage{Role: "assistant", Content: content},
			FinishReason: finishReasonStop,
		}},
		Usage: ChatUsage{}, // MCP doesn't provide token counts
	}
}

func buildChatChunk(id, model, content string, done bool) ChatCompletionChunk {
	choice := ChunkChoice{
		Index: 0,
		Delta: ChatMessage{Role: "assistant", Content: content},
	}
	if done {
		reason := finishReasonStop
		choice.FinishReason = &reason
		choice.Delta = ChatMessage{}
	}
	return ChatCompletionChunk{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []ChunkChoice{choice},
	}
}

func buildModelList() ModelList {
	return ModelList{
		Object: "list",
		Data: []ModelObject{{
			ID:      "workiq",
			Object:  "model",
			Created: 1700000000,
			OwnedBy: "microsoft",
		}},
	}
}

func buildOpenAIError(status int, message string) OpenAIError {
	var errType string
	switch status {
	case 400:
		errType = "invalid_request_error"
	case 404:
		errType = "not_found_error"
	default:
		errType = "api_error"
	}
	return OpenAIError{Error: OpenAIErrorDetail{
		Message: message,
		Type:    errType,
	}}
}

// extractTextContent pulls the combined text from an MCP tool call result.
func extractTextContent(result json.RawMessage) (string, bool) {
	var r struct {
		Content []contentItem `json:"content"`
		IsError bool          `json:"isError"`
	}
	if err := json.Unmarshal(result, &r); err != nil {
		return "", false
	}
	if r.IsError {
		var parts []string
		for _, c := range r.Content {
			if c.Type == contentTypeText {
				parts = append(parts, unwrapResponseText(c.Text))
			}
		}
		return fmt.Sprintf("Error: %s", joinNonEmpty(parts)), true
	}
	var parts []string
	for _, c := range r.Content {
		if c.Type == contentTypeText {
			parts = append(parts, unwrapResponseText(c.Text))
		}
	}
	return joinNonEmpty(parts), false
}

func joinNonEmpty(parts []string) string {
	out := ""
	for _, p := range parts {
		if p != "" {
			if out != "" {
				out += "\n"
			}
			out += p
		}
	}
	return out
}
