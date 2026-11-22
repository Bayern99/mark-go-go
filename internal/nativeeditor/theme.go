package nativeeditor

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func documentTheme(t *ui.Theme, mode Mode) *editor.Theme {
	th := editor.Theme{Background: t.Background, Text: t.Text, Cursor: t.Accent, Selection: t.Selection}
	for i := range th.Tokens {
		th.Tokens[i].Color = t.Text
	}
	th.Tokens[editor.TokenHeading].Bold = true
	th.Tokens[editor.TokenStrong].Bold = true
	th.Tokens[editor.TokenEmphasis].Italic = true
	th.Tokens[editor.TokenLink].Color = t.Accent
	th.Tokens[editor.TokenDeleted].Color = t.TextMuted
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
