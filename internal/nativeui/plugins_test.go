package nativeui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

func pluginWindow(values map[string]string) *Window {
	return &Window{preferences: &preferenceStore{values: values, windows: map[*Window]bool{}}}
}

func TestGenUIIsOnUntilAnExplicitOff(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		want   bool
	}{
		{"no saved preference", map[string]string{}, true},
		{"saved on", map[string]string{"mgo.plugin.genui": "on"}, true},
		{"saved off", map[string]string{"mgo.plugin.genui": "off"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pluginWindow(tt.values).PluginEnabled("genui"); got != tt.want {
				t.Fatalf("GenUI enabled = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFeynmanAndGitStayOffUntilEnabled(t *testing.T) {
	for _, name := range []string{"feynman", "git"} {
		if pluginWindow(map[string]string{}).PluginEnabled(name) {
			t.Fatalf("%s starts enabled without a saved preference", name)
		}
		if !pluginWindow(map[string]string{"mgo.plugin." + name: "on"}).PluginEnabled(name) {
			t.Fatalf("%s saved on reads as disabled", name)
		}
	}
}

func TestGenUIPluginOffReachesEveryWindowAndSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() { delete(preferenceStores, dir) })
	first := &Window{preferences: preferencesFor(dir)}
	second := &Window{preferences: preferencesFor(dir)}
	first.preferences.windows[first], first.preferences.windows[second] = true, true
	heard := false
	second.changed = []func(){func() { heard = true }}

	first.setPlugin("genui", false)

	if first.PluginEnabled("genui") || second.PluginEnabled("genui") {
		t.Fatal("turning GenUI off left it enabled")
	}
	if !heard {
		t.Fatal("the other window was not asked to redraw")
	}
	data, err := os.ReadFile(filepath.Join(dir, "portrait.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"mgo.plugin.genui":"off"`) {
		t.Fatalf("saved preferences = %s", data)
	}

	delete(preferenceStores, dir)
	restarted := &Window{preferences: preferencesFor(dir)}
	if restarted.PluginEnabled("genui") {
		t.Fatal("GenUI came back on after a restart")
	}
}

func genuiWindow(values map[string]string, views ...string) *Window {
	w := pluginWindow(values)
	w.documents = map[string]*openDocument{}
	documents := make([]workspace.DocumentInfo, len(views))
	for i, view := range views {
		id := fmt.Sprintf("doc%d", i)
		path := "/docs/" + id + ".md"
		documents[i] = workspace.DocumentInfo{ID: id, Name: id + ".md", Path: path, ViewMode: view}
		w.documents[id] = &openDocument{source: nativeeditor.New("# " + id)}
	}
	w.snapshot = workspace.WorkspaceSnapshot{ActiveID: documents[0].ID, Documents: documents}
	return w
}

func TestDocumentsFallBackFromGenUIWhileItsPluginIsOff(t *testing.T) {
	off := genuiWindow(map[string]string{"mgo.plugin.genui": "off", "mgo.viewMode:/docs/doc0.md": "genui"}, "genui", "genui")
	off.syncEditorMode()
	for _, doc := range off.snapshot.Documents {
		if mode := off.viewMode(doc); mode != "preview" {
			t.Fatalf("%s shows %q while GenUI is off", doc.ID, mode)
		}
	}
	if mode := off.documents["doc0"].source.Mode(); mode != nativeeditor.Preview {
		t.Fatalf("the reopened document was left in %v, want Preview", mode)
	}
	if saved := off.Preference("mgo.viewMode:/docs/doc0.md"); saved != "genui" {
		t.Fatalf("falling back rewrote the saved view mode to %q", saved)
	}

	on := genuiWindow(map[string]string{}, "genui")
	on.syncEditorMode()
	if mode := on.viewMode(on.Active()); mode != "genui" {
		t.Fatalf("a GenUI document left GenUI while its plugin is on: %q", mode)
	}
}

func TestSetModeRefusesGenUIWhileItsPluginIsOff(t *testing.T) {
	w := genuiWindow(map[string]string{"mgo.plugin.genui": "off"}, "preview")
	w.SetMode("genui")
	if got := w.snapshot.Documents[0].ViewMode; got != "preview" {
		t.Fatalf("a shortcut or menu left the document in %q", got)
	}
	if saved := w.Preference("mgo.viewMode:/docs/doc0.md"); saved != "" {
		t.Fatalf("a refused switch saved the view mode %q", saved)
	}
}
