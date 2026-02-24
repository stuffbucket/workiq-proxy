package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// runAcceptEula starts the workiq MCP server and drives the accept_eula tool
// call over JSON-RPC stdio. This lets the user run "workiq-proxy accept-eula"
// from the terminal (or postinstall) without needing to know the MCP protocol.
func runAcceptEula(cmdParts []string) {
	cmdPath, err := exec.LookPath(cmdParts[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "workiq-proxy: cannot find %q: %v\n", cmdParts[0], err)
		os.Exit(1)
	}

	child := exec.Command(cmdPath, cmdParts[1:]...)
	child.Stderr = os.Stderr

	childIn, err := child.StdinPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "workiq-proxy: stdin pipe: %v\n", err)
		os.Exit(1)
	}
	childOut, err := child.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "workiq-proxy: stdout pipe: %v\n", err)
		os.Exit(1)
	}

	if err := child.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "workiq-proxy: failed to start %q: %v\n", strings.Join(cmdParts, " "), err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(childOut)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	send := func(msg interface{}) {
		data, _ := json.Marshal(msg)
		childIn.Write(data)
		childIn.Write([]byte("\n"))
	}

	waitResponse := func() (rpcMessage, bool) {
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var msg rpcMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				continue
			}
			return msg, true
		}
		return rpcMessage{}, false
	}

	// Step 1: initialize
	initID := json.RawMessage(`1`)
	send(rpcMessage{
		JSONRPC: "2.0",
		ID:      &initID,
		Method:  "initialize",
		Params: mustMarshal(map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    "workiq-proxy",
				"version": "0.1.0",
			},
		}),
	})

	if _, ok := waitResponse(); !ok {
		fmt.Fprintln(os.Stderr, "workiq-proxy: no response to initialize")
		os.Exit(1)
	}

	// Step 2: initialized notification
	send(rpcMessage{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	})

	// Step 3: call accept_eula
	callID := json.RawMessage(`2`)
	send(rpcMessage{
		JSONRPC: "2.0",
		ID:      &callID,
		Method:  "tools/call",
		Params: mustMarshal(toolCallParams{
			Name:      "accept_eula",
			Arguments: mustMarshal(map[string]string{"eulaUrl": "https://github.com/microsoft/work-iq-mcp"}),
		}),
	})

	resp, ok := waitResponse()
	if !ok {
		fmt.Fprintln(os.Stderr, "workiq-proxy: no response to accept_eula")
		os.Exit(1)
	}

	childIn.Close()
	child.Wait()

	// Print the result text for the user.
	if resp.Error != nil {
		fmt.Fprintf(os.Stderr, "workiq-proxy: EULA acceptance failed: %s\n", resp.Error.Message)
		os.Exit(1)
	}

	if resp.Result != nil {
		var result struct {
			Content []contentItem `json:"content"`
			IsError bool          `json:"isError"`
		}
		if err := json.Unmarshal(resp.Result, &result); err == nil {
			for _, c := range result.Content {
				if c.Type == "text" {
					fmt.Fprintln(os.Stderr, c.Text)
				}
			}
			if result.IsError {
				os.Exit(1)
			}
		}
	}

	// Suppress the child's stderr about broken pipe / signal.
	_ = child.Process.Signal(os.Kill)

	io.Copy(io.Discard, childOut)
}
