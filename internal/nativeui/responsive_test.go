package nativeui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func fileModel(entries []workspace.NavEntry) nativeui.Model {
	return nativeui.Model{
		Workspace:  workspace.WorkspaceSnapshot{Kind: "folder", Name: "Library", Root: "/library"},
		Navigation: workspace.NavSnapshot{Root: "/library", Entries: entries, Total: len(entries)},
	}
}

func TestNavigationTitlesMiddleTruncateWithoutCoveringStatusOrChevron(t *testing.T) {
	hash := strings.Repeat("a1b2c3d4", 8) + ".md"
	snake := "a_very_long_snake_case_file_name_that_cannot_wrap_anywhere_and_keeps_going.md"
	folder := "a_folder_with_an_unreasonably_long_snake_case_name_that_must_not_cover_its_chevron"
	entries := []workspace.NavEntry{
		{ID: "hash", Kind: "file", Name: hash, Path: "/library/" + hash, Conflict: true},
		{ID: "snake", Kind: "file", Name: snake, Path: "/library/" + snake, Edited: true},
		{ID: "folder", Kind: "folder", Name: folder, Path: "/library/" + folder, MarkdownCount: 17},
	}
	shortEntries := []workspace.NavEntry{
		{ID: "hash", Kind: "file", Name: "hash.md", Path: "/library/hash.md", Conflict: true},
		{ID: "snake", Kind: "file", Name: "snake.md", Path: "/library/snake.md", Edited: true},
		{ID: "folder", Kind: "folder", Name: "folder", Path: "/library/folder", MarkdownCount: 17},
	}
	render := func(entries []workspace.NavEntry) *ui.Tester {
		model := fileModel(entries)
		var shell nativeui.Shell
		app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 900, 720)
		app.Frame()
		return app
	}
	app, reference := render(entries), render(shortEntries)
	for _, trailing := range []string{"Conflict", "Unsaved", "17"} {
		long, longOK := app.Find(trailing)
		short, shortOK := reference.Find(trailing)
		if !longOK || !shortOK {
			t.Fatalf("trailing status %q missing: long=%v short=%v", trailing, longOK, shortOK)
		}
		if n := changed(pixels(app.Image(), long), pixels(reference.Image(), short)); n != 0 {
			t.Errorf("long title changes %d pixels inside trailing status %q", n, trailing)
		}
	}
	longChevron, longOK := app.Find(portrait.IconCaretRight.Glyph)
	shortChevron, shortOK := reference.Find(portrait.IconCaretRight.Glyph)
	if !longOK || !shortOK {
		t.Fatal("folder chevron is missing")
	}
	if n := changed(pixels(app.Image(), longChevron), pixels(reference.Image(), shortChevron)); n != 0 {
		t.Errorf("long folder title changes %d pixels inside its chevron", n)
	}

	row := mustAccess(t, app, ui.RoleTreeItem, hash)
	app.Move(row.Bounds.X+row.Bounds.W/2, row.Bounds.Y+row.Bounds.H/2)
	app.Advance(time.Second)
	if _, ok := app.AccessByRole(ui.RoleTooltip, "/library/"+hash); ok {
		t.Fatal("hovering a sidebar file row should not pop up a full-path tooltip")
	}
}

func TestSamePrefixVersionsRemainVisuallyDistinct(t *testing.T) {
	prefix := "release_notes_with_a_shared_prefix_that_is_far_too_long_to_fit_"
	v2, v3 := prefix+"v2.md", prefix+"v3.md"
	model := fileModel([]workspace.NavEntry{
		{ID: "v2", Kind: "file", Name: v2, Path: "/library/" + v2},
		{ID: "v3", Kind: "file", Name: v3, Path: "/library/" + v3},
	})
	var shell nativeui.Shell
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 760, 720)
	app.Frame()
	r2 := mustAccess(t, app, ui.RoleTreeItem, v2).Bounds
	r3 := mustAccess(t, app, ui.RoleTreeItem, v3).Bounds
	r2.X, r3.X = r2.X+30, r3.X+30
	r2.W, r3.W = r2.W-38, r3.W-38
	if changed(pixels(app.Image(), r2), pixels(app.Image(), r3)) == 0 {
		t.Fatal("same-prefix -v2 and -v3 titles have identical pixels; middle truncation hid the version")
	}
}

