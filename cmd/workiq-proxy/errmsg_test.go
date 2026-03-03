package main

import (
	"errors"
	"strings"
	"testing"
)

// ── Builder chain ───────────────────────────────────────────────────

func TestUerrSetsWhat(t *testing.T) {
	e := uerr("something broke")
	if e.What != "something broke" {
		t.Fatalf("What = %q, want %q", e.What, "something broke")
	}
	if e.Why != "" || e.Fix != "" || e.Detail != "" {
		t.Fatal("optional fields should be empty by default")
	}
}

func TestBuilderChainIsImmutable(t *testing.T) {
	base := uerr("base")
	withWhy := base.because("reason")
	withFix := base.fix("do this")

	if base.Why != "" {
		t.Fatal("because() mutated the original")
	}
	if base.Fix != "" {
		t.Fatal("fix() mutated the original")
	}
	if withWhy.Why != "reason" {
		t.Fatalf("because() did not set Why: %q", withWhy.Why)
	}
	if withFix.Fix != "do this" {
		t.Fatalf("fix() did not set Fix: %q", withFix.Fix)
	}
}

func TestDetailFromError(t *testing.T) {
	e := uerr("fail").detail(errors.New("disk full"))
	if e.Detail != "disk full" {
		t.Fatalf("Detail = %q, want %q", e.Detail, "disk full")
	}
}

func TestDetailf(t *testing.T) {
	e := uerr("fail").detailf("code %d", 42)
	if e.Detail != "code 42" {
		t.Fatalf("Detail = %q, want %q", e.Detail, "code 42")
	}
}

// ── ANSI delivery ───────────────────────────────────────────────────

func TestANSI_AllFields(t *testing.T) {
	out := uerr("Something broke.").
		because("You can't do X.").
		fix("Try Y.").
		detail(errors.New("ENOENT")).
		ANSI()

	assertContains(t, out, "Something broke.")
	assertContains(t, out, "You can't do X.")
	assertContains(t, out, "Detail: ENOENT")
	assertContains(t, out, "Try Y.")

	// Bold red on the What line.
	if !strings.Contains(out, "\x1b[1;31m") {
		t.Error("missing bold-red escape for What")
	}
	// Reset after What.
	if !strings.Contains(out, "\x1b[0m") {
		t.Error("missing ANSI reset")
	}
	// Raw terminal line endings.
	if !strings.Contains(out, "\r\n") {
		t.Error("ANSI output should use \\r\\n line endings")
	}
}

func TestANSI_WhatOnly(t *testing.T) {
	out := uerr("Oops.").ANSI()

	assertContains(t, out, "Oops.")
	// Should not contain indented lines for missing fields.
	if strings.Contains(out, "Detail:") {
		t.Error("empty Detail should not appear")
	}
	lines := strings.Split(out, "\r\n")
	// Expect exactly: [whatLine, ""] (trailing \r\n splits into empty last element).
	nonEmpty := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty++
		}
	}
	if nonEmpty != 1 {
		t.Errorf("expected 1 non-empty line, got %d: %q", nonEmpty, out)
	}
}

func TestANSI_NoDetailField(t *testing.T) {
	out := uerr("Broke.").because("matters").fix("do this").ANSI()
	if strings.Contains(out, "Detail:") {
		t.Error("Detail line should be absent when Detail is empty")
	}
	assertContains(t, out, "matters")
	assertContains(t, out, "do this")
}

// ── Pre-built constructors ──────────────────────────────────────────

func TestErrCLINotFound(t *testing.T) {
	e := errCLINotFound("workiq")
	assertContains(t, e.What, "workiq")
	if e.Why == "" || e.Fix == "" {
		t.Fatal("pre-built constructor should populate Why and Fix")
	}
}

func TestErrBackendStart(t *testing.T) {
	e := errBackendStart()
	if e.What == "" || e.Why == "" || e.Fix == "" {
		t.Fatal("errBackendStart should populate What, Why, Fix")
	}
}

func TestErrBackendNoResponse(t *testing.T) {
	e := errBackendNoResponse()
	if e.What == "" || e.Why == "" || e.Fix == "" {
		t.Fatal("errBackendNoResponse should populate What, Why, Fix")
	}
}

func TestErrBackendDied_WithError(t *testing.T) {
	e := errBackendDied(errors.New("signal: killed"))
	assertContains(t, e.Detail, "signal: killed")
	if e.Fix == "" {
		t.Fatal("errBackendDied should have a Fix")
	}
}

func TestErrBackendDied_NilError(t *testing.T) {
	e := errBackendDied(nil)
	if e.Detail != "" {
		t.Fatalf("nil error should leave Detail empty, got %q", e.Detail)
	}
}

func TestErrPortUnavailable(t *testing.T) {
	e := errPortUnavailable()
	if e.What == "" || e.Why == "" || e.Fix == "" {
		t.Fatal("errPortUnavailable should populate all fields")
	}
}

func TestErrServerStopped(t *testing.T) {
	e := errServerStopped()
	if e.What == "" || e.Why == "" || e.Fix == "" {
		t.Fatal("errServerStopped should populate all fields")
	}
}

// ── Detail chaining on pre-built errors ─────────────────────────────

func TestPrebuiltDetailChaining(t *testing.T) {
	e := errBackendStart().detail(errors.New("exec: not found"))
	assertContains(t, e.Detail, "exec: not found")
	// Original fields should survive the chain.
	if e.What == "" || e.Why == "" || e.Fix == "" {
		t.Fatal("detail() should not clear other fields")
	}
}

func TestPrebuiltANSIRendering(t *testing.T) {
	out := errCLINotFound("npx").detail(errors.New("ENOENT")).ANSI()
	assertContains(t, out, "npx")
	assertContains(t, out, "Detail: ENOENT")
	assertContains(t, out, "\x1b[1;31m") // bold red
}

// ── Helpers ─────────────────────────────────────────────────────────

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected %q to contain %q", haystack, needle)
	}
}
