package main

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/desktop"
	"github.com/Bayern99/mark-go-go/internal/ipc"
	"github.com/Bayern99/mark-go-go/internal/mcp"
	"github.com/Bayern99/mark-go-go/internal/nativeai"
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
	// The markgogo CLI resolves the same user data folder, so an override has
	// to move the app with it.
	if dir := strings.TrimSpace(os.Getenv(ipc.UserDataEnv)); dir != "" {
		mygo.App.SetPath(mygo.PathUserData, dir)
	}
	host := workspace.NewService()
	host.SetAutosave(documentAutosave)
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
	externalServer := mcp.NewExternalServer(mcp.Dependencies{
		Workspace: host,
		Review:    reviewSvc,
	})
	host.CreateWindow = func(opts mygo.WindowOptions) *mygo.Window {
		return nativeui.NewWindow(host, opts, nativegenui.Install(host), nativereview.Install(host, reviewSvc, review.NewCheckpointHost(adapter)), nativefeynman.NewExtension(research.Host(), reviewSvc), nativeai.Install(host, nativeai.Channel{
			Listening: externalServer.Listening,
			LastCall:  externalServer.LastCall,
		}))
	}
	mcpServer := mcp.NewServer(mcp.Dependencies{Workspace: host, Review: reviewSvc})
	research.SetBeforeStart(mcpServer.BeforeResearchStart)
	research.SetAfterStop(mcpServer.AfterResearchStop)
	nativeui.AfterWorkspaceChange = func() { refreshMenu(host) }
	nativeui.AfterGenUIPluginChange = func(enabled bool) { updateGenUIMenu(mygo.App.Menu(), enabled) }

	applyExternalIPC := func(s workspace.Settings) {
		if !s.ExternalIPC {
			if err := externalServer.Close(); err != nil {
				log.Printf("markgogo: stop the local MCP channel: %v", err)
			}
			return
		}
		if err := externalServer.Start(); err != nil {
			log.Printf("markgogo: start the local MCP channel: %v", err)
			return
		}
		log.Printf("markgogo: the local MCP channel listens at %s", externalServer.SocketPath())
	}
	host.OnSettingsChanged(applyExternalIPC)
	if settings, err := host.GetSettings(); err == nil {
		applyExternalIPC(settings)
	}

	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(buildMenu(host, nativeui.ProfilePluginEnabled(host.UserDataDir(), "genui")))
		workspace.ApplyNativeEditMenu(false, false)
	})
	err := mygo.App.Run()
	research.Close()
	_ = mcpServer.Close()
	_ = externalServer.Close()
	host.WaitForGenerations(5 * time.Second)
	host.WaitForSessionWrites(2 * time.Second)
	host.CloseAgentCache()
	if err != nil {
		log.Fatal(err)
	}
}

// menuRecents is the recent-document list the current menu was built from, so
// a workspace change rebuilds the app's own Open Recent submenu only when MyGo
// has a new list: MyGo refreshes role-based submenus, and this one keeps its
// errors inside the Portrait window.
var menuRecents struct {
	sync.Mutex
	paths []string
	built bool
}

// refreshMenu rebuilds the menu when the recent documents changed.
func refreshMenu(host *workspace.Service) {
	if host == nil || !mygo.App.IsReady() {
		return
	}
	paths := mygo.App.RecentDocuments()
	menuRecents.Lock()
	same := menuRecents.built && slices.Equal(paths, menuRecents.paths)
	menuRecents.paths, menuRecents.built = slices.Clone(paths), true
	menuRecents.Unlock()
	if same {
		return
	}
	mygo.App.SetMenu(buildMenu(host, nativeui.ProfilePluginEnabled(host.UserDataDir(), "genui")))
	nativeui.ResetAllMenuStates()
}

// recentItems lists the recent documents, keeping duplicate file names apart
// by their folders, and ends with Clear Recent as AppKit's menu does.
func recentItems(host *workspace.Service) []*mygo.MenuItem {
	paths := mygo.App.RecentDocuments()
	menuRecents.Lock()
	menuRecents.paths, menuRecents.built = slices.Clone(paths), true
	menuRecents.Unlock()
	labels := recentLabels(paths)
	items := make([]*mygo.MenuItem, 0, len(paths)+2)
	for i, path := range paths {
		path, label := path, labels[i]
		items = append(items, &mygo.MenuItem{Label: label, ToolTip: path, Click: func(_ *mygo.MenuItem, win *mygo.Window) {
			if w := portraitWindow(win); w != nil {
				w.OpenRecentDocument(path)
				return
			}
			go func() {
				if _, err := host.Open(desktop.WithWindow(context.Background(), win), path); err != nil {
					log.Printf("markgogo: open recent: %v", err)
				}
			}()
		}})
	}
	if len(items) == 0 {
		return []*mygo.MenuItem{{Label: "No Recent Documents", Disabled: true}}
	}
	return append(items, mygo.Separator(), &mygo.MenuItem{Label: "Clear Recent", Click: func(*mygo.MenuItem, *mygo.Window) {
		if err := mygo.App.ClearRecentDocuments(); err != nil {
			log.Printf("markgogo: clear recent documents: %v", err)
		}
		go refreshMenu(host)
	}})
}

