package nativeeditor

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestDocumentThemeKeepsSharedTypeTokenColor(t *testing.T) {
	window := &ui.Theme{}
	for _, mode := range []Mode{Preview, Markdown} {
		got := documentTheme(window, mode).Tokens[editor.TokenType].Color
		want := portrait.CodeTheme(window).Tokens[editor.TokenType].Color
		if got != want || got != portrait.Colors.Warn {
			t.Errorf("%s TokenType color = %v, want shared CodeTheme color %v", mode, got, want)
		}
	}
}
