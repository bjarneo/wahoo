package main

import (
	"io"
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
