package main

import (
	"encoding/json"
	"sync"
)

type rpcMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *rpcErrorObj     `json:"error,omitempty"`
}

type rpcErrorObj struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// initResult is only used in tests for assertion convenience.
type initResult struct {
	ProtocolVersion string                     `json:"protocolVersion,omitempty"`
	Capabilities    map[string]json.RawMessage `json:"capabilities,omitempty"`
	ServerInfo      json.RawMessage            `json:"serverInfo,omitempty"`
}

type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

var (
	pending   = make(map[string]string)
	pendingMu sync.Mutex
)

func idString(raw *json.RawMessage) string {
	if raw == nil {
		return ""
	}
	return string(*raw)
}

func trackRequest(id *json.RawMessage, method string) {
	if id == nil {
		return
	}
	pendingMu.Lock()
	pending[idString(id)] = method
	pendingMu.Unlock()
}

func popRequest(id *json.RawMessage) string {
	if id == nil {
		return ""
	}
	key := idString(id)
	pendingMu.Lock()
	method := pending[key]
	delete(pending, key)
	pendingMu.Unlock()
	return method
}

func peekRequest(id *json.RawMessage) string {
	if id == nil {
		return ""
	}
	pendingMu.Lock()
	method := pending[idString(id)]
	pendingMu.Unlock()
	return method
}

func makeResult(id *json.RawMessage, result interface{}) []byte {
	r, _ := json.Marshal(result)
	msg := rpcMessage{JSONRPC: "2.0", ID: id, Result: r}
	out, _ := json.Marshal(msg)
	return out
}

func mustMarshal(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