func TestNarrowDrawerKeepsFooterInsideItsClip(t *testing.T) {
	entries := make([]workspace.NavEntry, 205)
	for i := range entries {
		name := fmt.Sprintf("File %03d.md", i)
		entries[i] = workspace.NavEntry{ID: name, Kind: "file", Name: name, Path: "/library/" + name}
	}
	model := fileModel(entries)
	var shell nativeui.Shell
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 560, 720)
	app.Frame()
	if err := app.Click("Show navigation"); err != nil {
		t.Fatal(err)
	}
	app.Advance(time.Second)
	drawer := mustAccess(t, app, ui.RoleDialog, "Navigation").Bounds
	for _, name := range []string{"Settings", "New document"} {
		button := mustAccess(t, app, ui.RoleButton, name)
		r := button.Bounds
		if button.Offscreen || r.X < drawer.X || r.Y < drawer.Y || r.X+r.W > drawer.X+drawer.W || r.Y+r.H > drawer.Y+drawer.H {
			t.Errorf("%s footer %v is outside drawer %v (offscreen=%v)", name, r, drawer, button.Offscreen)
		}
	}
}

func TestBreadcrumbPreservesActiveFilenameAtNarrowWidths(t *testing.T) {
	for _, width := range []int{641, 784} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			model := libraryModel()
			model.Crumbs = []workspace.Crumb{
				{Name: "很长的祖先文件夹名称", Path: "/library/很长的祖先文件夹名称"},
				{Name: "an-extremely-long-latin-ancestor-folder-name", Path: "/library/ancestor"},
				{Name: "另一个需要折叠的长目录", Path: "/library/ancestor/另一个需要折叠的长目录"},
				{Name: "One.md", Path: "/library/One.md", IsFile: true},
			}
			var shell nativeui.Shell
			app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, width, 720)
			app.Frame()
			name, ok := app.Find("One.md")
			if !ok || name.W <= 24 {
				t.Fatalf("active filename bounds = %v, present=%v; want readable width", name, ok)
			}
			preview := mustAccess(t, app, ui.RoleTab, "Preview").Bounds
			if name.X+name.W > preview.X {
				t.Fatalf("active filename %v overlaps view control %v", name, preview)
			}
		})
	}
}

func TestAppearanceGridUsesAvailableWidthAt784(t *testing.T) {
	render := func(width int) (*ui.Tester, ui.Rect) {
		model := libraryModel()
		var shell nativeui.Shell
		app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, width, 820)
		app.Frame()
		shell.OpenSettings("appearance")
		app.Frame()
		app.Frame()
		label, ok := app.Find("Metal–Water")
		if !ok {
			t.Fatalf("Metal–Water is not drawn at %dpt", width)
		}
		return app, label
	}
	_, wide := render(1280)
	app, narrow := render(784)
	if narrow.W+1 < wide.W {
		t.Fatalf("Metal–Water truncated at 784pt: width %v, full width %v", narrow.W, wide.W)
	}
	first := mustAccess(t, app, ui.RoleButton, "Space: "+portrait.Spaces[0].Label).Bounds
	second := mustAccess(t, app, ui.RoleButton, "Space: "+portrait.Spaces[1].Label).Bounds
	third := mustAccess(t, app, ui.RoleButton, "Space: "+portrait.Spaces[2].Label).Bounds
	if second.Y < first.Y-1 || second.Y > first.Y+1 || third.Y <= first.Y {
		t.Fatalf("784pt grid did not collapse to two columns: %v %v %v", first, second, third)
	}
}

func dragSidebarTo(t *testing.T, app *ui.Tester, width float32) {
	t.Helper()
	var divider ui.AccessInfo
	for _, info := range app.AccessAll() {
		if info.Role == ui.RoleSplitter {
			divider = info
			break
		}
	}
	if divider.Bounds.W == 0 {
		t.Fatal("sidebar splitter is missing")
	}
	y := divider.Bounds.Y + divider.Bounds.H/2
	app.Press(divider.Bounds.X+divider.Bounds.W/2, y)
	app.Move(width, y)
	app.Release(width, y)
	app.Frame()
}

