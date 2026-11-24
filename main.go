package main

import (
	"context"
	"io"
	"log"
	"path/filepath"
	"time"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/desktop"
	"github.com/Bayern99/mark-go-go/internal/mcp"
	"github.com/Bayern99/mark-go-go/internal/nativefeynman"
	"github.com/Bayern99/mark-go-go/internal/nativegenui"
	"github.com/Bayern99/mark-go-go/internal/nativereview"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo"
)

var documentAutosave = mygo.AutosaveInPlace

type workspaceAdapter struct{ svc *workspace.Service }

func (a workspaceAdapter) WindowWorkspace(ctx context.Context, root string) (feynman.WorkspaceRef, error) {
	ref, err := a.svc.WindowWorkspace(ctx, root)
	if err != nil {
		return feynman.WorkspaceRef{}, err
	}
	return feynman.WorkspaceRef{Root: ref.Root, Window: ref.Window}, nil
}
func (a workspaceAdapter) EditableDocument(ctx context.Context, root, documentID string) error {
	return a.svc.EditableDocument(ctx, root, documentID)
}
func (a workspaceAdapter) UserDataDir() string { return a.svc.UserDataDir() }
func (a workspaceAdapter) CheckpointDocument(ctx context.Context, documentID, path string) (review.CheckpointDoc, error) {
	doc, err := a.svc.CheckpointDocument(ctx, documentID, path)
	if err != nil {
		return review.CheckpointDoc{}, err
	}
	return review.CheckpointDoc{WorkspaceRoot: doc.WorkspaceRoot, Path: doc.Path, Content: doc.Content, Edited: doc.Edited, Conflict: doc.Conflict, Deleted: doc.Deleted}, nil
}

func main() {
	host := workspace.NewService()
	configureReadBench(host)
	mygo.App.OnBeforeQuit(host.OnBeforeQuit)
	ds := mygo.NewDocuments(mygo.DocumentsOptions[string]{
		Types: []mygo.DocumentType{{Name: "Markdown Document", Extensions: []string{"md", "markdown"}}},
		Read: func(r io.Reader) (string, error) {
			b, err := io.ReadAll(r)
			return string(b), err
		},
		Write: func(w io.Writer, text string) error { _, err := io.WriteString(w, text); return err },
		Equal: func(a, b string) bool { return a == b }, Autosave: documentAutosave, NoUntitled: true,
		Window: func(doc *mygo.Document[string]) *mygo.Window { return host.OpenWindow(doc) },
		Command: func(role mygo.MenuRole, doc *mygo.Document[string], win *mygo.Window) bool {
			return host.InterceptCommand(role, doc, win)
		},
		DocumentFocused: host.DocumentFocused, PathChange: host.PathChangeLock, ReservePath: host.ReservePath,
		BeforeWindowClose: func(win *mygo.Window) bool { return host.InterceptWindowClose(win) },
		AfterStart:        host.RestoreAfterStart,
	})
	host.SetDocuments(ds)
	adapter := workspaceAdapter{svc: host}
	reviewSvc := review.NewService()
	research := feynman.NewResearchManager(adapter)
	host.CreateWindow = func(opts mygo.WindowOptions) *mygo.Window {
		return nativeui.NewWindow(host, opts, nativegenui.Install(host), nativereview.Install(host, reviewSvc, review.NewCheckpointHost(adapter)), nativefeynman.NewExtension(research.Host(), reviewSvc))
	}
	mcpServer := mcp.NewServer(mcp.Dependencies{Workspace: host, Review: reviewSvc})
	research.SetBeforeStart(mcpServer.BeforeResearchStart)
	research.SetAfterStop(mcpServer.AfterResearchStop)
	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(buildMenu(host))
		workspace.ApplyNativeEditMenu(false, false)
	})
	err := mygo.App.Run()
	research.Close()
	_ = mcpServer.Close()
	host.WaitForGenerations(5 * time.Second)
	host.WaitForSessionWrites(2 * time.Second)
	host.CloseAgentCache()
	if err != nil {
		log.Fatal(err)
	}
}

