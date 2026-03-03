package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	charmlog "github.com/charmbracelet/log"
)

// mcpBackend owns the child MCP process and serializes tool calls.
type mcpBackend struct {
	childIn   io.WriteCloser
	responses chan rpcMessage
	mu        sync.Mutex // serialize request/response pairs
	nextID    int
}

func newMCPBackend(cmdParts []string) (*mcpBackend, *exec.Cmd, error) {
	cmdPath, err := exec.LookPath(cmdParts[0])
	if err != nil {
		return nil, nil, fmt.Errorf("cannot find %q: %w", cmdParts[0], err)
	}

	child := exec.Command(cmdPath, cmdParts[1:]...)
	child.Stderr = os.Stderr

	childIn, err := child.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdin pipe: %w", err)
	}
	childOut, err := child.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := child.Start(); err != nil {
		return nil, nil, fmt.Errorf("start %q: %w", strings.Join(cmdParts, " "), err)
	}

	responses := make(chan rpcMessage, 16)
	go func() {
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

	b := &mcpBackend{childIn: childIn, responses: responses}

	// MCP handshake.
	b.send("initialize", map[string]interface{}{
		"protocolVersion": MCPProtocol,
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]string{"name": appName, "version": Version},
	})
	if _, ok := b.waitResponse(); !ok {
		_ = child.Process.Kill()
		return nil, nil, fmt.Errorf("server did not respond to initialize")
	}
	b.sendNotification("notifications/initialized")

	return b, child, nil
}

func (b *mcpBackend) send(method string, params interface{}) {
	b.nextID++
	id := json.RawMessage(fmt.Sprintf("%d", b.nextID))
	msg := rpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  mustMarshal(params),
	}
	data, _ := json.Marshal(msg)
	_, _ = b.childIn.Write(data)
	_, _ = b.childIn.Write([]byte("\n"))
}

func (b *mcpBackend) sendNotification(method string) {
	msg := rpcMessage{JSONRPC: "2.0", Method: method}
	data, _ := json.Marshal(msg)
	_, _ = b.childIn.Write(data)
	_, _ = b.childIn.Write([]byte("\n"))
}

func (b *mcpBackend) waitResponse() (rpcMessage, bool) {
	for {
		msg, ok := <-b.responses
		if !ok {
			return msg, false
		}
		if msg.ID == nil && msg.Method != "" {
			continue // skip server notifications
		}
		return msg, true
	}
}

type askResult struct {
	text string
	err  error
}

// ask sends a question to the MCP backend and returns the text result.
// It is safe for concurrent callers (serialized by mu).
func (b *mcpBackend) ask(question string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.send("tools/call", toolCallParams{
		Name:      "ask_work_iq",
		Arguments: mustMarshal(map[string]string{"question": question}),
	})

	resp, ok := b.waitResponse()
	if !ok {
		return "", fmt.Errorf("MCP backend disconnected")
	}
	if resp.Error != nil {
		return "", fmt.Errorf("%s (code %d)", resp.Error.Message, resp.Error.Code)
	}
	if resp.Result == nil {
		return "", fmt.Errorf("empty result from MCP backend")
	}

	resp.Result = enrichToolCallResult(resp.Result)
	text, _ := extractTextContent(resp.Result)
	return text, nil
}

// askAsync runs ask in a goroutine and returns a channel for the result.
func (b *mcpBackend) askAsync(question string) <-chan askResult {
	ch := make(chan askResult, 1)
	go func() {
		text, err := b.ask(question)
		ch <- askResult{text, err}
	}()
	return ch
}

// ── HTTP server ─────────────────────────────────────────────────────

var apiLog *charmlog.Logger

func initAPILog() {
	apiLog = charmlog.NewWithOptions(os.Stderr, charmlog.Options{
		ReportTimestamp: true,
		TimeFormat:      time.TimeOnly,
		Prefix:          "api",
	})
	initDiskLog()
}

func runServe(cmdParts []string, port int) {
	initAPILog()
	backend, child, err := newMCPBackend(cmdParts)
	if err != nil {
		errBackendStart().detail(err).Die()
	}
	defer func() {
		_ = backend.childIn.Close()
		// child.Wait() is owned by watchChild — do not call it twice.
	}()

	// Watch for unexpected backend death.
	go watchChild(child)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		handleChatCompletions(w, r, backend)
	})
	mux.HandleFunc("/v1/models", handleModels)
	mux.HandleFunc("/v1/repl", handleRepl)
	mux.HandleFunc("/", handleNotFound)

	handler := loggingMiddleware(corsMiddleware(mux))

	ln := findListener(port)
	addr := ln.Addr().String()
	fmt.Fprintf(os.Stderr, "workiq-proxy serving OpenAI-compatible API at http://%s\n", addr)
	fmt.Fprintf(os.Stderr, "POST /v1/chat/completions   GET /v1/models   WS /v1/repl\n")
	fmt.Fprintf(os.Stderr, "Press Ctrl-C to stop.\n")

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		errServerStopped().detail(err).Die()
	}
}

