package main

import (
	"bytes"
	_ "embed"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"text/template"
)

//go:generate go test -run TestGenerateDocs -args -update-docs

//go:embed cli-reference.md.tmpl
var cliTmplSrc string

//go:embed error-reference.md.tmpl
var errorTmplSrc string

//go:embed versions.md.tmpl
var versionsTmplSrc string

var updateDocs = flag.Bool("update-docs", false, "regenerate reference docs from source data")

// repoRoot returns the workspace root (two levels up from this test file).
func repoRoot() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// ── Template data types ─────────────────────────────────────────────

type flagDoc struct {
	Name    string
	Default string
	Usage   string
}

type slashDoc struct {
	Name string
	Desc string
}

type cliData struct {
	Commands [][2]string
	Friendly [][2]string
	Flags    []flagDoc
	Slash    []slashDoc
}

type errorData struct {
	Errors []userError
}

type versionsData struct {
	Core []versionEntry
	Npm  []versionEntry
	Go   []versionEntry
}

// ── Generator ───────────────────────────────────────────────────────

func collectFlags() []flagDoc {
	var flags []flagDoc
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		if strings.HasPrefix(f.Name, "test.") || f.Name == "update-docs" {
			return
		}
		def := f.DefValue
		if def == "" {
			def = "*(none)*"
		} else {
			def = "`" + def + "`"
		}
		flags = append(flags, flagDoc{Name: f.Name, Default: def, Usage: f.Usage})
	})
	return flags
}

func collectSlash() []slashDoc {
	var cmds []slashDoc
	for _, sc := range slashCommands {
		name := sc.name
		if sc.argHint != "" {
			name += " " + sc.argHint
		}
		cmds = append(cmds, slashDoc{Name: name, Desc: sc.desc})
	}
	return cmds
}

func generateCLI() []byte {
	tmpl := template.Must(template.New("cli").Parse(cliTmplSrc))
	var buf bytes.Buffer
	data := cliData{
		Commands: allCLICommands,
		Friendly: friendlyCLICommands,
		Flags:    collectFlags(),
		Slash:    collectSlash(),
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		panic("cli template: " + err.Error())
	}
	return buf.Bytes()
}

func generateErrors() []byte {
	tmpl := template.Must(template.New("errors").Parse(errorTmplSrc))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, errorData{Errors: errorCatalog}); err != nil {
		panic("error template: " + err.Error())
	}
	return buf.Bytes()
}

func generateVersions() []byte {
	tmpl := template.Must(template.New("versions").Parse(versionsTmplSrc))
	entries := getVersions()
	var data versionsData
	for _, e := range entries {
		switch e.Group {
		case "core":
			data.Core = append(data.Core, e)
		case "npm":
			data.Npm = append(data.Npm, e)
		case "go":
			data.Go = append(data.Go, e)
		}
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		panic("versions template: " + err.Error())
	}
	return buf.Bytes()
}

// ── Test: generate and write files ──────────────────────────────────

func TestGenerateDocs(t *testing.T) {
	if !*updateDocs {
		t.Skip("use -args -update-docs to regenerate reference docs")
	}

	outDir := filepath.Join(repoRoot(), "docs", "src", "content", "docs", "reference")

	files := map[string][]byte{
		"cli-reference.md":   generateCLI(),
		"error-reference.md": generateErrors(),
		"versions.md":        generateVersions(),
	}

	for name, content := range files {
		path := filepath.Join(outDir, name)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
		t.Logf("wrote %s (%d bytes)", path, len(content))
	}
}

// ── Test: verify generated files are fresh ──────────────────────────

func TestGeneratedDocsAreFresh(t *testing.T) {
	outDir := filepath.Join(repoRoot(), "docs", "src", "content", "docs", "reference")

	files := map[string][]byte{
		"cli-reference.md":   generateCLI(),
		"error-reference.md": generateErrors(),
		"versions.md":        generateVersions(),
	}

	for name, want := range files {
		path := filepath.Join(outDir, name)
		got, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			t.Errorf("%s does not exist — run 'make docs' to generate it", name)
			continue
		}
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is stale — run 'make docs' to regenerate", name)
			// Show first difference for debugging.
			gotLines := strings.Split(string(got), "\n")
			wantLines := strings.Split(string(want), "\n")
			for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
				g, w := "", ""
				if i < len(gotLines) {
					g = gotLines[i]
				}
				if i < len(wantLines) {
					w = wantLines[i]
				}
				if g != w {
					t.Logf("  first diff at line %d:\n    got:  %q\n    want: %q", i+1, g, w)
					break
				}
			}
		}
	}
}

// ── Test: error catalog covers all constructors ─────────────────────

func TestErrorCatalogCompleteness(t *testing.T) {
	// These are all the pre-built error constructor names.
	// If you add a new errFoo() function in errmsg.go, add it to
	// errorCatalog and this list.
	wantMessages := []string{
		"Could not find the Work IQ CLI",
		"Could not start the backend process",
		"The backend did not respond during startup",
		"The backend process stopped unexpectedly",
		"Could not find an available port",
		"The server stopped unexpectedly",
	}

	catalogMessages := make(map[string]bool)
	for _, e := range errorCatalog {
		catalogMessages[e.What] = true
	}

	for _, msg := range wantMessages {
		found := false
		for cat := range catalogMessages {
			if strings.Contains(cat, msg) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("errorCatalog is missing an entry containing %q", msg)
		}
	}
}
