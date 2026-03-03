package main

import (
	"bufio"
	"encoding/json"
	"fmt"
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
		errCLINotFound(cmdParts[0]).Die()
	}

	child := exec.Command(cmdPath, cmdParts[1:]...)
	child.Stderr = os.Stderr

	childIn, err := child.StdinPipe()
	if err != nil {
		errBackendStart().detail(err).Die()
	}
	childOut, err := child.StdoutPipe()
	if err != nil {
		errBackendStart().detail(err).Die()
	}

	if err := child.Start(); err != nil {
		errBackendStart().detail(err).Die()
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
			"protocolVersion": MCPProtocol,
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    appName,
				"version": Version,
			},
		}),
	})

	if _, ok := waitResponse(); !ok {
		errBackendNoResponse().Die()
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
		uerr("The backend stopped before EULA acceptance completed.").
			because("Your EULA acceptance was not saved.").
			fix("Try running the command again: workiq-proxy accept-eula").
			Die()
	}

	childIn.Close()
	_ = child.Wait()

	// Print the result text for the user.
	if resp.Error != nil {
		uerr("EULA acceptance failed.").
			because("You must accept the EULA before workiq-proxy can access Work IQ.").
			fix("Accept it at: https://github.com/microsoft/work-iq-mcp").
			detailf("%s", resp.Error.Message).
			Die()
	}

	if resp.Result != nil {
		var result struct {
			Content []contentItem `json:"content"`
			IsError bool          `json:"isError"`
		}
		if err := json.Unmarshal(resp.Result, &result); err == nil {
			for _, c := range result.Content {
				if c.Type == contentTypeText {
					fmt.Fprintln(os.Stderr, c.Text)
				}
			}
			if result.IsError {
				os.Exit(1)
			}
		}
	}

}
