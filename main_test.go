package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo"
)

func TestModulePath(t *testing.T) {
	mod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(mod), "module github.com/Bayern99/mark-go-go\n") {
		t.Fatalf("go.mod does not declare module github.com/Bayern99/mark-go-go:\n%s", mod)
	}
}

func TestCompositionRootServicesRequireWindowContext(t *testing.T) {
	svc := workspace.NewService()
	adapter := workspaceAdapter{svc: svc}
	if _, err := adapter.WindowWorkspace(context.Background(), "/tmp"); err == nil {
		t.Fatal("expected error without caller window")
	}
	if err := adapter.EditableDocument(context.Background(), "doc-1", "/tmp/a.md"); err == nil {
		t.Fatal("expected error without caller window")
	}
	if _, err := adapter.CheckpointDocument(context.Background(), "doc-1", "/tmp/a.md"); err == nil {
		t.Fatal("expected error without caller window")
	}
}

func TestBuildMenuUsesSettingsCenterDeepLinks(t *testing.T) {
	menu := buildMenu(workspace.NewService())
	labels := map[string]bool{}
	var walk func([]*mygo.MenuItem)
	walk = func(items []*mygo.MenuItem) {
		for _, item := range items {
			labels[item.Label] = true
			if item.Label == "Space" || item.Label == "Appearance" {
				t.Errorf("View menu still has a %s editor", item.Label)
			}
			walk(item.Submenu)
		}
	}
	walk(menu.Items())
	for _, label := range []string{"Settings…", "Appearance…", "Checkpoints…", "Help", "Keyboard Shortcuts"} {
		if !labels[label] {
			t.Errorf("menu is missing %q", label)
		}
	}
}
