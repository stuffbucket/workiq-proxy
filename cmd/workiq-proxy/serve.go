package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
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
	dead      atomic.Bool
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
	if _, ok := b.waitResponse(context.Background()); !ok {
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

// responseTimeout is how long ask() waits for the MCP backend to
// respond before giving up. Work IQ queries can be slow, but 2 minutes
// is well beyond any reasonable response time.
const responseTimeout = 2 * time.Minute

func (b *mcpBackend) waitResponse(ctx context.Context) (rpcMessage, bool) {
	timer := time.NewTimer(responseTimeout)
	defer timer.Stop()
	for {
		select {
		case msg, ok := <-b.responses:
			if !ok {
				return msg, false
			}
			if msg.ID == nil && msg.Method != "" {
				continue // skip server notifications
			}
			return msg, true
		case <-timer.C:
			return rpcMessage{}, false
		case <-ctx.Done():
			return rpcMessage{}, false
		}
	}
}

type askResult struct {
	text string
	err  error
}

// ask sends a question to the MCP backend and returns the text result.
// It is safe for concurrent callers (serialized by mu). The context
// allows callers (e.g. HTTP handlers) to abort when clients disconnect,
// releasing the mutex so other requests aren't blocked.
func (b *mcpBackend) ask(ctx context.Context, question string) (string, error) {
	if b.dead.Load() {
		return "", fmt.Errorf("MCP backend is not running")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.send("tools/call", toolCallParams{
		Name:      "ask_work_iq",
		Arguments: mustMarshal(map[string]string{"question": question}),
	})

	resp, ok := b.waitResponse(ctx)
	if !ok {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
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
func (b *mcpBackend) askAsync(ctx context.Context, question string) <-chan askResult {
	ch := make(chan askResult, 1)
	go func() {
		text, err := b.ask(ctx, question)
		ch <- askResult{text, err}
	}()
	return ch
}

// ── HTTP server ─────────────────────────────────────────────────────

var apiLog *charmlog.Logger
var jsonLog *slog.Logger // non-nil when --json is active

func initAPILog() {
	if cfg.JSON {
		jsonLog = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	} else {
		apiLog = charmlog.NewWithOptions(os.Stderr, charmlog.Options{
			ReportTimestamp: true,
			TimeFormat:      time.TimeOnly,
			Prefix:          "api",
		})
	}
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
	go watchChild(child, backend)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		handleChatCompletions(w, r, backend)
	})
	mux.HandleFunc("/v1/models", handleModels)
	mux.HandleFunc("/v1/repl", handleRepl)
	mux.HandleFunc("/v1/repl/", handleRepl)
	mux.HandleFunc("/", handleNotFound)

	handler := loggingMiddleware(corsMiddleware(mux))

	ln := findListener(port)
	scheme := "http"
	if cfg.TLS {
		tlsCfg, tlsErr := loadOrGenerateTLSConfig()
		if tlsErr != nil {
			errServerStopped().detail(tlsErr).Die()
		}
		ln = tls.NewListener(ln, tlsCfg)
		scheme = "https"
	}
	addr := ln.Addr().String()
	if jsonLog != nil {
		jsonLog.Info("listening",
			"addr", addr,
			"tls", cfg.TLS,
			"routes", []string{
				"POST /v1/chat/completions",
				"GET /v1/models",
				"WS /v1/repl",
			})
	} else {
		fmt.Fprintf(os.Stderr, "workiq-proxy serving OpenAI-compatible API at %s://%s\n", scheme, addr)
		fmt.Fprintf(os.Stderr, "POST /v1/chat/completions   GET /v1/models   WS /v1/repl\n")
		if cfg.TLS {
			fmt.Fprintf(os.Stderr, "TLS enabled — visit %s://%s in your browser to trust the certificate.\n", scheme, addr)
		}
		fmt.Fprintf(os.Stderr, "Press Ctrl-C to stop.\n")
	}

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		<-sigCh
		if jsonLog != nil {
			jsonLog.Info("shutting-down")
		} else {
			fmt.Fprintln(os.Stderr, "\nworkiq-proxy: shutting down...")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		_ = backend.childIn.Close()
	}()

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
// unexpectedly, we mark the backend dead so in-flight and future
// requests get clean 502 errors instead of a hard process exit.
// The child dying is genuinely exceptional — all known workiq error
// modes (EULA, auth, token protection) return MCP-level errors and
// keep the process alive.
func watchChild(child *exec.Cmd, backend *mcpBackend) {
	err := child.Wait()
	backend.dead.Store(true)
	e := errBackendDied(err)
	e.Print()
	diskError("backend-died", "err", e.Detail)
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
	text, err := backend.ask(r.Context(), question)
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
	resultCh := backend.askAsync(r.Context(), question)
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
		strings.HasPrefix(origin, "https://localhost:") ||
		strings.HasPrefix(origin, "https://localhost/") ||
		strings.HasPrefix(origin, "https://127.0.0.1:") ||
		strings.HasPrefix(origin, "https://127.0.0.1/") ||
		origin == "http://localhost" ||
		origin == "http://127.0.0.1" ||
		origin == "https://localhost" ||
		origin == "https://127.0.0.1"
}

// isTrustedOrigin returns true for localhost and known first-party origins.
// The server only binds to 127.0.0.1, so the network interface is the real
// access boundary; origin checks are defense-in-depth against CSRF.
func isTrustedOrigin(origin string) bool {
	return isLocalOrigin(origin) ||
		origin == "https://stuffbucket.github.io"
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		if !isTrustedOrigin(origin) {
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
		if apiLog == nil && jsonLog == nil {
			next.ServeHTTP(w, r)
			return
		}

		id := reqCounter.Add(1)

		if jsonLog != nil {
			jsonLog.Info("http-request",
				"id", id,
				"method", r.Method,
				"path", r.URL.Path)
		} else {
			tag := fmt.Sprintf("#%d", id)
			apiLog.Info(tag+" → "+r.Method, "path", r.URL.Path)
		}
		diskInfo("http-request",
			"id", id,
			"method", r.Method,
			"path", r.URL.Path)

		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		duration := time.Since(start)

		status := rec.status

		if jsonLog != nil {
			lvl := slog.LevelInfo
			if status >= 500 {
				lvl = slog.LevelError
			} else if status >= 400 {
				lvl = slog.LevelWarn
			}
			jsonLog.Log(context.Background(), lvl, "http-response",
				"id", id,
				"status", status,
				"duration_ms", duration.Milliseconds())
		} else {
			tag := fmt.Sprintf("#%d", id)
			switch {
			case status >= 500:
				apiLog.Error(tag+" ←",
					"status", status,
					"duration", duration.Round(time.Millisecond))
			case status >= 400:
				apiLog.Warn(tag+" ←",
					"status", status,
					"duration", duration.Round(time.Millisecond))
			default:
				apiLog.Info(tag+" ←",
					"status", status,
					"duration", duration.Round(time.Millisecond))
			}
		}

		switch {
		case status >= 500:
			diskError("http-response",
				"id", id,
				"status", status,
				"duration_ms", duration.Milliseconds())
		case status >= 400:
			diskWarn("http-response",
				"id", id,
				"status", status,
				"duration_ms", duration.Milliseconds())
		default:
			diskInfo("http-response",
				"id", id,
				"status", status,
				"duration_ms", duration.Milliseconds())
		}
	})
}
