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
	if _, err := os.Stat(filepath.Join(directory, "Dockerfile.example")); err != nil {
		t.Fatalf("Dockerfile.example missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, ".dockerignore")); err != nil {
		t.Fatalf(".dockerignore missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "main.go.tmpl")); !os.IsNotExist(err) {
		t.Fatalf("template artifact exists: %v", err)
	}
	manifest, err := os.ReadFile(filepath.Join(directory, ".wahoo", "modules.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(manifest) != "{\n  \"modules\": []\n}\n" {
		t.Fatalf("unexpected module manifest: %s", manifest)
	}
}

func TestCreateWithModules(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "acme")
	if err := Create(directory, "example.com/acme", "/framework", ModuleAuth, ModuleSSE); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"auth.go", "sse.go"} {
		if _, err := os.Stat(filepath.Join(directory, "app", name)); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
	routes, err := os.ReadFile(filepath.Join(directory, "app", "routes.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(routes), "registerAuth(runtime)") || !strings.Contains(string(routes), "registerSSE(runtime)") {
		t.Fatalf("module registrations missing:\n%s", routes)
	}
}

func TestAdd(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "acme")
	if err := Create(directory, "example.com/acme", "/framework"); err != nil {
		t.Fatal(err)
	}
	if err := Add(directory, ModuleWebSocket); err != nil {
		t.Fatal(err)
	}
	if err := Add(directory, ModuleWebSocket); err != nil {
		t.Fatal(err)
	}
	routes, err := os.ReadFile(filepath.Join(directory, "app", "routes.go"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(routes), "registerWebSocket(runtime)"); got != 1 {
		t.Fatalf("WebSocket registration count = %d, want 1", got)
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
