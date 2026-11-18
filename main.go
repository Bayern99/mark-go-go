// Command mark-go-go is a local-first Markdown editor for macOS.
//
// The Go side owns documents: MyGo reads and writes files, autosaves,
// keeps recovery copies and reports file state. A workspace window hosts
// several documents as tabs. See workspace.go and documenthost.go.
//
//	mygo dev     # or: go tool mygo dev
//	mygo build   # or: go tool mygo build
package main

import (
	"context"
	"io"
	"log"
	"time"

	"github.com/egoist/mygo"
)

// documentAutosave remains the production default. Explicit e2e builds turn
// it off so native Save tests cannot pass because an autosave won the race.
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
	host.waitForGenerations(5 * time.Second)
	host.closeAgentCache()
	if err != nil {
		log.Fatal(err)
	}
}
