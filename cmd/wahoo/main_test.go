package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bjarneo/wahoo/internal/scaffold"
)

func TestPromptModules(t *testing.T) {
	t.Parallel()
	modules, err := promptModules(strings.NewReader("yes\nno\ny\n"), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	want := []scaffold.Module{scaffold.ModuleAuth, scaffold.ModuleWebSocket}
	if len(modules) != len(want) {
		t.Fatalf("module count = %d, want %d", len(modules), len(want))
	}
	for i, module := range modules {
		if module != want[i] {
			t.Fatalf("module %d = %q, want %q", i, module, want[i])
		}
	}
}

func TestPromptModulesRejectsInvalidAnswer(t *testing.T) {
	t.Parallel()
	_, err := promptModules(strings.NewReader("maybe\n"), io.Discard)
	if err == nil {
		t.Fatal("promptModules() returned nil for an invalid answer")
	}
}

func TestUpgradeProjectCheckDoesNotWrite(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "acme")
	if err := scaffold.Create(directory, "example.com/acme", "/framework"); err != nil {
		t.Fatal(err)
	}
	goModPath := filepath.Join(directory, "go.mod")
	before, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := upgradeProject([]string{directory, "--check", "--to", "v0.3.1"}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("upgrade --check changed go.mod")
	}
}
