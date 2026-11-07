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
	"io"
	"log"
	"time"

	"github.com/egoist/mygo"
)

func main() {
	host := newDocumentHost()

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
		Equal: func(a, b string) bool { return a == b },
		Window: func(doc *mygo.Document[string]) *mygo.Window {
			return host.openWindow(doc)
		},
		Command: func(role mygo.MenuRole, doc *mygo.Document[string], win *mygo.Window) bool {
			return host.interceptCommand(role, doc, win)
		},
		BeforeWindowClose: func(win *mygo.Window) bool {
			return host.interceptWindowClose(win)
		},
	})
	host.setDocuments(ds)
	mygo.BindAs("WorkspaceHost", host)

	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Role: mygo.RoleFileMenu},
			{Role: mygo.RoleEditMenu},
			{Role: mygo.RoleViewMenu},
			{Role: mygo.RoleWindowMenu},
		}))
	})

	err := mygo.App.Run()
	host.waitForGenerations(5 * time.Second)
	if err != nil {
		log.Fatal(err)
	}
}