// recentLabels names the recent documents in a menu: each by its file name,
// and one whose name another shares by the folders above it, one folder at a
// time, until the names differ. work/notes/draft.md and personal/notes/draft.md
// are therefore "notes/draft.md" no more: the first is told apart by the folder
// above the one they share. A document no folder can tell apart keeps its whole
// path.
func recentLabels(paths []string) []string {
	parts := make([][]string, len(paths))
	depth := make([]int, len(paths))
	for i, path := range paths {
		cleaned := filepath.ToSlash(filepath.Clean(path))
		trimmed := strings.TrimPrefix(cleaned, "/")
		if trimmed == "" || strings.HasSuffix(trimmed, "/") {
			parts[i] = []string{cleaned}
		} else {
			parts[i] = strings.Split(trimmed, "/")
		}
		depth[i] = 1
	}
	labels := make([]string, len(paths))
	for {
		shared := map[string]int{}
		for i := range paths {
			labels[i] = strings.Join(parts[i][len(parts[i])-depth[i]:], "/")
			shared[labels[i]]++
		}
		grew := false
		for i := range paths {
			if shared[labels[i]] > 1 && depth[i] < len(parts[i]) {
				depth[i]++
				grew = true
			}
		}
		if !grew {
			break
		}
	}
	shared := map[string]int{}
	for _, label := range labels {
		shared[label]++
	}
	for i, label := range labels {
		if shared[label] > 1 {
			labels[i] = paths[i]
		}
	}
	return labels
}

// portraitWindow is the Portrait window a menu command belongs to: the
// command's own window when it is registered, else the front one, so Open
// still works for a window whose Portrait layer has not registered yet.
func portraitWindow(win *mygo.Window) *nativeui.Window {
	if w := nativeui.WindowFor(win); w != nil {
		return w
	}
	return nativeui.FrontWindow()
}

const genUIMenuID = "view.genui"

func updateGenUIMenu(menu *mygo.Menu, enabled bool) {
	if menu != nil {
		if item := menu.ItemByID(genUIMenuID); item != nil && item.IsEnabled() != enabled {
			item.SetEnabled(enabled)
		}
	}
}

func buildMenu(host *workspace.Service, genuiEnabled bool) *mygo.Menu {
	withWindow := func(run func(*nativeui.Window)) func(*mygo.MenuItem, *mygo.Window) {
		return func(_ *mygo.MenuItem, win *mygo.Window) {
			if w := portraitWindow(win); w != nil {
				run(w)
			}
		}
	}
	recent := recentItems(host)
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout}, mygo.Separator(),
			{Label: "Settings…", Accelerator: "CmdOrCtrl+,", Click: withWindow(func(w *nativeui.Window) { w.OpenSettings("") })},
			mygo.Separator(), {Role: mygo.RoleServices}, mygo.Separator(), {Role: mygo.RoleHide}, {Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide}, mygo.Separator(), {Role: mygo.RoleQuit},
		}},
		{Role: mygo.RoleFileMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleNew}, {Label: "Open…", Accelerator: "CmdOrCtrl+O", Click: func(_ *mygo.MenuItem, win *mygo.Window) {
				if w := portraitWindow(win); w != nil {
					w.OpenFiles()
					return
				}
				go func() {
					if err := host.OpenFiles(desktop.WithWindow(context.Background(), win)); err != nil && !errors.Is(err, mygo.ErrCanceled) {
						log.Printf("markgogo: open files: %v", err)
					}
				}()
			}},
			{Label: "Open Folder…", Accelerator: "CmdOrCtrl+Shift+O", Click: func(_ *mygo.MenuItem, win *mygo.Window) {
				go func() { _, _ = host.PickFolder(desktop.WithWindow(context.Background(), win)) }()
			}},
			{Label: "Open Recent", Submenu: recent}, mygo.Separator(), {Role: mygo.RoleClose}, {Role: mygo.RoleSave}, {Role: mygo.RoleSaveAs}, {Role: mygo.RoleDuplicate}, {Role: mygo.RoleRename}, {Role: mygo.RoleMoveTo}, {Role: mygo.RoleRevert},
		}},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleViewMenu, Submenu: []*mygo.MenuItem{
			{Label: "Command Bar…", Accelerator: "CmdOrCtrl+K", Click: withWindow(func(w *nativeui.Window) { w.OpenCommandBar() })}, mygo.Separator(),
			{Label: "Toggle Preview / Markdown", Accelerator: "CmdOrCtrl+E", Click: withWindow(func(w *nativeui.Window) { w.ToggleMode() })},
			{ID: genUIMenuID, Label: "GenUI", Disabled: !genuiEnabled, Type: mygo.MenuItemRadio, Click: withWindow(func(w *nativeui.Window) { w.SetMode("genui") })},
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
