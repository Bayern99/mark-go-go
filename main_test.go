package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

func TestModulePathAndGeneratedTypeScriptBindings(t *testing.T) {
	mod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(mod), "module github.com/Bayern99/mark-go-go\n") {
		t.Fatalf("go.mod does not declare module github.com/Bayern99/mark-go-go:\n%s", mod)
	}

	ts, err := os.ReadFile(filepath.Join("web", "src", "host", "generated", "mygo.ts"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(ts)
	for _, want := range []string{
		`export type FolderState = "open" | "collapsed" | "empty";`,
		`export type IndexStatus = "idle" | "scanning" | "indexing" | "ready" | "error";`,
		`/** Events sent by the Go side. Subscribe with ` + "`events.name.on(listener)`" + `. */`,
		`export const WorkspaceHost = {`,
		`export const ResearchHost = {`,
		`export const ResearchConsoleHost = {`,
		`export const CheckpointHost = {`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("generated mygo.ts missing %q", want)
		}
	}
}

func TestCompositionRootBindAsSurfacesAreNarrow(t *testing.T) {
	cases := []struct {
		name string
		typ  reflect.Type
		want int
	}{
		{"WorkspaceHost", reflect.TypeFor[*workspace.Host](), 34},
		{"ResearchHost", reflect.TypeFor[*feynman.ResearchHost](), 17},
		{"ResearchConsoleHost", reflect.TypeFor[*feynman.ResearchConsoleHost](), 5},
		{"CheckpointHost", reflect.TypeFor[*review.CheckpointHost](), 3},
	}
	for _, tc := range cases {
		if got := tc.typ.NumMethod(); got != tc.want {
			t.Errorf("%s exported methods = %d, want %d", tc.name, got, tc.want)
		}
	}

	svc := workspace.NewService()
	adapter := workspaceAdapter{svc: svc}
	if _, err := adapter.WindowWorkspace(context.Background(), "/tmp"); err == nil {
		t.Fatal("expected error without caller window")
	}
	if err := adapter.EditableDocument(context.Background(), "/tmp", "doc-1"); err == nil {
		t.Fatal("expected error without caller window")
	}
	if _, err := adapter.CheckpointDocument(context.Background(), "doc-1", "/tmp/a.md"); err == nil {
		t.Fatal("expected error without caller window")
	}
}
