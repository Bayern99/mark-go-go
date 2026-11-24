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
	forbidden := []string{"mygo.Dialog.Message", `doc.Rename("")`, `doc.MoveTo("")`}
	err := filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		src := read(path)
		clean := filepath.Clean(path)
		for _, token := range forbidden {
			audited := clean == filepath.Clean("../workspace/documenthost.go") && token != "mygo.Dialog.Message" ||
				clean == filepath.Clean("../workspace/restore_host.go") && token == "mygo.Dialog.Message"
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
}
