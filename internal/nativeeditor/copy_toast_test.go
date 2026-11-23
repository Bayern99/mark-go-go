package nativeeditor

import (
	"errors"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestCopyCodeButtonSkipsEmptyAndFailedWrites(t *testing.T) {
	body := ""
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			copyButton(c, body)
		})
		portrait.Toasts(c)
	}, 400, 200)

	// Empty code block preserves existing clipboard and shows no toast.
	app.Frame()
	app.SetClipboard("keep me")
	if err := app.Click("Copy code"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if got := app.Clipboard(); got != "keep me" {
		t.Fatalf("empty code block overwrote clipboard with %q", got)
	}
	if app.HasText("Copied") {
		t.Fatal("empty code block showed Copied toast")
	}

	// Failed write must not show Copied even when clipboard already holds body.
	body = "fmt.Println(1)"
	app.SetClipboard(body)
	app.FailClipboard(errors.New("denied"))
	app.Frame()
	if err := app.Click("Copy code"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	app.Frame()
	if app.HasText("Copied") {
		t.Fatal("failed clipboard write with identical preexisting text showed Copied toast")
	}

	// Successful write shows Copied toast.
	app.FailClipboard(nil)
	app.SetClipboard("old")
	app.Frame()
	if err := app.Click("Copy code"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	app.Frame()
	if got := app.Clipboard(); got != body {
		t.Fatalf("clipboard = %q, want %q", got, body)
	}
	if !app.HasText("Copied") {
		t.Fatal("successful code block copy did not show Copied toast")
	}
}
