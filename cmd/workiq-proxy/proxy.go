package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

func runProxy(cmdParts []string, tty bool) {
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

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigCh {
			child.Process.Signal(sig)
		}
	}()

	var outMu sync.Mutex
	writeOut := func(data []byte) {
		outMu.Lock()
		os.Stdout.Write(data)
		os.Stdout.Write([]byte("\n"))
		if tty {
			fmt.Fprint(os.Stderr, ">>> ")
		}
		outMu.Unlock()
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		relayResponses(childOut, writeOut)
	}()

	if tty {
		fmt.Fprint(os.Stderr, ">>> ")
	}
	relayRequests(os.Stdin, childIn, writeOut, tty)

	childIn.Close()
	wg.Wait()

	exitCode := 1
	if err := child.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	} else {
		exitCode = 0
	}

	logMsg("---", mustMarshal(map[string]interface{}{"event": "child-exit", "code": exitCode}))
	os.Exit(exitCode)
}

func relayResponses(r io.Reader, writeOut func([]byte)) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" {
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal([]byte(trimmed), &msg); err != nil {
			logMsg("<<<", line)
			writeOut(line)
			continue
		}
		method := peekRequest(msg.ID)
		if method == "initialize" && msg.Result != nil {
			msg.Result = patchCapabilities(msg.Result)
		}
		if method == "tools/list" && msg.Result != nil {
			msg.Result = patchToolsList(msg.Result)
		}
		if method == "tools/call" && msg.Result != nil {
			msg.Result = enrichToolCallResult(msg.Result)
		}
		if msg.ID != nil {
			popRequest(msg.ID)
		}
		out, _ := json.Marshal(msg)
		logMsg("<<<", out)
		writeOut(out)
	}
}

func relayRequests(r io.Reader, childIn io.Writer, writeOut func([]byte), tty bool) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			if tty {
				fmt.Fprint(os.Stderr, ">>> ")
			}
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			childIn.Write([]byte(line + "\n"))
			continue
		}
		logMsg(">>>", []byte(line))
		if msg.Method == "prompts/list" && interceptPrompts {
			resp := makeResult(msg.ID, map[string]interface{}{"prompts": []interface{}{}})
			logMsg("<<<", resp)
			writeOut(resp)
			continue
		}
		if msg.Method == "resources/list" && interceptResources {
			resp := makeResult(msg.ID, map[string]interface{}{"resources": []interface{}{}})
			logMsg("<<<", resp)
			writeOut(resp)
			continue
		}
		if msg.Method == "resources/templates/list" && interceptResources {
			resp := makeResult(msg.ID, map[string]interface{}{"resourceTemplates": []interface{}{}})
			logMsg("<<<", resp)
			writeOut(resp)
			continue
		}
		if msg.Method == "tools/call" && msg.Params != nil {
			var params toolCallParams
			if err := json.Unmarshal(msg.Params, &params); err == nil {
				if syntheticToolNames[params.Name] {
					question := buildQuestion(params.Name, params.Arguments)
					newParams := toolCallParams{
						Name:      "ask_work_iq",
						Arguments: mustMarshal(map[string]string{"question": question}),
					}
					msg.Params = mustMarshal(newParams)
					trackRequest(msg.ID, "tools/call")
					rewritten, _ := json.Marshal(msg)
					logMsg(">>>", rewritten)
					childIn.Write(rewritten)
					childIn.Write([]byte("\n"))
					continue
				}
			}
		}
		if msg.ID != nil && msg.Method != "" {
			trackRequest(msg.ID, msg.Method)
		}
		childIn.Write([]byte(line + "\n"))
	}
}

func runPassthrough(cmdParts []string) {
	cmdPath, err := exec.LookPath(cmdParts[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "workiq-proxy: cannot find %q: %v\n", cmdParts[0], err)
		os.Exit(1)
	}
	child := exec.Command(cmdPath, cmdParts[1:]...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}
