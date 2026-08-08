package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreate(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "My SaaS!")
	if err := Create(directory, "example.com/acme", "/framework"); err != nil {
		t.Fatal(err)
	}

	goMod, err := os.ReadFile(filepath.Join(directory, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goMod), "module example.com/acme") || !strings.Contains(string(goMod), "replace github.com/bjarneo/wahoo => /framework") {
		t.Fatalf("unexpected go.mod:\n%s", goMod)
	}

	packageJSON, err := os.ReadFile(filepath.Join(directory, "web", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(packageJSON), `"name": "my-saas-web"`) {
		t.Fatalf("project name was not normalized:\n%s", packageJSON)
	}

	if _, err := os.Stat(filepath.Join(directory, "main.go")); err != nil {
		t.Fatalf("main.go missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "main.go.tmpl")); !os.IsNotExist(err) {
		t.Fatalf("template artifact exists: %v", err)
	}
}

func TestCreateRejectsNonEmptyDirectory(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "existing"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Create(directory, "example.com/acme", ""); err == nil {
		t.Fatal("Create() returned nil for a non-empty directory")
	}
}

func TestProjectName(t *testing.T) {
	t.Parallel()
	if got := ProjectName("!!!"); got != "app" {
		t.Fatalf("ProjectName() = %q, want app", got)
	}
}
