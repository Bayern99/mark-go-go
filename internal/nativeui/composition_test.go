package nativeui

import (
	"context"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func TestTableInputSurvivesPublishFailure(t *testing.T) {
	for _, action := range []string{"save", "saveAs", "close", "markdown", "genui", "publish"} {
		t.Run(action, func(t *testing.T) {
			const src = "| Name | Tag |\n| --- | --- |\n| cat | x |"
			d := nativeeditor.New(src)
			host := workspace.NewService()
			t.Cleanup(host.CloseAgentCache)
			w := &Window{
				host: host, ctx: context.Background(),
				snapshot:  workspace.WorkspaceSnapshot{ActiveID: "missing"},
				documents: map[string]*openDocument{"missing": {source: d}},
			}
			tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 420)
			tt.Frame()
			if err := tt.Click("Table row 2 column 1"); err != nil {
				t.Fatal(err)
			}
			tt.Command("selectAll")
			tt.Compose("ni", 2)
			tt.Frame()
			tt.Compose("你", 1)
			tt.Frame()
			switch action {
			case "markdown", "genui":
				w.SetMode(action)
			case "publish":
				if err := w.Publish(); err == nil {
					t.Fatal("publish to missing host document succeeded")
				}
			default:
				w.command(workspace.NativeCommand{ID: "missing", Command: action})
			}
			if action != "publish" && w.failure == "" {
				t.Fatal("missing host document did not surface a failure")
			}
			want := strings.Replace(src, "cat", "你", 1)
			if d.Source() != want {
				t.Fatalf("source after %s = %q, want %q", action, d.Source(), want)
			}
			if !d.Undo() || d.Source() != src || d.CanUndo() {
				t.Fatalf("undo = %q, remaining undo=%v", d.Source(), d.CanUndo())
			}
		})
	}
}
