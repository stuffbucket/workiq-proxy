package main

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty/v2"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "" || isLocalOrigin(origin)
	},
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

// handleRepl upgrades an HTTP request to a WebSocket, spawns a new
// workiq-proxy process in a pseudo-terminal, and bridges pty I/O to
// the WebSocket using binary frames.
func handleRepl(w http.ResponseWriter, r *http.Request) {
	if apiLog != nil {
		apiLog.Info("repl: upgrade request",
			"method", r.Method,
			"upgrade", r.Header.Get("Upgrade"),
			"connection", r.Header.Get("Connection"),
			"origin", r.Header.Get("Origin"))
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if apiLog != nil {
			apiLog.Error("repl: websocket upgrade failed", "err", err)
		}
		return
	}
	defer conn.Close()

	// Find our own binary so we can re-exec in interactive mode.
	self, err := os.Executable()
	if err != nil {
		writeWSError(conn,
			uerr("Cannot locate workiq-proxy binary.").
				because("The WebSocket REPL cannot start without re-execing the binary.").
				fix("Ensure workiq-proxy is on your PATH.").
				detail(err).ANSI())
		return
	}

	// Build command: ourselves with the same config, no subcommand.
	// When run in a pty this triggers the REPL path in main().
	args := []string{self}
	if cfg.WorkiqCmd != DefaultWorkiqCmd {
		args = append(args, "--workiq-cmd", cfg.WorkiqCmd)
	}
	if cfg.NoLog {
		args = append(args, "--no-log")
	}
	if cfg.LogFile != "" {
		args = append(args, "--log-file", cfg.LogFile)
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = os.Environ()

	// Wait for the first resize message so the pty starts at the right size.
	// The xterm.js client sends {"cols":N,"rows":N} immediately on connect.
	initSize := &pty.Winsize{Rows: 24, Cols: 80}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, data, err := conn.ReadMessage(); err == nil && isResizeMessage(data) {
		cols, rows := parseResize(data)
		if cols > 0 && rows > 0 {
			initSize = &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}
		}
	}
	conn.SetReadDeadline(time.Time{}) // clear deadline

	ptmx, err := pty.StartWithSize(cmd, initSize)
	if err != nil {
		writeWSError(conn,
			errBackendStart().detail(err).ANSI())
		return
	}
	defer func() {
		_ = ptmx.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	if apiLog != nil {
		apiLog.Info("repl: session started", "pid", cmd.Process.Pid)
	}

	var wg sync.WaitGroup

	// pty → WebSocket: read pty output and send as binary frames.
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				if wErr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); wErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// WebSocket → pty: read WebSocket messages and write to pty.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}

			switch msgType {
			case websocket.BinaryMessage, websocket.TextMessage:
				// Check for resize message: JSON {"cols":N,"rows":N}
				if isResizeMessage(data) {
					cols, rows := parseResize(data)
					if cols > 0 && rows > 0 {
						_ = pty.Setsize(ptmx, &pty.Winsize{
							Rows: uint16(rows),
							Cols: uint16(cols),
						})
					}
					continue
				}
				if _, err := ptmx.Write(data); err != nil {
					return
				}
			}
		}
	}()

	wg.Wait()

	if apiLog != nil {
		apiLog.Info("repl: session ended", "pid", cmd.Process.Pid)
	}
}

func writeWSError(conn *websocket.Conn, ansi string) {
	_ = conn.WriteMessage(websocket.TextMessage, []byte(ansi))
	time.Sleep(100 * time.Millisecond)
	// Close messages have a 125-byte limit; send a short reason.
	_ = conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "error"))
}

// resizeMessage is the JSON structure sent by xterm.js to resize the pty.
type resizeMessage struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// isResizeMessage checks if data looks like a JSON resize command.
func isResizeMessage(data []byte) bool {
	if len(data) < 2 || data[0] != '{' {
		return false
	}
	var msg resizeMessage
	return json.Unmarshal(data, &msg) == nil && msg.Cols > 0 && msg.Rows > 0
}

// parseResize extracts cols and rows from a JSON resize message.
func parseResize(data []byte) (cols, rows int) {
	var msg resizeMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return 0, 0
	}
	return msg.Cols, msg.Rows
}
