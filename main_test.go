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

func TestRecentLabelsTellSameNamedFilesApart(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paths []string
		want  []string
	}{
		{
			name:  "distinct names keep their file name",
			paths: []string{"/work/notes/draft.md", "/work/notes/plan.md"},
			want:  []string{"draft.md", "plan.md"},
		},
		{
			name:  "one shared folder is not enough",
			paths: []string{"/work/notes/draft.md", "/personal/notes/draft.md"},
			want:  []string{"work/notes/draft.md", "personal/notes/draft.md"},
		},
		{
			name:  "only the colliding names grow",
			paths: []string{"/a/b/draft.md", "/a/b/plan.md", "/c/d/draft.md"},
			want:  []string{"b/draft.md", "plan.md", "d/draft.md"},
		},
		{
			name:  "three files with one name",
			paths: []string{"/one/draft.md", "/two/draft.md", "/three/draft.md"},
			want:  []string{"one/draft.md", "two/draft.md", "three/draft.md"},
		},
		{
			name:  "a name no folder can tell apart keeps its path",
			paths: []string{"/draft.md", "draft.md"},
			want:  []string{"/draft.md", "draft.md"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := recentLabels(tc.paths)
			if len(got) != len(tc.want) {
				t.Fatalf("labels = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("labels = %v, want %v", got, tc.want)
				}
			}
		})
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
