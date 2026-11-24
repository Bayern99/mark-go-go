package nativefeynman

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
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

## 中文排版

界面以英文为主，正文同样照顾中文段落：汉字自动落到中文字体，行高与字间距保持
舒展，中英文混排（例如 Markdown 与 GenUI）不需要额外设置。

| Surface | Role | Material |
| --- | --- | --- |
| Scenery | Window field, sidebar | Tinted, translucent |
| Page | The document | Opaque paper |

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
		Feynman: feynmanOn, Git: true, Version: 3,
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
	for _, sc := range sceneList() {
		t.Run(sc.name, func(t *testing.T) {
			var shell nativeui.Shell
			model := sceneModel(sc.feynman, sc.mode)
			if sc.empty {
				model = nativeui.Model{Recents: []workspace.RecentWorkspace{{Name: "Field Notes", Root: "/Users/ada/Field Notes"}, {Name: "Lab", Root: "/Users/ada/Lab"}}}
			}
			source := nativeeditor.New(sceneDocument)
			source.SetMode(nativeeditor.Mode(sc.mode))
			p := scenePanel()
			slots := nativeui.Slots{
				Document: func(c *ui.Context) { source.View(c.Key("scene-doc"), false).Grow(1).MinHeight(0).FillWidth() },
				Research: func(c *ui.Context) {
					ui.Column(c.Key("scene-research")).Fill().MinWidth(0).Children(func() {
						p.header(c)
						p.tabs(c)
						p.chat(c)
					})
				},
				Checkpoints: func(c *ui.Context) {},
			}
			app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, sceneActions(&model), slots) }, sc.width, sc.height)
			app.SetScale(2)
			app.Frame()
			if sc.act != nil {
				sc.act(app)
			}
			app.Advance(2 * time.Second)
			app.Frame()
			file, err := os.Create(filepath.Join(dir, prefix+sc.name+".png"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if err := png.Encode(file, app.Image()); err != nil {
				t.Fatal(err)
			}
		})
	}
}
