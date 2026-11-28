package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/appversion"
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
		{
			name:  "filesystem root preserves a visible label",
			paths: []string{"/", "/work/notes/draft.md"},
			want:  []string{"/", "draft.md"},
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

func TestViewMenuUsesCommandEForPreviewMarkdownToggle(t *testing.T) {
	menu := buildMenu(workspace.NewService(), true)
	var toggle *mygo.MenuItem
	var walk func([]*mygo.MenuItem)
	walk = func(items []*mygo.MenuItem) {
		for _, item := range items {
			if item.Accelerator == "CmdOrCtrl+/" {
				t.Errorf("%s still owns Command-/, which belongs to Toggle Comment", item.Label)
			}
			if item.Label == "Toggle Preview / Markdown" {
				toggle = item
			}
			walk(item.Submenu)
		}
	}
	walk(menu.Items())
	if toggle == nil || toggle.Accelerator != "CmdOrCtrl+E" {
		t.Fatalf("Preview / Markdown toggle = %+v, want Command-E", toggle)
	}
}

func TestBuildMenuUsesSettingsCenterDeepLinks(t *testing.T) {
	menu := buildMenu(workspace.NewService(), true)
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

func TestFileMenuIncludesPDFExportAndPreferences(t *testing.T) {
	previous := supportsPDFBackground
	supportsPDFBackground = func() bool { return true }
	t.Cleanup(func() { supportsPDFBackground = previous })
	menu := buildMenu(nil, false)
	var exportMenu *mygo.MenuItem
	var walk func([]*mygo.MenuItem)
	walk = func(items []*mygo.MenuItem) {
		for _, item := range items {
			if item.Label == "Export" {
				exportMenu = item
			}
			walk(item.Submenu)
		}
	}
	walk(menu.Items())
	if exportMenu == nil || len(exportMenu.Submenu) != 11 {
		t.Fatalf("Export submenu = %+v", exportMenu)
	}
	if exportMenu.Submenu[0].Label != "HTML…" || exportMenu.Submenu[0].Click == nil {
		t.Fatalf("HTML export item = %+v", exportMenu.Submenu[0])
	}
	if exportMenu.Submenu[1].Label != "PDF…" || exportMenu.Submenu[1].Click == nil {
		t.Fatalf("PDF export item = %+v", exportMenu.Submenu[1])
	}
	for i, tc := range []struct{ id, label string }{
		{"export.markdown-docx", "Word (.docx)…"},
		{"export.markdown-pptx", "PowerPoint (.pptx)…"},
		{"export.markdown-epub", "EPUB…"},
		{"export.markdown-rtf", "RTF…"},
		{"export.markdown-latex", "LaTeX…"},
	} {
		item := exportMenu.Submenu[i+3]
		if item.ID != tc.id || item.Label != tc.label || item.Subtitle != "Requires Pandoc" || item.Click == nil || item.Disabled {
			t.Errorf("Pandoc export item %d = %+v", i, item)
		}
	}
	notes := exportMenu.Submenu[9]
	if notes.Label != "Include Reader Notes" || notes.Type != mygo.MenuItemCheckbox || notes.Click == nil || notes.Checked {
		t.Fatalf("Reader Notes item = %+v", notes)
	}
	keepDark := exportMenu.Submenu[10]
	if keepDark.ID != "export.keepDarkPDF" || keepDark.Label != "Keep Dark Theme for PDF" || keepDark.Type != mygo.MenuItemCheckbox || keepDark.Click == nil || keepDark.Checked || keepDark.Disabled {
		t.Fatalf("Keep Dark item = %+v", keepDark)
	}
}

func TestKeepDarkPDFMenuItemRequiresMacOS13Dot3(t *testing.T) {
	previous := supportsPDFBackground
	supportsPDFBackground = func() bool { return false }
	t.Cleanup(func() { supportsPDFBackground = previous })
	menu := buildMenu(nil, false)
	item := menu.ItemByID("export.keepDarkPDF")
	if item == nil || !item.Disabled || item.ToolTip != "Requires macOS 13.3 or later" {
		t.Fatalf("Keep Dark item = %+v", item)
	}
}

func TestViewMenuGenUIItemTracksThePluginPreference(t *testing.T) {
	menu := buildMenu(nil, false)
	item := menu.ItemByID("view.genui")
	if item == nil {
		t.Fatal("View is missing the GenUI item")
	}
	if item.IsEnabled() {
		t.Fatal("GenUI remained available while the plugin was off")
	}
	updateGenUIMenu(menu, true)
	if !item.IsEnabled() {
		t.Fatal("GenUI remained unavailable after the plugin was enabled")
	}
	updateGenUIMenu(menu, false)
	if item.IsEnabled() {
		t.Fatal("GenUI remained available after the plugin was disabled")
	}
}
