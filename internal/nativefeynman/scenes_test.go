package nativefeynman

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/nativegenui"
	"github.com/Bayern99/mark-go-go/internal/nativereview"
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

func denseSidebarSceneModel() nativeui.Model {
	root := "/Users/ada/sections"
	names := []string{
		"ch04-s05-trade-shells.md", "ch04-s06-lapidary-technology.md", "ch04-s07-highland-longshan-taosi.md",
		"ch04-s08-taosi-culinary.md", "ch04-s09-taosi-music.md", "ch04-s10-dress-body.md",
		"ch04-s11-aura-kingship.md", "ch04-s12-jinnan-crisis.md", "ch04-s13-ordos-shimao.md",
		"ch04-s14-ordos-exchange.md", "ch04-s15a-sichuan-and-huai.md", "ch04-s15b-yuhui-and-coast.md",
		"ch04-s16-yangzi-transition.md", "ch04-s17-landscape-conclusion.md", "ch05-s01-longshan-collapse-erlitou.md",
		"ch05-s02-luoyang-intensification.md", "ch05-s03-erlitou-rise-palaces.md", "ch05-s04-stockbreeding-sound-color.md",
		"ch05-s05-jade-hybrid-ceramics.md", "ch05-s06-erlitou-bronze-industry.md", "ch05-s07-consumption-apparatus.md",
		"ch05-s08-magical-aura-metal.md", "ch05-s09-erlitou-political-network.md", "ch05-s10-erlitou-peers-conclusion.md",
		"ch05-s11-becoming-central-plains.md", "ch06-s01-shang-material-henei.md", "ch06-s02-luoyang-infiltration.md",
		"ch06-s03-forging-new-identity.md", "ch06-s04-zhengzhou-urbanism.md", "ch07-s07-reconfiguring-shang-landscape.md",
		"ch08-s02-ritual-bronzes-craft.md", "ch09-s01-emergence-transformation.md", "ch09-s02-epilogue-notes.md",
	}
	docs := []workspace.DocumentInfo{
		{WorkspaceRoot: root, ID: "t1", Name: "ch07-s07-reconfiguring-shang-landscape.md", Path: root + "/ch07-s07-reconfiguring-shang-landscape.md", ViewMode: "preview"},
		{WorkspaceRoot: root, ID: "t2", Name: "ch09-s01-emergence-transformation.md", Path: root + "/ch09-s01-emergence-transformation.md", ViewMode: "preview"},
		{WorkspaceRoot: root, ID: "active", Name: "ch05-s05-jade-hybrid-ceramics.md", Path: root + "/ch05-s05-jade-hybrid-ceramics.md", ViewMode: "genui"},
	}
	entries := make([]workspace.NavEntry, 0, len(names))
	for i, name := range names {
		docID := ""
		switch name {
		case "ch07-s07-reconfiguring-shang-landscape.md":
			docID = "t1"
		case "ch09-s01-emergence-transformation.md":
			docID = "t2"
		case "ch05-s05-jade-hybrid-ceramics.md":
			docID = "active"
		}
		entries = append(entries, workspace.NavEntry{
			ID: "f" + strings.Repeat("0", 2-len(name)%2) + name, Kind: "file", Name: name, Path: root + "/" + name,
			Parent: "sections", Depth: 0, DocumentID: docID,
		})
		_ = i
	}
	return nativeui.Model{
		Feynman: true, Git: true,
		Crumbs: workspace.Breadcrumbs(root, root+"/ch05-s05-jade-hybrid-ceramics.md"),
		Workspace: workspace.WorkspaceSnapshot{
			ID: "w-dense", Root: root, Name: "sections", Kind: "folder", ActiveID: "active", Documents: docs,
			Workspaces: []workspace.WorkspaceSummary{{ID: "w-dense", Root: root, Name: "sections"}},
		},
		Navigation: workspace.NavSnapshot{Root: root, Total: len(entries), Indexed: len(entries), Entries: entries},
		Sections:   workspace.ExtractSections(sceneDocument),
	}
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
			if sc.name == "dense_sidebar_genui_failure_light" {
				model = denseSidebarSceneModel()
			}
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
			space := portrait.DefaultSpace
			if named, ok := portrait.SpaceNamed(sc.space); ok {
				space = named
			}
			// Every scene draws production surfaces: the dialogs come from
			// Window.drawDialog, the review panels from nativereview's own
			// draw code, and the reading controls from nativegenui's.
			sceneWindow := nativeui.SceneWindow(model.Workspace, model.Navigation, sceneFolders(), sourceText, space, sc.dark)
			if sc.name == "dense_sidebar_genui_failure_light" || sc.name == "feynman_working_light" {
				p.window = sceneWindow
				p.state.root = model.Workspace.Root
			}
			reviewScene := nativereview.NewScene(sceneWindow, sceneReview())
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
				Settings: p.settingsPages(),
			}
			switch {
			case sc.dialog != "":
				if !sceneWindow.SceneDialog(sc.dialog) {
					t.Fatalf("scene dialog %q is not a production dialog", sc.dialog)
				}
			case strings.HasPrefix(sc.name, "proposals_"):
				reviewScene.OpenProposals()
				slots.Review = reviewScene.Proposals
			case strings.HasPrefix(sc.name, "checkpoints_"):
				reviewScene.ShowCheckpointList()
				slots.Checkpoints = reviewScene.Checkpoints
			case strings.HasPrefix(sc.name, "diff_"):
				slots.Checkpoints = reviewScene.Checkpoints
			case strings.HasPrefix(sc.name, "genui_controls_"):
				reading, err := nativegenui.SceneReading(sceneStatements)
				if err != nil {
					t.Fatalf("genui statements: %v", err)
				}
				reading.SelectItem = func(nativegenui.Node, string) {}
				reading.CanSelect = func(nativegenui.Node) bool { return true }
				slots.GenUI = func(c *ui.Context) { reading.View(c) }
			case sc.name == "genui_request_light":
				request := "Create a calm reading view with a summary and key principles."
				slots.GenUI = func(c *ui.Context) { nativegenui.SceneRequest(c, request) }
			case sc.name == "dense_sidebar_genui_failure_light":
				slots.GenUI = func(c *ui.Context) {
					nativegenui.SceneFailure(c, "The agent is not signed in.", "Run `agy acp --login` in Terminal, then retry.", "Authentication required")
				}
			case strings.HasPrefix(sc.name, "settings_genui_"):
				settings := workspace.DefaultSettings()
				settings.GenUI.Backends[genui.BackendAntigravity] = workspace.GenUIBackend{
					Command: "feynman",
					Args:    []string{"genui", "--json"},
					Model:   "claude-sonnet-4-5",
					Models:  []string{"claude-sonnet-4-5", "gpt-5.4", "gemini-3-pro"},
				}
				slots.Settings = append(slots.Settings, nativegenui.SceneSettingsPage(settings))
			}
			app := ui.NewTester(func(c *ui.Context) {
				c.SetMenuPresentation(ui.PopoverMenus)
				shell.Draw(c, model, sceneActions(&model), slots)
				sceneWindow.DrawDialog(c)
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
			// Each production surface must show a control only it has, so a
			// scene cannot pass with a stand-in panel.
			for _, want := range sceneWitnesses(sc) {
				if _, ok := app.Find(want); !ok {
					t.Fatalf("scene %s does not show its production control %q", sc.name, want)
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

// sceneWitnesses names the control a surface is known by.
func sceneWitnesses(sc shellScene) []string {
	switch {
	case sc.dialog == "unsaved_single":
		return []string{"Don't Save"}
	case sc.dialog == "unsaved_multi":
		return []string{"Discard All"}
	case sc.dialog == "conflict_decision":
		return []string{"Save Anyway", "Revert to Disk"}
	case sc.dialog == "readonly_decision":
		return []string{"Save As…"}
	case sc.dialog == "rename_prompt":
		return []string{"File name"}
	case sc.dialog == "move_picker":
		return []string{"Choose another folder…"}
	case sc.dialog == "revert_confirm":
		return []string{"Revert"}
	case sc.dialog == "quit_failed":
		return []string{"Dismiss alert"}
	case strings.HasPrefix(sc.name, "proposals_"):
		return []string{"Apply decisions", "Dismiss proposal"}
	case strings.HasPrefix(sc.name, "checkpoints_"):
		return []string{"Create checkpoint"}
	case strings.HasPrefix(sc.name, "diff_"):
		return []string{"@@ -3,3 +3,4 @@"}
	case strings.HasPrefix(sc.name, "genui_controls_"):
		return []string{"Reading width", "Choose…"}
	case sc.name == "genui_request_light":
		return []string{"Generate"}
	case sc.name == "dense_sidebar_genui_failure_light":
		return []string{"Retry", "Generation details", "Include doc"}
	case sc.name == "feynman_working_light":
		return []string{"Steer", "Follow up", "Stop", "Include doc"}
	case sc.name == "feynman_files_empty_light":
		return []string{"No research files yet"}
	case sc.name == "settings_mcp_add_light":
		return []string{"Project-local"}
	case sc.name == "settings_search_dark":
		return []string{"Search & Papers", "Web search"}
	case strings.HasPrefix(sc.name, "settings_genui_"):
		return []string{"Agent command"}
	}
	return nil
}
