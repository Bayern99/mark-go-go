package main

import (
	"context"
	"io"
	"log"
	"time"

	"github.com/egoist/mygo"
)

var documentAutosave = mygo.AutosaveInPlace

func main() {
	host := newDocumentHost()
	configureReadBench(host)
	mygo.App.OnBeforeQuit(host.onBeforeQuit)

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
			return host.openWindow(doc)
		},
		Command: func(role mygo.MenuRole, doc *mygo.Document[string], win *mygo.Window) bool {
			return host.interceptCommand(role, doc, win)
		},
		DocumentFocused: host.documentFocused,
		PathChange:      host.pathChangeLock,
		ReservePath:     host.reservePath,
		BeforeWindowClose: func(win *mygo.Window) bool {
			return host.interceptWindowClose(win)
		},
		AfterStart: host.restoreAfterStart,
	})
	host.setDocuments(ds)
	mygo.BindAs("WorkspaceHost", host)
	research := newResearchHost(host)
	mygo.BindAs("ResearchHost", research)
	mygo.BindAs("CheckpointHost", &checkpointHost{documents: host})

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
		applyNativeEditMenu(false, false)
	})
	err := mygo.App.Run()
	research.close()
	host.waitForGenerations(5 * time.Second)
	host.closeAgentCache()
	if err != nil {
		log.Fatal(err)
	}
}
