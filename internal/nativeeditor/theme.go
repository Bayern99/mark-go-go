package nativeeditor

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func documentTheme(t *ui.Theme, mode Mode) *editor.Theme {
	th := *portrait.CodeTheme(t)
	th.Background, th.Text, th.Cursor, th.Selection = t.Background, t.Text, t.Accent, t.Selection
	for i := range th.Tokens {
		th.Tokens[i].Color = t.Text
	}
	th.Tokens[editor.TokenHeading].Bold = true
	th.Tokens[editor.TokenStrong].Bold = true
	th.Tokens[editor.TokenEmphasis].Italic = true
	th.Tokens[editor.TokenLink].Color = t.Accent
	th.Tokens[editor.TokenDeleted].Color = t.TextMuted
	th.Tokens[editor.TokenType].Color = portrait.Colors.Ink2
	if mode == Preview {
		th.Tokens[editor.TokenCode].Family = portrait.FontMono
	}
	if mode == Preview || mode == Markdown {
		colorTokens(&th, t.Accent, editor.TokenKeyword, editor.TokenString)
		colorTokens(&th, t.Warning, editor.TokenNumber, editor.TokenConstant)
		th.Tokens[editor.TokenComment] = editor.TokenStyle{Color: t.TextMuted, Italic: true}
	}
	if mode == Markdown {
		colorTokens(&th, portrait.Colors.Ink4, editor.TokenMeta, editor.TokenPunctuation, editor.TokenOperator)
		colorTokens(&th, t.Accent, editor.TokenCode, editor.TokenTag)
		th.Tokens[editor.TokenInvalid].Color = t.Danger
	}
	return &th
}

func colorTokens(th *editor.Theme, color ui.Color, kinds ...editor.TokenKind) {
	for _, kind := range kinds {
		th.Tokens[kind].Color = color
	}
}
