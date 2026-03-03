//go:build ignore

// gen_licenses collects license texts for every Go module and npm
// package used by the project, then writes THIRD_PARTY_DISCLOSURES.md.
//
// Usage: go run cmd/workiq-proxy/gen_licenses.go
//
// This is intentionally a standalone tool (not go:generate) because it
// is slow — it walks the module cache and node_modules. Run it via
// "make licenses" when preparing a release.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ── Problematic licenses ────────────────────────────────────────────

// Any SPDX identifier containing these substrings triggers a warning.
var problematicLicenses = []string{
	"GPL",      // GPL, LGPL, AGPL
	"SSPL",     // Server Side Public License
	"EUPL",     // European Union Public License
	"CPAL",     // Common Public Attribution License
	"OSL",      // Open Software License
	"AGPL",     // Affero GPL (overlap with GPL but explicit)
	"CC-BY-SA", // Creative Commons ShareAlike
	"CC-BY-NC", // Creative Commons NonCommercial
}

func isProblematic(license string) bool {
	up := strings.ToUpper(license)
	for _, p := range problematicLicenses {
		if strings.Contains(up, p) {
			return true
		}
	}
	return false
}

// ── Go modules ──────────────────────────────────────────────────────

type goModEntry struct {
	Path    string
	Version string
	Dir     string
}

func goModules() ([]goModEntry, error) {
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list -m -json all: %w", err)
	}

	var entries []goModEntry
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var m struct {
			Path    string `json:"Path"`
			Version string `json:"Version"`
			Dir     string `json:"Dir"`
			Main    bool   `json:"Main"`
		}
		if err := dec.Decode(&m); err != nil {
			return nil, err
		}
		if m.Main || m.Dir == "" {
			continue
		}
		entries = append(entries, goModEntry{Path: m.Path, Version: m.Version, Dir: m.Dir})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

// findLicenseFiles searches a directory for common license file names.
// It returns all matching files. Dual-licensed projects (e.g. LICENSE-MIT,
// LICENSE-BSD) will return multiple results.
func findLicenseFiles(dir string) ([]string, error) {
	candidates := []string{
		"LICENSE", "LICENSE.md", "LICENSE.txt",
		"LICENCE", "LICENCE.md", "LICENCE.txt",
		"COPYING", "COPYING.md", "COPYING.txt",
		"license", "license.md", "license.txt",
	}
	for _, name := range candidates {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return []string{p}, nil
		}
	}
	// Fall back to LICENSE-* glob (dual-license repos like go-udiff).
	matches, _ := filepath.Glob(filepath.Join(dir, "LICENSE-*"))
	if len(matches) > 0 {
		sort.Strings(matches)
		return matches, nil
	}
	return nil, fmt.Errorf("no license file in %s", dir)
}

// ── npm packages ────────────────────────────────────────────────────

type npmEntry struct {
	Name    string
	Version string
	License string // SPDX from package.json
	Dir     string // absolute path in node_modules
}

// npmPackages finds installed npm packages under the given roots.
func npmPackages(roots ...string) []npmEntry {
	var entries []npmEntry
	seen := make(map[string]bool)

	for _, root := range roots {
		nmDir := filepath.Join(root, "node_modules")
		walkNM(nmDir, &entries, seen)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries
}

func walkNM(nmDir string, entries *[]npmEntry, seen map[string]bool) {
	dirs, err := os.ReadDir(nmDir)
	if err != nil {
		return
	}
	for _, d := range dirs {
		if !d.IsDir() || d.Name() == ".package-lock.json" {
			continue
		}
		name := d.Name()
		pkgDir := filepath.Join(nmDir, name)

		// Scoped packages: @scope/pkg
		if strings.HasPrefix(name, "@") {
			subdirs, err := os.ReadDir(pkgDir)
			if err != nil {
				continue
			}
			for _, sd := range subdirs {
				if !sd.IsDir() {
					continue
				}
				scopedName := name + "/" + sd.Name()
				scopedDir := filepath.Join(pkgDir, sd.Name())
				addNpmEntry(scopedName, scopedDir, entries, seen)
			}
			continue
		}
		addNpmEntry(name, pkgDir, entries, seen)
	}
}

func addNpmEntry(name, dir string, entries *[]npmEntry, seen map[string]bool) {
	if seen[name] {
		return
	}
	pjPath := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(pjPath)
	if err != nil {
		return
	}
	var pj struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		License any    `json:"license"` // string or object
	}
	if json.Unmarshal(data, &pj) != nil {
		return
	}
	lic := ""
	switch v := pj.License.(type) {
	case string:
		lic = v
	case map[string]any:
		if t, ok := v["type"].(string); ok {
			lic = t
		}
	}
	seen[name] = true
	*entries = append(*entries, npmEntry{
		Name:    name,
		Version: pj.Version,
		License: lic,
		Dir:     dir,
	})
}