func TestSpaceDotsWrapWithoutOverflowAndKeepTheirPitch(t *testing.T) {
	for _, width := range []float32{232, portrait.Shell.SidebarMin} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			model := libraryModel()
			var shell nativeui.Shell
			app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 1280, 820)
			app.Frame()
			dragSidebarTo(t, app, width)
			side := mustAccess(t, app, ui.RoleNavigation, "Workspace navigation").Bounds
			var previous ui.AccessInfo
			rows := 0
			for _, space := range portrait.Spaces {
				dot := mustAccess(t, app, ui.RoleButton, "Space: "+space.Label)
				r := dot.Bounds
				if dot.Offscreen || r.X < side.X || r.X+r.W > side.X+side.W {
					t.Errorf("%s at %v overflows sidebar %v", space.Label, r, side)
				}
				if previous.Bounds.W > 0 {
					if r.Y != previous.Bounds.Y {
						rows++
					} else if pitch := (r.X + r.W/2) - (previous.Bounds.X + previous.Bounds.W/2); pitch != portrait.Shell.SpaceDotButton+2 {
						t.Errorf("dot pitch = %v, want %v", pitch, portrait.Shell.SpaceDotButton+2)
					}
				}
				previous = dot
			}
			if width == portrait.Shell.SidebarMin && rows == 0 {
				t.Fatal("Space dots did not wrap at SidebarMin")
			}
			if width == 232 && rows != 0 {
				t.Fatal("Space dots wrapped at the default 232pt sidebar")
			}
		})
	}
}

func TestDirectoryDrillRestoresParentScrollOffset(t *testing.T) {
	entries := make([]workspace.NavEntry, 0, 46)
	for i := range 45 {
		name := fmt.Sprintf("File %02d.md", i)
		entries = append(entries, workspace.NavEntry{ID: name, Kind: "file", Name: name, Path: "/library/" + name})
	}
	entries = append(entries,
		workspace.NavEntry{ID: "folder", Kind: "folder", Name: "Folder", Path: "/library/Folder", FolderState: workspace.FolderOpen},
		workspace.NavEntry{ID: "nested", Kind: "file", Name: "Nested.md", Path: "/library/Folder/Nested.md", Depth: 1},
	)
	model := fileModel(entries)
	var shell nativeui.Shell
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 900, 500)
	app.Frame()
	tree := mustAccess(t, app, ui.RoleTree, "Workspace files").Bounds
	app.Scroll(tree.X+tree.W/2, tree.Y+tree.H/2, 0, 2000)
	app.Frame()
	before := mustAccess(t, app, ui.RoleTreeItem, "File 44.md").Bounds
	if err := app.Click("Folder"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleTreeItem, "Nested.md"); !ok {
		t.Fatal("folder drill did not open")
	}
	if err := app.Click("Go to Library"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	after := mustAccess(t, app, ui.RoleTreeItem, "File 44.md").Bounds
	if delta := after.Y - before.Y; delta > 1 || delta < -1 {
		t.Fatalf("parent row moved from Y=%v to %v after drill and return", before.Y, after.Y)
	}
}

func TestActiveTabAndFocusedFileScrollClearOfViewportEdges(t *testing.T) {
	model := libraryModel()
	model.Workspace.Documents = nil
	for i := range 12 {
		id := fmt.Sprintf("d%d", i)
		model.Workspace.Documents = append(model.Workspace.Documents, workspace.DocumentInfo{ID: id, Name: fmt.Sprintf("Document %02d.md", i)})
	}
	model.Workspace.ActiveID = "d11"
	var shell nativeui.Shell
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 900, 500)
	app.Frame()
	app.Frame()
	list := mustAccess(t, app, ui.RoleTabList, "Documents").Bounds
	active := mustAccess(t, app, ui.RoleTab, "Document 11.md")
	if active.Offscreen || active.Bounds.Y < list.Y || active.Bounds.Y+active.Bounds.H > list.Y+list.H {
		t.Fatalf("active tab %v is outside documents viewport %v", active.Bounds, list)
	}

	entries := make([]workspace.NavEntry, 211)
	for i := range entries {
		name := fmt.Sprintf("File %03d.md", i)
		entries[i] = workspace.NavEntry{ID: name, Kind: "file", Name: name, Path: "/library/" + name}
	}
	files := fileModel(entries)
	files.Workspace.Documents = model.Workspace.Documents[:1]
	files.Workspace.ActiveID = "d0"
	var fileShell nativeui.Shell
	fileApp := ui.NewTester(func(c *ui.Context) { fileShell.Draw(c, files, nativeui.Actions{}, nativeui.Slots{}) }, 900, 500)
	fileApp.Frame()
	focus(t, fileApp, "File 000.md")
	fileApp.Key(0, ui.KeyEnd)
	fileApp.Frame()
	fileApp.Frame()
	row := mustAccess(t, fileApp, ui.RoleTreeItem, "File 210.md")
	tree := mustAccess(t, fileApp, ui.RoleTree, "Workspace files").Bounds
	fadeStart := tree.Y + tree.H*(1-scrollEdgeFadeForTest)
	if row.Bounds.Y+row.Bounds.H > fadeStart {
		t.Fatalf("focused row %v extends into bottom fade beginning at %v", row.Bounds, fadeStart)
	}
}

const scrollEdgeFadeForTest = 0.02
