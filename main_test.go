package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/appversion"
	"github.com/Bayern99/mark-go-go/internal/workspace"
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
	if err := adapter.EditableDocument(context.Background(), "/tmp", "doc-1"); err == nil {
		t.Fatal("expected error without caller window")
	}
	if _, err := adapter.CheckpointDocument(context.Background(), "doc-1", "/tmp/a.md"); err == nil {
		t.Fatal("expected error without caller window")
	}
}

func TestAppVersionMatchesTheShippedVersion(t *testing.T) {
	raw, err := os.ReadFile("mygo.json")
	if err != nil {
		t.Fatal(err)
	}
	var config struct{ Version string }
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	if appversion.Version != config.Version {
		t.Fatalf("appversion.Version = %q, mygo.json ships %q", appversion.Version, config.Version)
	}
}
