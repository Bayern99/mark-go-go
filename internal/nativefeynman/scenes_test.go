package nativefeynman

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

const sceneDocument = `# Field Notes on Quiet Software

Good tools recede. The page stays in front, and everything that helps you
write waits at the edge of the window until you reach for it.

## Principles

- **Markdown is the artifact.** Every view reads and writes the same bytes.
- **Local first.** Files live in your folder, not in an account.
- [x] Sidebar holds documents, files and outline
- [ ] Command Bar reaches everything else

> A window should feel like a room, not a dashboard.

[Project](https://example.com/project) keeps link actions close to the text.

## 中文排版

界面以英文为主，正文同样照顾中文段落：汉字自动落到中文字体，行高与字间距保持
舒展，中英文混排（例如 Markdown 与 GenUI）不需要额外设置。

| Surface | Role | Material |
| --- | --- | --- |
| Scenery | Window field, sidebar | Tinted, translucent |
| Page | The document | Opaque paper |

| 五行 | 方位与时令 | 说明 |
| --- | --- | --- |
| 金 | 西 · 秋 | 素白与霜华，适合长篇阅读时保持冷静、克制的界面气质。 |
| 木 | 东 · 春 | 月白与松花绿，正文与表格中的中文长句应自动换行而不被裁切。 |
| 火 | 南 · 夏 | 鹅血石红作为强调色 |

` + "```go\nfunc main() {\n\tfmt.Println(\"hello, page\")\n}\n```\n"

func sceneModel(feynmanOn bool, mode string) nativeui.Model {
	root := "/Users/ada/Field Notes"
	docs := []workspace.DocumentInfo{
		{WorkspaceRoot: root, ID: "a", Name: "Quiet Software.md", Path: root + "/Essays/Quiet Software.md", ViewMode: mode},
		{WorkspaceRoot: root, ID: "b", Name: "Reading List.md", Path: root + "/Reading List.md", ViewMode: "preview", Edited: true},
		{WorkspaceRoot: root, ID: "c", Name: "排版笔记.md", Path: root + "/Essays/排版笔记.md", ViewMode: "preview"},
		{WorkspaceRoot: root, ID: "d", Name: "Roadmap.md", Path: root + "/Plans/Roadmap.md", ViewMode: "preview", Generating: true},
	}
	entry := func(id, kind, name, path string, depth, count int, state workspace.FolderState, doc string) workspace.NavEntry {
		parent := filepath.Dir(path)
		if parent == "." {
			parent = "Field Notes"
		}
		return workspace.NavEntry{ID: id, Kind: kind, Name: name, Path: root + "/" + path, Parent: parent, Depth: depth, MarkdownCount: count, FolderState: state, DocumentID: doc}
	}
	return nativeui.Model{
		Feynman: feynmanOn, Git: true,
		Crumbs: workspace.Breadcrumbs(root, root+"/Essays/Quiet Software.md"),
		Workspace: workspace.WorkspaceSnapshot{
			ID: "w1", Root: root, Name: "Field Notes", Kind: "folder", ActiveID: "a", Documents: docs,
			Workspaces: []workspace.WorkspaceSummary{{ID: "w1", Root: root, Name: "Field Notes"}, {ID: "w2", Root: "/Users/ada/Lab", Name: "Lab"}, {ID: "w3", Root: "/Users/ada/Journal", Name: "Journal"}},
		},
		Navigation: workspace.NavSnapshot{Root: root, Total: 9, Indexed: 9, Entries: []workspace.NavEntry{
			entry("f1", "folder", "Essays", "Essays", 0, 3, workspace.FolderOpen, ""),
			entry("e1", "file", "Quiet Software.md", "Essays/Quiet Software.md", 1, 0, "", "a"),
			entry("e2", "file", "排版笔记.md", "Essays/排版笔记.md", 1, 0, "", "c"),
			entry("e3", "file", "On Margins.md", "Essays/On Margins.md", 1, 0, "", ""),
			entry("f2", "folder", "Plans", "Plans", 0, 2, workspace.FolderOpen, ""),
			entry("p1", "file", "Roadmap.md", "Plans/Roadmap.md", 1, 0, "", "d"),
			entry("p2", "file", "Release Notes.md", "Plans/Release Notes.md", 1, 0, "", ""),
			entry("f3", "folder", "Archive", "Archive", 0, 3, workspace.FolderCollapsed, ""),
			entry("r1", "file", "Reading List.md", "Reading List.md", 0, 0, "", "b"),
		}},
		Sections: workspace.ExtractSections(sceneDocument),
	}
}

func scenePanel() *panel {
	p := fixtureReadyPanel()
	p.state.root = "/Users/ada/Field Notes"
	p.state.title = "Quiet software"
	p.state.draft = ""
	p.state.messages = []message{
		{kind: "user", messageID: "u1", text: "Tighten the Principles section and keep the checklist."},
		{kind: "assistant", messageID: "a1", text: "I read **Quiet Software.md** and drafted a change proposal:\n\n1. Merge the two opening bullets into one sentence.\n2. Keep both task items; the second stays open.\n\nReview it in the editor when you are ready."},
		{kind: "tool", tool: feynman.ResearchTool{ToolCallID: "t1", Title: "Propose change", Status: "completed", Output: "Quiet Software.md"}},
	}
	return p
}

