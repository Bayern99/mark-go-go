package main

import (
	"context"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/desktop"
	"github.com/Bayern99/mark-go-go/internal/ipc"
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

type workspaceAdapter struct {
	svc *workspace.Service
}

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

func (a workspaceAdapter) UserDataDir() string {
	return a.svc.UserDataDir()
}

func (a workspaceAdapter) CheckpointDocument(ctx context.Context, documentID, path string) (review.CheckpointDoc, error) {
	doc, err := a.svc.CheckpointDocument(ctx, documentID, path)
	if err != nil {
		return review.CheckpointDoc{}, err
	}
	return review.CheckpointDoc{
		WorkspaceRoot: doc.WorkspaceRoot,
		Path:          doc.Path,
		Content:       doc.Content,
		Edited:        doc.Edited,
		Conflict:      doc.Conflict,
		Deleted:       doc.Deleted,
	}, nil
}

func main() {
	// The markgogo CLI resolves the same user data folder, so an override has
	// to move the app with it.
	if dir := strings.TrimSpace(os.Getenv(ipc.UserDataEnv)); dir != "" {
		mygo.App.SetPath(mygo.PathUserData, dir)
	}
	host := workspace.NewService()
	configureReadBench(host)
	mygo.App.OnBeforeQuit(host.OnBeforeQuit)

	ds := mygo.NewDocuments(mygo.DocumentsOptions[string]{
		Types: []mygo.DocumentType{
			{Name: "Markdown Document", Extensions: []string{"md", "markdown"}},
		},
		Read: func(r io.Reader) (string, error) {
			b, err := io.ReadAll(r)
			return string(b), err
		},
		Write: func(w io.Writer, text string) error {
			_, err := io.WriteString(w, text)
			return err
		},
		Equal:      func(a, b string) bool { return a == b },
		Autosave:   documentAutosave,
		NoUntitled: true,
		Window: func(doc *mygo.Document[string]) *mygo.Window {
			return host.OpenWindow(doc)
		},
		Command: func(role mygo.MenuRole, doc *mygo.Document[string], win *mygo.Window) bool {
			return host.InterceptCommand(role, doc, win)
		},
		DocumentFocused: host.DocumentFocused,
		PathChange:      host.PathChangeLock,
		ReservePath:     host.ReservePath,
		BeforeWindowClose: func(win *mygo.Window) bool {
			return host.InterceptWindowClose(win)
		},
		AfterStart: host.RestoreAfterStart,
	})
	host.SetDocuments(ds)
	adapter := workspaceAdapter{svc: host}
	reviewSvc := review.NewService()
	research := feynman.NewResearchManager(adapter)
	host.CreateWindow = func(opts mygo.WindowOptions) *mygo.Window {
		return nativeui.NewWindow(host, opts,
			nativegenui.Install(host),
			nativereview.Install(host, reviewSvc, review.NewCheckpointHost(adapter)),
			nativefeynman.NewExtension(research.Host(), reviewSvc),
		)
	}
	mcpServer := mcp.NewServer(mcp.Dependencies{
		Workspace: host,
		Review:    reviewSvc,
	})
	research.SetBeforeStart(mcpServer.BeforeResearchStart)
	research.SetAfterStop(mcpServer.AfterResearchStop)

	externalServer := mcp.NewExternalServer(mcp.Dependencies{
		Workspace: host,
		Review:    reviewSvc,
	})
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
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Role: mygo.RoleFileMenu, Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleNew},
				{Role: mygo.RoleOpen},
				{Label: "Open Folder…", Accelerator: "CmdOrCtrl+Shift+O", Click: func(_ *mygo.MenuItem, win *mygo.Window) {
					go func() {
						_, _ = host.PickFolder(desktop.WithWindow(context.Background(), win))
					}()
				}},
				{Role: mygo.RoleOpenRecent},
				mygo.Separator(),
				{Role: mygo.RoleClose},
				{Role: mygo.RoleSave},
				{Role: mygo.RoleSaveAs},
				{Role: mygo.RoleDuplicate},
				{Role: mygo.RoleRename},
				{Role: mygo.RoleMoveTo},
				{Role: mygo.RoleRevert},
			}},
			{Role: mygo.RoleEditMenu},
			{Role: mygo.RoleViewMenu},
			{Role: mygo.RoleWindowMenu},
		}))
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
