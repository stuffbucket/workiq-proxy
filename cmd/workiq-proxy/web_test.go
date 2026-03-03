package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleWebUI_Root(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handleWebUI(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html*", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "xterm") {
		t.Error("expected page to contain 'xterm'")
	}
	if !strings.Contains(body, "/v1/repl") {
		t.Error("expected page to reference /v1/repl WebSocket endpoint")
	}
}

func TestHandleWebUI_NonRoot(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/foo", nil)
	rec := httptest.NewRecorder()

	handleWebUI(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestFindListener(t *testing.T) {
	ln := findListener(0)
	defer ln.Close()

	addr := ln.Addr().String()
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Errorf("addr = %q, want 127.0.0.1:*", addr)
	}
}

func TestFindListenerSkipsOccupied(t *testing.T) {
	// Occupy a port, then verify findListener returns a different one.
	first := findListener(19000)
	defer first.Close()

	second := findListener(19000)
	defer second.Close()

	if first.Addr().String() == second.Addr().String() {
		t.Error("expected different port when first is occupied")
	}
}

func TestWebHTMLEmbed(t *testing.T) {
	if len(webHTML) == 0 {
		t.Fatal("webHTML embed is empty")
	}
	html := string(webHTML)
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("expected HTML doctype")
	}
	if !strings.Contains(html, "onTitleChange") {
		t.Error("expected onTitleChange handler for /title support")
	}
}

func TestWebHTMLDisconnectMessage(t *testing.T) {
	html := string(webHTML)
	if !strings.Contains(html, "Connection lost") {
		t.Error("expected disconnect message in web HTML")
	}
	if !strings.Contains(html, "workiq-proxy web") {
		t.Error("expected recovery instructions in web HTML")
	}
}