// ── Output ──────────────────────────────────────────────────────────

func main() {
	var warnings []string
	var buf strings.Builder

	buf.WriteString("# Third-Party Disclosures\n\n")
	buf.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339)))
	buf.WriteString("This file lists the licenses of all third-party dependencies.\n\n")

	// ── Go modules ──────────────────────────────────────────────
	goMods, err := goModules()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: could not list Go modules: %v\n", err)
	}

	if len(goMods) > 0 {
		buf.WriteString("---\n\n## Go Modules\n\n")
		for _, m := range goMods {
			licPaths, err := findLicenseFiles(m.Dir)
			if err != nil {
				warning := fmt.Sprintf("Go: %s@%s — no license file found (%s)", m.Path, m.Version, m.Dir)
				warnings = append(warnings, warning)
				buf.WriteString(fmt.Sprintf("### %s %s\n\n", m.Path, m.Version))
				buf.WriteString("*License file not found.*\n\n")
				continue
			}

			buf.WriteString(fmt.Sprintf("### %s %s\n\n", m.Path, m.Version))
			for _, licPath := range licPaths {
				licText, err := os.ReadFile(licPath)
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("Go: %s@%s — could not read %s", m.Path, m.Version, licPath))
					continue
				}
				if len(licPaths) > 1 {
					buf.WriteString(fmt.Sprintf("**%s**\n\n", filepath.Base(licPath)))
				}
				buf.WriteString("```\n")
				buf.WriteString(strings.TrimRight(string(licText), "\n") + "\n")
				buf.WriteString("```\n\n")
			}
		}
	}

	// ── npm packages ────────────────────────────────────────────
	npmPkgs := npmPackages(".", "npm")

	if len(npmPkgs) > 0 {
		buf.WriteString("---\n\n## npm Packages\n\n")
		for _, p := range npmPkgs {
			if isProblematic(p.License) {
				warnings = append(warnings, fmt.Sprintf("npm: %s@%s — problematic license: %s", p.Name, p.Version, p.License))
			}

			buf.WriteString(fmt.Sprintf("### %s %s\n\n", p.Name, p.Version))

			if p.License != "" {
				buf.WriteString(fmt.Sprintf("SPDX: `%s`\n\n", p.License))
			}

			licPaths, err := findLicenseFiles(p.Dir)
			if err != nil {
				if p.License != "" {
					buf.WriteString("*No license file; see SPDX identifier above.*\n\n")
				} else {
					warnings = append(warnings, fmt.Sprintf("npm: %s@%s — no license information found", p.Name, p.Version))
					buf.WriteString("*No license information found.*\n\n")
				}
				continue
			}

			for _, licPath := range licPaths {
				licText, err := os.ReadFile(licPath)
				if err != nil {
					continue
				}
				if len(licPaths) > 1 {
					buf.WriteString(fmt.Sprintf("**%s**\n\n", filepath.Base(licPath)))
				}
				buf.WriteString("```\n")
				buf.WriteString(strings.TrimRight(string(licText), "\n") + "\n")
				buf.WriteString("```\n\n")
			}
		}
	}

	// ── Write file ──────────────────────────────────────────────
	outPath := "THIRD_PARTY_DISCLOSURES.md"
	if err := os.WriteFile(outPath, []byte(buf.String()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: writing %s: %v\n", outPath, err)
		os.Exit(1)
	}

	goCount := len(goMods)
	npmCount := len(npmPkgs)
	fmt.Fprintf(os.Stderr, "wrote %s (%d Go modules, %d npm packages)\n", outPath, goCount, npmCount)

	// ── Warnings ────────────────────────────────────────────────
	if len(warnings) > 0 {
		fmt.Fprintln(os.Stderr)
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "⚠  %s\n", w)
		}
		fmt.Fprintf(os.Stderr, "\n%d warning(s) — review before release.\n", len(warnings))
	}
}
