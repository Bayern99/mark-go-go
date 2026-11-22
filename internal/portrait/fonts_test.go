package portrait_test

import (
	"os"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestMain(m *testing.M) {
	if err := portrait.RegisterFonts(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestPortraitHasOfflineTypefaces(t *testing.T) {
	registered := map[string]bool{}
	for _, f := range ui.FontFamilies() {
		if f.Registered {
			registered[f.Name] = true
		}
	}
	for _, name := range []string{"IBM Plex Sans", "IBM Plex Sans SC", "IBM Plex Mono", "Barlow Semi Condensed", "Phosphor"} {
		if !registered[name] {
			t.Errorf("typeface absent from native text system: %s", name)
		}
	}
}
