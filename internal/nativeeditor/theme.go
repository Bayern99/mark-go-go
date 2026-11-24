package nativeeditor

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// documentTheme is the shared syntax palette with the overrides a document
// adds. The shared tokens keep portrait.CodeTheme's colors, so a code fence
// reads the same in the editor and in chat.
func documentTheme(t *ui.Theme, mode Mode) *editor.Theme {
	th := *portrait.CodeTheme(t)
	th.Background, th.Text, th.Cursor, th.Selection = t.Background, t.Text, t.Accent, t.Selection
	// The accent and warning tones follow the window theme, which the shell
	// sets from the Space; every other token keeps portrait.CodeTheme's.
	colorTokens(&th, t.Accent, editor.TokenKeyword, editor.TokenString, editor.TokenCode, editor.TokenTag)
	colorTokens(&th, t.Warning, editor.TokenNumber, editor.TokenConstant)
	th.Tokens[editor.TokenDeleted].Color = t.TextMuted
	th.Tokens[editor.TokenType].Color = portrait.Colors.Ink2
	if mode == Preview {
		th.Tokens[editor.TokenCode].Family = portrait.FontMono
	}
	if mode == Markdown {
		colorTokens(&th, portrait.Colors.Ink4, editor.TokenMeta, editor.TokenPunctuation, editor.TokenOperator)
		th.Tokens[editor.TokenInvalid].Color = t.Danger
	}
	return &th
}

func colorTokens(th *editor.Theme, color ui.Color, kinds ...editor.TokenKind) {
	for _, kind := range kinds {
		th.Tokens[kind].Color = color
	}
}
