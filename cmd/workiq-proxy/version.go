package main

//go:generate go run gen_npm_versions.go

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
)

// versionEntry pairs a component name with its version string and group.
type versionEntry struct {
	Component string
	Version   string
	Group     string // "core", "go", "npm"
}

var (
	versionsMu sync.Mutex
	versions   []versionEntry
)

func init() {
	registerVersion("workiq-proxy", Version, "core")
	registerVersion("mcp-protocol", MCPProtocol, "core")
	registerVersion("go", runtime.Version(), "core")
	registerBuildDeps()
}

// registerBuildDeps reads the binary's embedded build info and
// registers every direct dependency (not indirect/transitive).
func registerBuildDeps() {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, dep := range bi.Deps {
		if dep.Replace != nil {
			registerVersion(dep.Path, dep.Replace.Version, "go")
		} else {
			registerVersion(dep.Path, dep.Version, "go")
		}
	}
}

// registerVersion adds or updates a component in the version registry.
// The first registered component ("workiq-proxy") is the release version
// used for git tags and GitHub releases — it must stay in sync with the
// Version const in config.go.
func registerVersion(component, version, group string) {
	versionsMu.Lock()
	defer versionsMu.Unlock()
	for i, v := range versions {
		if v.Component == component {
			versions[i].Version = version
			versions[i].Group = group
			return
		}
	}
	versions = append(versions, versionEntry{component, version, group})
}

// removeVersion deletes a component from the version registry.
func removeVersion(component string) {
	versionsMu.Lock()
	defer versionsMu.Unlock()
	for i, v := range versions {
		if v.Component == component {
			versions = append(versions[:i], versions[i+1:]...)
			return
		}
	}
}

// getVersions returns a snapshot of all registered versions.
func getVersions() []versionEntry {
	versionsMu.Lock()
	defer versionsMu.Unlock()
	out := make([]versionEntry, len(versions))
	copy(out, versions)
	return out
}

// printVersions writes all registered component versions to stderr,
// grouped by category.
func printVersions() {
	writeVersions(os.Stderr)
}

// writeVersions writes grouped version output to w.
func writeVersions(w io.Writer) {
	entries := getVersions()

	groups := []struct {
		key   string
		label string
	}{
		{"core", ""},
		{"npm", "\nnpm:"},
		{"go", "\nGo modules:"},
	}

	for _, g := range groups {
		var rows [][2]string
		for _, e := range entries {
			if e.Group == g.key {
				rows = append(rows, [2]string{e.Component, e.Version})
			}
		}
		if len(rows) == 0 {
			continue
		}
		if g.label != "" {
			_, _ = fmt.Fprintln(w, g.label)
		}
		writeTable(w, rows)
	}
}