// findListener tries the requested port, then seeks upward until it
// finds an available one (up to 10 attempts). Returns the open listener
// so the caller can pass it directly to http.Server.Serve.
func findListener(port int) net.Listener {
	for i := 0; i < 10; i++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port+i)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			return ln
		}
	}
	// Fall back to OS-assigned port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		errPortUnavailable().Die()
	}
	return ln
}

// watchChild monitors the MCP backend child process. If it exits
// unexpectedly, we print a human-friendly message and shut down.
func watchChild(child *exec.Cmd) {
	err := child.Wait()
	errBackendDied(err).Die()
}

// ── Handlers ────────────────────────────────────────────────────────

func handleChatCompletions(w http.ResponseWriter, r *http.Request, backend *mcpBackend) {
	if r.Method != http.MethodPost {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "only POST is supported")
		return
	}

	// Cap request body at 1 MB to prevent memory exhaustion.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	question := lastUserMessage(req.Messages)
	if question == "" {
		writeOpenAIError(w, http.StatusBadRequest, "no user message found in messages array")
		return
	}

	if req.Stream {
		handleStream(w, r, backend, req.Model, question)
		return
	}

	// Non-streaming: blocks until MCP responds. No keep-alive bytes —
	// writing before we know success/failure would commit a 200 status
	// and prevent returning error codes.
	text, err := backend.ask(question)
	if err != nil {
		writeOpenAIError(w, http.StatusBadGateway, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(buildChatCompletion(req.Model, text))
}

func handleStream(w http.ResponseWriter, r *http.Request, backend *mcpBackend, model, question string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeOpenAIError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Send SSE headers immediately so the client knows we're alive.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	// Send SSE comments as heartbeats while the MCP backend is working.
	// Comments (lines starting with ':') are part of the SSE spec and
	// ignored by conforming EventSource clients, but visible in curl.
	resultCh := backend.askAsync(question)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var res askResult
	for {
		select {
		case res = <-resultCh:
			goto done
		case <-ticker.C:
			_, _ = fmt.Fprint(w, ": processing\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
done:
	id := nextChatID()

	if res.err != nil {
		errChunk := buildChatChunk(id, model, "Error: "+res.err.Error(), false)
		writeSSE(w, errChunk)
	} else {
		chunk := buildChatChunk(id, model, res.text, false)
		writeSSE(w, chunk)
	}
	flusher.Flush()

	finish := buildChatChunk(id, model, "", true)
	writeSSE(w, finish)
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "only GET is supported")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(buildModelList())
}

func handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeOpenAIError(w, http.StatusNotFound, "not found — try /v1/chat/completions or /v1/models")
}

// ── Helpers ─────────────────────────────────────────────────────────

func lastUserMessage(messages []ChatMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && strings.TrimSpace(messages[i].Content) != "" {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
}

func writeOpenAIError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(buildOpenAIError(status, message))
}

func writeSSE(w http.ResponseWriter, v interface{}) {
	data, _ := json.Marshal(v)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}

// isLocalOrigin returns true if the origin is a localhost URL.
func isLocalOrigin(origin string) bool {
	return strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "http://localhost/") ||
		strings.HasPrefix(origin, "http://127.0.0.1:") ||
		strings.HasPrefix(origin, "http://127.0.0.1/") ||
		origin == "http://localhost" ||
		origin == "http://127.0.0.1"
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		if !isLocalOrigin(origin) {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// statusRecorder wraps ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Flush() {
	if f, ok := sr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack exposes the underlying connection for WebSocket upgrades.
func (sr *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := sr.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("upstream ResponseWriter does not implement http.Hijacker")
}

var reqCounter atomic.Int64

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiLog == nil {
			next.ServeHTTP(w, r)
			return
		}

		id := reqCounter.Add(1)
		tag := fmt.Sprintf("#%d", id)

		apiLog.Info(tag+" → "+r.Method, "path", r.URL.Path)
		diskInfo("http-request",
			"id", id,
			"method", r.Method,
			"path", r.URL.Path)

		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		duration := time.Since(start)

		status := rec.status
		switch {
		case status >= 500:
			apiLog.Error(tag+" ←",
				"status", status,
				"duration", duration.Round(time.Millisecond))
			diskError("http-response",
				"id", id,
				"status", status,
				"duration_ms", duration.Milliseconds())
		case status >= 400:
			apiLog.Warn(tag+" ←",
				"status", status,
				"duration", duration.Round(time.Millisecond))
			diskWarn("http-response",
				"id", id,
				"status", status,
				"duration_ms", duration.Milliseconds())
		default:
			apiLog.Info(tag+" ←",
				"status", status,
				"duration", duration.Round(time.Millisecond))
			diskInfo("http-response",
				"id", id,
				"status", status,
				"duration_ms", duration.Milliseconds())
		}
	})
}
