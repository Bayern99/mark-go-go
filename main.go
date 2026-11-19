package main

import (
	"context"
	"io"
	"log"
	"time"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
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
	mygo.BindAs("WorkspaceHost", workspace.NewHost(host))
	mygo.BindAs("ReviewHost", reviewSvc.Host())
	research := feynman.NewResearchManager(adapter)
	mygo.BindAs("ResearchHost", research.Host())
	mygo.BindAs("CheckpointHost", review.NewCheckpointHost(adapter))

	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Role: mygo.RoleFileMenu, Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleNew},
				{Role: mygo.RoleOpen},
				{Label: "Open Folder…", Accelerator: "CmdOrCtrl+Shift+O", Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
					go func() {
						_, _ = host.PickFolder(context.Background())
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
	host.WaitForGenerations(5 * time.Second)
	host.WaitForSessionWrites(2 * time.Second)
	host.CloseAgentCache()
	if err != nil {
		log.Fatal(err)
	}
}