func TestShellScenes(t *testing.T) {
	dir := os.Getenv("MGG_SCENES_DIR")
	if dir == "" {
		t.Skip("set MGG_SCENES_DIR to export shell scenes as PNG")
	}
	prefix := os.Getenv("MGG_SCENES_PREFIX")
	scenes := sceneList()
	if os.Getenv("MGG_SCENES_SET") == "wuxing" {
		scenes = wuxingScenes()
	}
	t.Logf("exporting %d scenes", len(scenes))
	for _, sc := range scenes {
		t.Run(sc.name, func(t *testing.T) {
			var shell nativeui.Shell
			model := sceneModel(sc.feynman, sc.mode)
			if sc.space != "" {
				model.Spaces = map[string]string{model.Workspace.Root: sc.space}
			}
			if sc.dark {
				model.Appearance = portrait.Dark
			}
			if sc.empty {
				model = nativeui.Model{Recents: []workspace.RecentWorkspace{{Name: "Field Notes", Root: "/Users/ada/Field Notes"}, {Name: "Lab", Root: "/Users/ada/Lab"}}}
			}
			sourceText := sceneDocument
			if sc.document != "" {
				sourceText = sc.document
				model.Sections = workspace.ExtractSections(sourceText)
			}
			source := nativeeditor.New(sourceText)
			source.SetMode(nativeeditor.Mode(sc.mode))
			p := scenePanel()
			if sc.research != nil {
				sc.research(p)
			}
			slots := nativeui.Slots{
				Document: func(c *ui.Context) {
					field := source.View(c.Key("scene-doc"), false).Grow(1).MinHeight(0).FillWidth()
					if strings.HasPrefix(sc.name, "link_") {
						field.AutoFocus()
					}
				},
				Research: func(c *ui.Context) {
					ui.Column(c.Key("scene-research")).Fill().MinWidth(0).Children(func() { p.draw(c) })
				},
				Settings:    p.settingsPages(),
				Checkpoints: func(c *ui.Context) {},
			}
			if sc.name == "checkpoints_light" {
				slots.Checkpoints = func(c *ui.Context) {
					portrait.SettingsGroup(c, "Checkpoints", func(rows *portrait.Rows) {
						rows.Row(portrait.RowSpec{Title: "Quiet beginning", Description: "Git ref · 2026-01-14 09:30"}, func() { portrait.Button(c, "Compare", "Compare the selected checkpoint") })
						rows.Row(portrait.RowSpec{Title: "Before review", Description: "Local snapshot · 2026-01-14 10:15"}, func() {})
					})
				}
			}
			if strings.HasPrefix(sc.name, "proposals_") {
				open := false
				slots.Review = func(c *ui.Context) {
					button := portrait.ButtonWith(c, "Change proposals · 1", "Change proposals", portrait.ButtonSpec{Look: portrait.Quiet, Leading: portrait.IconSparkle, Selected: open})
					if button.Clicked() {
						open = !open
					}
					portrait.PopoverWithSpec(c, button, &open, portrait.PopoverSpec{Width: portrait.Form.PopoverWide, Title: "Change proposals", Opaque: true}, func() {
						ui.Column(c).Padding(0, portrait.Form.PadX).Children(func() {
							portrait.SettingsGroup(c, "Quiet Software.md", func(rows *portrait.Rows) {
								rows.Row(portrait.RowSpec{Title: "Tighten the introduction", Description: "Feynman Research · 2 clean hunks"}, func() { portrait.AccentButton(c, "Apply decisions", "Apply decisions") })
							})
						})
					}).Background(portrait.Colors.Paper)
				}
			}
			if strings.HasPrefix(sc.name, "diff_") {
				slots.Checkpoints = func(c *ui.Context) {
					portrait.SettingsGroup(c, "Quiet Software.md", func(rows *portrait.Rows) {
						rows.Row(portrait.RowSpec{Title: "@@ -1,3 +1,3 @@", Description: "2 additions · 1 deletion", Stacked: true}, func() {
							ui.Column(c).FillWidth().Radius(portrait.Radii.M).Clip().Children(func() {
								for _, line := range []struct {
									text string
									back ui.Color
								}{{"  Good tools recede.", portrait.Clear}, {"- The window stays in front.", portrait.Diff.Delete}, {"+ The page stays in front.", portrait.Diff.Insert}, {"+ Everything else waits at the edge.", portrait.Diff.Insert}} {
									portrait.Diff.Lines.Apply(ui.Text(c, line.text)).TextColor(portrait.Colors.Ink).FillWidth().Padding(portrait.Form.HelpGap, portrait.Form.PadX).Background(line.back)
								}
							})
						})
					})
				}
			}
			if strings.HasPrefix(sc.name, "genui_controls_") {
				textValue, areaValue := "Calm reader", "A multiline request with Portrait field chrome."
				selectedValue, radioValue, dateValue := "preview", "paper", time.Now()
				sliderValue := 64.0
				checked, tab := true, 0
				slots.GenUI = func(c *ui.Context) {
					ui.Scroll(c).Fill().Children(func() {
						ui.Column(c).FillWidth().MaxWidth(portrait.Settings.Column).Padding(portrait.Settings.ColumnPadTop).Gap(portrait.Form.Gap).Children(func() {
							portrait.PanelTitle(c, "GenUI Reader Controls")
							portrait.TextField(c, &textValue, portrait.FieldSpec{Label: "Title"}).Frame().FillWidth()
							portrait.TextArea(c, &areaValue, portrait.AreaSpec{Label: "Request", Rows: 3})
							portrait.Select(c, &selectedValue, []portrait.Option{{Value: "preview", Label: "Preview"}, {Value: "markdown", Label: "Markdown"}}, portrait.SelectSpec{Label: "View"})
							portrait.Slider(c, &sliderValue, 0, 100).Label("Reading width")
							portrait.Checkbox(c, &checked, "Show sources")
							ui.RadioGroup(c, func() {
								portrait.Radio(c.Key("paper"), &radioValue, "paper", "Published paper")
								portrait.Radio(c.Key("notes"), &radioValue, "notes", "Local notes")
							})
							portrait.DateField(c, &dateValue, "Publication date")
							portrait.Segmented(c, &tab, []string{"Summary", "Evidence", "Sources"}, "Reading sections")
							ui.Row(c).Gap(portrait.Form.Gap).Children(func() {
								portrait.AccentButton(c, "Generate", "Generate view")
								portrait.ButtonWith(c, "Delete", "Delete view", portrait.ButtonSpec{Look: portrait.DangerOutline})
							})
						})
					})
				}
			}
			if strings.HasPrefix(sc.name, "settings_genui_") {
				slots.Settings = append(slots.Settings, nativeui.SettingsPage{ID: "genui", Section: "Workspace", Label: "GenUI", Icon: portrait.IconSparkle, Order: 210, Draw: func(c *ui.Context, _ nativeui.SettingsQuery) {
					portrait.SettingsGroup(c, "Generated views", func(rows *portrait.Rows) {
						rows.Row(portrait.RowSpec{Title: "Default view", Description: "Choose how generated documents first open."}, func() {
							value := "preview"
							portrait.Select(c, &value, []portrait.Option{{Value: "preview", Label: "Preview"}, {Value: "genui", Label: "GenUI"}}, portrait.SelectSpec{Label: "Default view"})
						})
					})
				}})
			}
			if sc.name == "genui_request_light" {
				request := "Create a calm reading view with a summary and key principles."
				slots.GenUI = func(c *ui.Context) {
					ui.Column(c).Fill().Padding(0, 12, 12, 12).Gap(portrait.Form.Gap).Children(func() {
						portrait.PanelTitle(c, "Request")
						portrait.TextArea(c, &request, portrait.AreaSpec{Label: "Generation request", Rows: 3})
						portrait.AccentButton(c, "Generate", "Generate").AlignSelf(ui.Start)
					})
				}
			}
			app := ui.NewTester(func(c *ui.Context) {
				c.SetMenuPresentation(ui.PopoverMenus)
				shell.Draw(c, model, sceneActions(&model), slots)
				if sc.draw != nil {
					sc.draw(c)
				}
			}, sc.width, sc.height)
			app.SetScale(2)
			app.Frame()
			if sc.act != nil {
				sc.act(app)
			}
			if sc.actSource != nil {
				sc.actSource(app, source)
			}
			app.Advance(2 * time.Second)
			app.Frame()
			app.Frame()
			if strings.HasPrefix(sc.name, "feynman_picker_") {
				row, ok := app.AccessByRole(ui.RoleButton, "Show Anthropic models")
				if !ok {
					t.Fatal("model picker has no Anthropic provider row")
				}
				ink := 0
				img := app.Image()
				for y := int(row.Bounds.Y * 2); y < int((row.Bounds.Y+row.Bounds.H)*2); y++ {
					for x := int(row.Bounds.X * 2); x < int((row.Bounds.X+row.Bounds.W)*2); x++ {
						px := img.RGBAAt(x, y)
						sum := int(px.R) + int(px.G) + int(px.B)
						if !sc.dark && sum < 450 || sc.dark && sum > 450 {
							ink++
						}
					}
				}
				if ink < 10 {
					t.Fatalf("model provider row is blank: bounds=%+v contrasting pixels=%d", row.Bounds, ink)
				}
			}
			if strings.HasPrefix(sc.name, "link_hover_") {
				if _, ok := app.Find("Open link"); !ok {
					t.Fatal("link hover scene has no visible hover-card actions")
				}
			}
			path := filepath.Join(dir, prefix+sc.name+".png")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(file, app.Image()); err != nil {
				_ = file.Close()
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			exported, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := png.Decode(exported); err != nil {
				_ = exported.Close()
				t.Fatalf("exported scene %s is not a complete PNG: %v", path, err)
			}
			if err := exported.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