func buildMenu(host *workspace.Service) *mygo.Menu {
	withWindow := func(run func(*nativeui.Window)) func(*mygo.MenuItem, *mygo.Window) {
		return func(_ *mygo.MenuItem, win *mygo.Window) {
			if w := nativeui.WindowFor(win); w != nil {
				run(w)
			}
		}
	}
	recent := make([]*mygo.MenuItem, 0, 10)
	for _, path := range mygo.App.RecentDocuments() {
		path := path
		recent = append(recent, &mygo.MenuItem{Label: filepath.Base(path), Click: withWindow(func(w *nativeui.Window) { w.OpenRecentDocument(path) })})
	}
	if len(recent) == 0 {
		recent = append(recent, &mygo.MenuItem{Label: "No Recent Documents", Disabled: true})
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout}, mygo.Separator(),
			{Label: "Settings…", Accelerator: "CmdOrCtrl+,", Click: withWindow(func(w *nativeui.Window) { w.OpenSettings("") })},
			mygo.Separator(), {Role: mygo.RoleServices}, mygo.Separator(), {Role: mygo.RoleHide}, {Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide}, mygo.Separator(), {Role: mygo.RoleQuit},
		}},
		{Role: mygo.RoleFileMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleNew}, {Label: "Open…", Accelerator: "CmdOrCtrl+O", Click: withWindow(func(w *nativeui.Window) { w.OpenFiles() })},
			{Label: "Open Folder…", Accelerator: "CmdOrCtrl+Shift+O", Click: func(_ *mygo.MenuItem, win *mygo.Window) {
				go func() { _, _ = host.PickFolder(desktop.WithWindow(context.Background(), win)) }()
			}},
			{Label: "Open Recent", Submenu: recent}, mygo.Separator(), {Role: mygo.RoleClose}, {Role: mygo.RoleSave}, {Role: mygo.RoleSaveAs}, {Role: mygo.RoleDuplicate}, {Role: mygo.RoleRename}, {Role: mygo.RoleMoveTo}, {Role: mygo.RoleRevert},
		}},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleViewMenu, Submenu: []*mygo.MenuItem{
			{Label: "Command Bar…", Accelerator: "CmdOrCtrl+K", Click: withWindow(func(w *nativeui.Window) { w.OpenCommandBar() })}, mygo.Separator(),
			{Label: "Preview", Type: mygo.MenuItemRadio, Checked: true, Click: withWindow(func(w *nativeui.Window) { w.SetMode("preview") })},
			{Label: "Markdown", Type: mygo.MenuItemRadio, Accelerator: "CmdOrCtrl+/", Click: withWindow(func(w *nativeui.Window) { w.SetMode("markdown") })},
			{Label: "GenUI", Type: mygo.MenuItemRadio, Click: withWindow(func(w *nativeui.Window) { w.SetMode("genui") })},
			mygo.Separator(),
			{Label: "Toggle Sidebar", Accelerator: "CmdOrCtrl+0", Click: withWindow(func(w *nativeui.Window) { w.ToggleSidebar() })},
			{Label: "Toggle Research", Accelerator: "CmdOrCtrl+Alt+0", Click: withWindow(func(w *nativeui.Window) { w.ToggleResearch() })},
			mygo.Separator(),
			{Label: "Appearance…", Click: withWindow(func(w *nativeui.Window) { w.OpenSettings("appearance") })},
			{Label: "Checkpoints…", Click: withWindow(func(w *nativeui.Window) { w.OpenCheckpoints() })},
			mygo.Separator(), {Role: mygo.RoleToggleFullScreen},
		}},
		{Role: mygo.RoleWindowMenu},
		{Label: "Help", Submenu: []*mygo.MenuItem{{Label: "Keyboard Shortcuts", Click: withWindow(func(w *nativeui.Window) { w.OpenSettings("shortcuts") })}}},
	})
}
