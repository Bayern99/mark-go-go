package portrait_test

import (
	"sync"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui/editor"
)

func TestExportSnapshotDoesNotChangeActiveTokens(t *testing.T) {
	portrait.Use(portrait.DefaultSpace, false)
	before := portrait.Colors
	snapshot := portrait.Snapshot(portrait.Spaces[7], true)
	if snapshot.Colors.Paper != portrait.Spaces[7].Dark.Paper || snapshot.Reader.Charts != portrait.Reader.Charts {
		t.Fatal("export snapshot did not resolve its own appearance")
	}
	if portrait.Colors != before {
		t.Fatal("export snapshot changed global colors")
	}
	space, dark := portrait.ActiveSpace()
	if space.Name != portrait.DefaultSpace.Name || dark {
		t.Fatal("export snapshot changed the active Space")
	}
}

func TestExportSnapshotsAreIndependent(t *testing.T) {
	var wg sync.WaitGroup
	for i, space := range portrait.Spaces {
		for _, dark := range []bool{false, true} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				snapshot := portrait.Snapshot(space, dark)
				if snapshot.Colors.Accent != space.In(dark).Accent {
					t.Errorf("%s dark=%v resolved the wrong accent", space.Name, dark)
				}
			}()
		}
		_ = i
	}
	wg.Wait()
}

func TestLightSnapshotDeletedTokenDoesNotReadActiveDarkPalette(t *testing.T) {
	space := portrait.Spaces[0]
	portrait.Use(space, true)
	snapshot := portrait.Snapshot(space, false)
	if got, want := snapshot.Code.Tokens[editor.TokenDeleted].Color, snapshot.Colors.Ink3; got != want {
		t.Fatalf("deleted token = %+v, want light Ink3 %+v", got, want)
	}
}

func TestSnapshotAndAppearanceUseAreRaceFree(t *testing.T) {
	space := portrait.Spaces[1]
	want := portrait.Snapshot(space, false).Colors.Ink3
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			portrait.Use(space, i%2 == 0)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			snapshot := portrait.Snapshot(space, false)
			if got := snapshot.Code.Tokens[editor.TokenDeleted].Color; got != want {
				t.Errorf("deleted token = %+v, want %+v", got, want)
				return
			}
		}
	}()
	wg.Wait()
}

func TestExportFontFile(t *testing.T) {
	data, err := portrait.FontFile("Phosphor.ttf")
	if err != nil || len(data) == 0 {
		t.Fatalf("FontFile: %d bytes, %v", len(data), err)
	}
	if _, err := portrait.FontFile("../portrait.go"); err == nil {
		t.Fatal("FontFile accepted traversal")
	}
}
