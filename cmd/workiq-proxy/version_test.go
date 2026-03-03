package main

import (
	"bytes"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

func TestInitRegistersDefaults(t *testing.T) {
	entries := getVersions()

	want := map[string]string{
		"workiq-proxy": Version,
		"mcp-protocol": MCPProtocol,
		"go":           runtime.Version(),
	}

	found := make(map[string]string)
	for _, e := range entries {
		found[e.Component] = e.Version
	}

	for comp, wantVer := range want {
		if got, ok := found[comp]; !ok {
			t.Errorf("missing component %q in version registry", comp)
		} else if got != wantVer {
			t.Errorf("version[%q] = %q, want %q", comp, got, wantVer)
		}
	}
}

func TestBuildDepsRegistered(t *testing.T) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		t.Skip("no build info available (go test without module?)")
	}

	found := make(map[string]string)
	for _, e := range getVersions() {
		found[e.Component] = e.Version
	}

	for _, dep := range bi.Deps {
		wantVer := dep.Version
		if dep.Replace != nil {
			wantVer = dep.Replace.Version
		}
		got, ok := found[dep.Path]
		if !ok {
			t.Errorf("dependency %q not in version registry", dep.Path)
		} else if got != wantVer {
			t.Errorf("version[%q] = %q, want %q", dep.Path, got, wantVer)
		}
	}
}

func TestRegisterVersion_Update(t *testing.T) {
	registerVersion("test-component", "1.0.0", "core")
	defer removeVersion("test-component")

	registerVersion("test-component", "2.0.0", "core")

	entries := getVersions()
	count := 0
	for _, e := range entries {
		if e.Component == "test-component" {
			count++
			if e.Version != "2.0.0" {
				t.Errorf("version = %q, want 2.0.0", e.Version)
			}
		}
	}
	if count != 1 {
		t.Errorf("found %d entries for test-component, want 1", count)
	}
}

func TestRemoveVersion(t *testing.T) {
	registerVersion("ephemeral", "0.0.1", "core")
	removeVersion("ephemeral")

	entries := getVersions()
	for _, e := range entries {
		if e.Component == "ephemeral" {
			t.Error("ephemeral should have been removed")
		}
	}
}

func TestRemoveVersion_NoOp(t *testing.T) {
	before := len(getVersions())
	removeVersion("does-not-exist")
	after := len(getVersions())
	if before != after {
		t.Errorf("registry size changed: %d → %d", before, after)
	}
}

func TestGetVersions_Snapshot(t *testing.T) {
	snap := getVersions()
	// Mutating the snapshot should not affect the registry.
	if len(snap) > 0 {
		snap[0].Version = "mutated"
	}
	fresh := getVersions()
	for _, e := range fresh {
		if e.Version == "mutated" {
			t.Error("getVersions returned a live reference, not a snapshot")
		}
	}
}

func TestRegisterVersion_OrderPreserved(t *testing.T) {
	registerVersion("zzz-last", "1.0", "core")
	registerVersion("aaa-first", "1.0", "core")
	defer removeVersion("zzz-last")
	defer removeVersion("aaa-first")

	entries := getVersions()
	zIdx, aIdx := -1, -1
	for i, e := range entries {
		if e.Component == "zzz-last" {
			zIdx = i
		}
		if e.Component == "aaa-first" {
			aIdx = i
		}
	}
	if zIdx >= aIdx {
		t.Errorf("registration order not preserved: zzz-last at %d, aaa-first at %d", zIdx, aIdx)
	}
}

func TestNpmDepsRegistered(t *testing.T) {
	found := make(map[string]string)
	for _, e := range getVersions() {
		if e.Group == "npm" {
			found[e.Component] = e.Version
		}
	}

	// npm_versions.go registers at least @microsoft/workiq.
	if _, ok := found["@microsoft/workiq"]; !ok {
		t.Error("@microsoft/workiq not in npm version registry")
	}
}

func TestGroupAssignment(t *testing.T) {
	for _, e := range getVersions() {
		if e.Group == "" {
			t.Errorf("component %q has empty group", e.Component)
		}
	}
}

func TestWriteVersionsOutput(t *testing.T) {
	var buf bytes.Buffer
	writeVersions(&buf)
	out := buf.String()

	// Core entries should always appear.
	if !strings.Contains(out, "workiq-proxy") {
		t.Error("output missing workiq-proxy")
	}
	if !strings.Contains(out, Version) {
		t.Errorf("output missing version %q", Version)
	}
	if !strings.Contains(out, runtime.Version()) {
		t.Error("output missing Go runtime version")
	}

	// Group headers should appear when those groups have entries.
	if !strings.Contains(out, "npm:") {
		t.Error("output missing npm group header")
	}
	if !strings.Contains(out, "Go modules:") {
		t.Error("output missing Go modules group header")
	}

	// npm must appear before Go modules.
	npmIdx := strings.Index(out, "npm:")
	goIdx := strings.Index(out, "Go modules:")
	if npmIdx >= goIdx {
		t.Errorf("npm group (%d) should appear before Go modules group (%d)", npmIdx, goIdx)
	}
}
