package nativeui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortraitDialogAndMenuSourceContract(t *testing.T) {
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if src := read("window.go"); !strings.Contains(src, "c.SetMenuPresentation(ui.PopoverMenus)") {
		t.Fatal("Window.View must opt every text/editor menu into in-window popover menus")
	}
	// Save, Close, Rename and MoveTo with an empty argument each open a native
	// alert, panel or confirmation; the Portrait surfaces own those decisions.
	forbidden := []string{"mygo.Dialog.Message", "doc.Save()", "doc.Close()", `doc.Rename("")`, `doc.MoveTo("")`}
	err := filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		src := read(path)
		clean := filepath.Clean(path)
		for _, token := range forbidden {
			audited := clean == filepath.Clean("../workspace/restore_host.go") && token == "mygo.Dialog.Message"
			if !audited && strings.Contains(src, token) {
				t.Errorf("%s contains forbidden interactive native call %s", path, token)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// documenthost.go is the one audited boundary for conflict-checked Save;
	// its Save method returns Portrait-decision sentinels before native prompts.
	host := read("../workspace/documenthost.go")
	if !strings.Contains(host, "return ErrSaveConflict") || !strings.Contains(host, "return ErrSaveReadOnly") {
		t.Fatal("interactive Save must be pre-empted by Portrait conflict/read-only decisions")
	}
	// A move must displace a taken destination, never delete it up front.
	moves := read("../workspace/document_decisions.go")
	if strings.Contains(moves, "os.Remove(target)") {
		t.Fatal("a move must never remove its destination before the move committed")
	}
	if !strings.Contains(moves, "displaceFile(") || !strings.Contains(moves, "restoreFile(") {
		t.Fatal("a move replacement must displace the destination and restore it on failure")
	}
}

func TestCloseDialogOwnsItsDocumentsUntilItResolves(t *testing.T) {
	w := &Window{}
	w.setClosing([]string{"a", "b"}, true)
	w.dialog = &windowDialog{kind: "closeWorkspace", ids: []string{"a", "b"}, open: true}

	// A quit failure alert arrives while the sheet is up: it waits instead of
	// replacing the sheet and stranding the closing flags.
	w.ShowAlert("Could not quit", "The document is still open.")
	if w.dialog == nil || w.dialog.kind != "closeWorkspace" {
		t.Fatalf("the alert replaced the close sheet: %+v", w.dialog)
	}
	if len(w.alerts) != 1 {
		t.Fatalf("queued alerts = %d, want 1", len(w.alerts))
	}
	if !w.closing["a"] || !w.closing["b"] {
		t.Fatal("the queued alert dropped the close's closing flags")
	}

	// Every other request waits for the sheet as well, as a modal window
	// blocks its menu commands.
	w.openRename("a")
	if w.dialog.kind != "closeWorkspace" {
		t.Fatalf("a rename replaced the close sheet: %+v", w.dialog)
	}

	w.cancelDialog()
	if w.closing["a"] || w.closing["b"] {
		t.Fatal("cancel left documents marked closing")
	}
	if w.dialog == nil || w.dialog.kind != "alert" {
		t.Fatalf("queued alert not shown after the sheet: %+v", w.dialog)
	}
}

func TestReplacingAPromptNeverClearsCloseFlags(t *testing.T) {
	w := &Window{}
	w.setClosing([]string{"a"}, true)
	w.dialog = &windowDialog{kind: "rename", id: "a", open: true}
	w.showDialog(&windowDialog{kind: "move", id: "a", open: true})
	if w.dialog == nil || w.dialog.kind != "move" {
		t.Fatalf("dialog = %+v, want the move prompt", w.dialog)
	}
	if !w.closing["a"] {
		t.Fatal("replacing a non-close dialog dropped a close's protection")
	}
	w.cancelDialog()
}
