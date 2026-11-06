// Command Mark Go Go is a local-first Markdown editor for macOS.
//
// The Go side owns the document: MyGo's document controller reads and
// writes the file, autosaves it, keeps recovery copies and reports the
// file's state. The web page edits the Markdown and sends the composed
// text back; see documenthost.go.
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

	mygo.NewDocuments(mygo.DocumentsOptions[string]{
		Types: []mygo.DocumentType{
			{Name: "Markdown Document", Extensions: []string{"md", "markdown"}},
		},
		// Read and write the bytes as they are: no newline or encoding
		// conversion, so a file keeps the line endings it has.
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
	})
	mygo.BindAs("DocumentHost", host)

	mygo.App.WhenReady(func() {
		// The standard menus: the document roles of File, and the editing
		// roles of Edit, whose keys reach the page's first responder.
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Role: mygo.RoleFileMenu},
			{Role: mygo.RoleEditMenu},
			{Role: mygo.RoleViewMenu},
			{Role: mygo.RoleWindowMenu},
		}))
	})

	err := mygo.App.Run()
	// Quitting closed every window, which cancelled their generations;
	// let them stop their agents and remove their folders before the
	// process exits, which would otherwise orphan the agents.
	host.waitForGenerations(5 * time.Second)
	if err != nil {
		log.Fatal(err)
	}
}
