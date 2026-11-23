package nativeeditor

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestCopyCodeButtonSkipsEmptyBlockAndNotifiesOnWrite(t *testing.T) {
	body := ""
	var services ui.Services
	app := ui.NewTester(func(c *ui.Context) {
		services = c.Services()
		ui.Column(c).Fill().Children(func() {
			copyButton(c, body)
		})
		portrait.Toasts(c)
	}, 400, 200)

	app.Frame()
	services.WriteClipboard("keep me")
	if err := app.Click("Copy code"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if got := services.ReadClipboard(); got != "keep me" {
		t.Fatalf("empty code block overwrote clipboard with %q", got)
	}
	if app.HasText("Copied") {
		t.Fatal("empty code block showed Copied toast")
	}

	body = "fmt.Println(1)"
	app.Frame()
	if err := app.Click("Copy code"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if got := services.ReadClipboard(); got != body {
		t.Fatalf("clipboard = %q, want %q", got, body)
	}
	if !app.HasText("Copied") {
		t.Fatal("non-empty code block copy did not show Copied toast")
	}
}
