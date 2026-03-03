package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── Type builder tests ──────────────────────────────────────────────

func TestBuildChatCompletion(t *testing.T) {
	resp := buildChatCompletion("workiq", "Hello, world!")

	if resp.Object != "chat.completion" {
		t.Errorf("object = %q, want chat.completion", resp.Object)
	}
	if resp.Model != "workiq" {
		t.Errorf("model = %q, want workiq", resp.Model)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("len(choices) = %d, want 1", len(resp.Choices))
	}
	c := resp.Choices[0]
	if c.Message.Role != "assistant" {
		t.Errorf("role = %q, want assistant", c.Message.Role)
	}
	if c.Message.Content != "Hello, world!" {
		t.Errorf("content = %q, want Hello, world!", c.Message.Content)
	}
	if c.FinishReason != "stop" {
		t.Errorf("finish_reason = %q, want stop", c.FinishReason)
	}
	if resp.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestBuildChatChunk(t *testing.T) {
	chunk := buildChatChunk("chatcmpl-1", "workiq", "partial", false)
	if chunk.Object != "chat.completion.chunk" {
		t.Errorf("object = %q, want chat.completion.chunk", chunk.Object)
	}
	if chunk.Choices[0].Delta.Content != "partial" {
		t.Errorf("delta.content = %q, want partial", chunk.Choices[0].Delta.Content)
	}
	if chunk.Choices[0].FinishReason != nil {
		t.Error("expected nil finish_reason for non-done chunk")
	}

	done := buildChatChunk("chatcmpl-1", "workiq", "", true)
	if done.Choices[0].FinishReason == nil || *done.Choices[0].FinishReason != "stop" {
		t.Error("expected finish_reason=stop for done chunk")
	}
	if done.Choices[0].Delta.Content != "" {
		t.Errorf("done delta.content = %q, want empty", done.Choices[0].Delta.Content)
	}
}

func TestBuildModelList(t *testing.T) {
	ml := buildModelList()
	if ml.Object != "list" {
		t.Errorf("object = %q, want list", ml.Object)
	}
	if len(ml.Data) == 0 {
		t.Fatal("expected at least one model")
	}
	if ml.Data[0].ID != "workiq" {
		t.Errorf("model id = %q, want workiq", ml.Data[0].ID)
	}
}

func TestBuildOpenAIError(t *testing.T) {
	tests := []struct {
		status   int
		wantType string
	}{
		{400, "invalid_request_error"},
		{404, "not_found_error"},
		{500, "api_error"},
		{502, "api_error"},
	}
	for _, tt := range tests {
		e := buildOpenAIError(tt.status, "test")
		if e.Error.Type != tt.wantType {
			t.Errorf("status %d: type = %q, want %q", tt.status, e.Error.Type, tt.wantType)
		}
		if e.Error.Message != "test" {
			t.Errorf("status %d: message = %q, want test", tt.status, e.Error.Message)
		}
	}
}

func TestExtractTextContent(t *testing.T) {
	result := json.RawMessage(`{"content":[{"type":"text","text":"Hello"},{"type":"text","text":"World"}],"isError":false}`)
	text, isErr := extractTextContent(result)
	if isErr {
		t.Error("expected isErr=false")
	}
	if text != "Hello\nWorld" {
		t.Errorf("text = %q, want Hello\\nWorld", text)
	}
}

func TestExtractTextContentError(t *testing.T) {
	result := json.RawMessage(`{"content":[{"type":"text","text":"Something went wrong"}],"isError":true}`)
	text, isErr := extractTextContent(result)
	if !isErr {
		t.Error("expected isErr=true")
	}
	if !strings.Contains(text, "Error:") {
		t.Errorf("expected Error: prefix, got %q", text)
	}
}

// ── lastUserMessage tests ───────────────────────────────────────────

func TestLastUserMessage(t *testing.T) {
	tests := []struct {
		name     string
		messages []ChatMessage
		want     string
	}{
		{"single user", []ChatMessage{{Role: "user", Content: "hello"}}, "hello"},
		{"system+user", []ChatMessage{{Role: "system", Content: "sys"}, {Role: "user", Content: "hi"}}, "hi"},
		{"multi user picks last", []ChatMessage{{Role: "user", Content: "first"}, {Role: "user", Content: "second"}}, "second"},
		{"assistant after user", []ChatMessage{{Role: "user", Content: "q"}, {Role: "assistant", Content: "a"}, {Role: "user", Content: "followup"}}, "followup"},
		{"empty messages", []ChatMessage{}, ""},
		{"only system", []ChatMessage{{Role: "system", Content: "sys"}}, ""},
		{"whitespace trimmed", []ChatMessage{{Role: "user", Content: "  spaces  "}}, "spaces"},
		{"empty user content", []ChatMessage{{Role: "user", Content: "  "}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastUserMessage(tt.messages)
			if got != tt.want {
				t.Errorf("lastUserMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ── HTTP handler tests ──────────────────────────────────────────────

func TestHandleModels(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()
	handleModels(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var ml ModelList
	if err := json.NewDecoder(w.Body).Decode(&ml); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ml.Object != "list" || len(ml.Data) == 0 {
		t.Errorf("unexpected model list: %+v", ml)
	}
}

func TestHandleModelsWrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/models", nil)
	w := httptest.NewRecorder()
	handleModels(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/unknown", nil)
	w := httptest.NewRecorder()
	handleNotFound(w, req)

	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestChatCompletionsWrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	w := httptest.NewRecorder()
	handleChatCompletions(w, req, nil)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestChatCompletionsInvalidJSON(t *testing.T) {
	body := bytes.NewBufferString("{bad json")
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	w := httptest.NewRecorder()
	handleChatCompletions(w, req, nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestChatCompletionsNoUserMessage(t *testing.T) {
	payload := ChatCompletionRequest{
		Model:    "workiq",
		Messages: []ChatMessage{{Role: "system", Content: "you are helpful"}},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleChatCompletions(w, req, nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ── CORS tests ──────────────────────────────────────────────────────

func TestCORSPreflight(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := corsMiddleware(inner)

	req := httptest.NewRequest(http.MethodOptions, "/v1/chat/completions", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if v := w.Header().Get("Access-Control-Allow-Origin"); v != "http://localhost:3000" {
		t.Errorf("allow-origin = %q, want http://localhost:3000", v)
	}
	if v := w.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(v, "POST") {
		t.Errorf("allow-methods = %q, expected POST", v)
	}
}

func TestCORSNoOrigin(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := corsMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("expected inner handler to be called")
	}
	if v := w.Header().Get("Access-Control-Allow-Origin"); v != "" {
		t.Errorf("expected no CORS headers without origin, got %q", v)
	}
}

func TestCORSWithOrigin(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := corsMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if v := w.Header().Get("Access-Control-Allow-Origin"); v != "http://localhost:3000" {
		t.Errorf("allow-origin = %q, want http://localhost:3000", v)
	}
}

func TestCORSRejectsNonLocalOrigin(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := corsMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if v := w.Header().Get("Access-Control-Allow-Origin"); v != "" {
		t.Errorf("expected no allow-origin for non-local origin, got %q", v)
	}
}

// ── writeSSE test ───────────────────────────────────────────────────

func TestWriteSSE(t *testing.T) {
	w := httptest.NewRecorder()
	writeSSE(w, map[string]string{"hello": "world"})

	body := w.Body.String()
	if !strings.HasPrefix(body, "data: ") {
		t.Errorf("expected data: prefix, got %q", body)
	}
	if !strings.HasSuffix(body, "\n\n") {
		t.Errorf("expected trailing \\n\\n, got %q", body)
	}
}

// ── Logging middleware tests ────────────────────────────────────────

func TestStatusRecorder(t *testing.T) {
	w := httptest.NewRecorder()
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

	rec.WriteHeader(http.StatusNotFound)
	if rec.status != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.status, http.StatusNotFound)
	}
	if w.Code != http.StatusNotFound {
		t.Errorf("underlying status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	initAPILog()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	handler := loggingMiddleware(inner)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
}

func TestLoggingMiddlewareRequestIDs(t *testing.T) {
	initAPILog()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := loggingMiddleware(inner)

	// Fire two requests and verify the counter increments.
	before := reqCounter.Load()
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
	after := reqCounter.Load()
	if after-before != 3 {
		t.Errorf("reqCounter advanced by %d, want 3", after-before)
	}
}
