package main

import (
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

//go:embed web.html
var webHTML []byte

// runWeb starts the serve backend, serves a full-viewport xterm web UI,
// and opens the browser. API endpoints (/v1/chat/completions, /v1/models,
// /v1/repl) remain available alongside the web page.
func runWeb(cmdParts []string, port int) {
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
	mux.HandleFunc("/", handleWebUI)

	handler := loggingMiddleware(corsMiddleware(mux))

	// Bind the port first, then open the browser — the listener is
	// already accepting connections at this point.
	ln := findListener(port)
	url := "http://" + ln.Addr().String()

	fmt.Fprintf(os.Stderr, "workiq-proxy web UI at %s\n", url)
	fmt.Fprintf(os.Stderr, "Press Ctrl-C to stop.\n")

	openBrowser(url)

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		errServerStopped().detail(err).Die()
	}
}

func handleWebUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		handleNotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(webHTML)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	_ = cmd.Start()
}
