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

	worker, err := os.ReadFile(filepath.Join(directory, "web", "worker.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(worker), `"/__wahoo_ready"`) {
		t.Fatalf("worker readiness endpoint missing:\n%s", worker)
	}

	dockerfile, err := os.ReadFile(filepath.Join(directory, "Dockerfile.example"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"supervisor.mjs", "HEALTHCHECK", "USER wahoo", `"/sbin/tini"`} {
		if !strings.Contains(string(dockerfile), expected) {
			t.Fatalf("Dockerfile.example missing %q:\n%s", expected, dockerfile)
		}
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
	metadata, err := os.ReadFile(filepath.Join(directory, ".wahoo", "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	wantMetadata := "{\n  \"format_version\": 1,\n  \"framework_version\": \"" + FrameworkVersion + "\",\n  \"modules\": []\n}\n"
	if string(metadata) != wantMetadata {
		t.Fatalf("unexpected project metadata: %s", metadata)
	}
	if _, err := os.Stat(filepath.Join(directory, "internal", "config", "config.go")); err != nil {
		t.Fatalf("configuration template missing: %v", err)
	}
}

func TestCreateWithModules(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "acme")
	if err := Create(directory, "example.com/acme", "/framework", ModuleAuth, ModuleSSE, ModuleOpenAPI, ModuleJobs); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"auth.go", "sse.go", "openapi.go", "openapi.json", "jobs.go"} {
		if _, err := os.Stat(filepath.Join(directory, "app", name)); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(directory, "cmd", "worker", "main.go")); err != nil {
		t.Fatalf("worker command missing: %v", err)
	}
	routes, err := os.ReadFile(filepath.Join(directory, "app", "routes.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(routes), "registerAuth(runtime)") || !strings.Contains(string(routes), "registerSSE(runtime)") || !strings.Contains(string(routes), "registerOpenAPI(runtime)") {
		t.Fatalf("module registrations missing:\n%s", routes)
	}
	openAPI, err := os.ReadFile(filepath.Join(directory, "app", "openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"openapi": "3.1.0"`, `"url": "/api/v1"`} {
		if !strings.Contains(string(openAPI), expected) {
			t.Fatalf("OpenAPI document missing %q:\n%s", expected, openAPI)
		}
	}
}

func TestAdd(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "acme")
	if err := Create(directory, "example.com/acme", "/framework"); err != nil {
		t.Fatal(err)
	}
	if err := Add(directory, ModuleOpenAPI); err != nil {
		t.Fatal(err)
	}
	if err := Add(directory, ModuleOpenAPI); err != nil {
		t.Fatal(err)
	}
	routes, err := os.ReadFile(filepath.Join(directory, "app", "routes.go"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(routes), "registerOpenAPI(runtime)"); got != 1 {
		t.Fatalf("OpenAPI registration count = %d, want 1", got)
	}
	for _, name := range []string{"openapi.go", "openapi.json"} {
		if _, err := os.Stat(filepath.Join(directory, "app", name)); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
}

func TestAddRejectsManagedSymlink(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "acme")
	if err := Create(directory, "example.com/acme", "/framework"); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.RemoveAll(filepath.Join(directory, "app")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(directory, "app")); err != nil {
		t.Fatal(err)
	}
	if err := Add(directory, ModuleSSE); err == nil {
		t.Fatal("Add() returned nil for a managed symlink")
	}
}

func TestAddRejectsNestedManagedSymlink(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "acme")
	if err := Create(directory, "example.com/acme", "/framework"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(directory, "cmd")); err != nil {
		t.Fatal(err)
	}
	if err := Add(directory, ModuleJobs); err == nil {
		t.Fatal("Add() returned nil for a nested managed symlink")
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

func TestParseModulesIncludesGeneratedSeams(t *testing.T) {
	t.Parallel()
	modules, err := ParseModules([]string{"openapi,jobs", "uploads,mail,audit,webhooks,entitlements"})
	if err != nil {
		t.Fatal(err)
	}
	want := []Module{ModuleOpenAPI, ModuleJobs, ModuleUploads, ModuleMail, ModuleAudit, ModuleWebhooks, ModuleEntitlements}
	if len(modules) != len(want) {
		t.Fatalf("module count = %d, want %d", len(modules), len(want))
	}
	for index, module := range want {
		if modules[index] != module {
			t.Fatalf("module %d = %q, want %q", index, modules[index], module)
		}
	}
}

func TestUpgradeCheckAndApply(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "acme")
	if err := Create(directory, "example.com/acme", "/framework"); err != nil {
		t.Fatal(err)
	}
	goModPath := filepath.Join(directory, "go.mod")
	before, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatal(err)
	}

	result, err := Upgrade(directory, "v0.3.1", false)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.From != FrameworkVersion || result.To != "v0.3.1" {
		t.Fatalf("unexpected check result: %+v", result)
	}
	afterCheck, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterCheck) != string(before) {
		t.Fatal("upgrade check changed go.mod")
	}

	result, err = Upgrade(directory, "v0.3.1", true)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Fatal("upgrade apply did not report a change")
	}
	goMod, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goMod), "github.com/bjarneo/wahoo v0.3.1") {
		t.Fatalf("Wahoo dependency was not upgraded:\n%s", goMod)
	}
	metadata, err := os.ReadFile(filepath.Join(directory, ".wahoo", "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(metadata), `"framework_version": "v0.3.1"`) {
		t.Fatalf("metadata was not upgraded:\n%s", metadata)
	}
}

func TestReadLegacyModulesManifest(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "acme")
	if err := Create(directory, "example.com/acme", "/framework"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(directory, ".wahoo", "project.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, ".wahoo", "modules.json"), []byte("{\n  \"modules\": [\"jobs\"]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	project, err := readManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(project.Modules) != 1 || project.Modules[0] != ModuleJobs || project.FormatVersion != 0 {
		t.Fatalf("unexpected legacy manifest: %+v", project)
	}
}

func TestUpgradeRejectsManagedSymlink(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "acme")
	if err := Create(directory, "example.com/acme", "/framework"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(directory, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "go.mod"), filepath.Join(directory, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if _, err := Upgrade(directory, "v0.3.1", false); err == nil {
		t.Fatal("Upgrade() returned nil for a managed symlink")
	}
}

func TestProjectName(t *testing.T) {
	t.Parallel()
	if got := ProjectName("!!!"); got != "app" {
		t.Fatalf("ProjectName() = %q, want app", got)
	}
}
