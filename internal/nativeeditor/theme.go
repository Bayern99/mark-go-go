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
		for _, kind := range []editor.TokenKind{editor.TokenKeyword, editor.TokenString} {
			th.Tokens[kind].Color = t.Accent
		}
		for _, kind := range []editor.TokenKind{editor.TokenNumber, editor.TokenConstant} {
			th.Tokens[kind].Color = t.Warning
		}
		th.Tokens[editor.TokenComment] = editor.TokenStyle{Color: t.TextMuted, Italic: true}
	}
	if mode == Markdown {
		for _, kind := range []editor.TokenKind{editor.TokenMeta, editor.TokenPunctuation, editor.TokenOperator} {
			th.Tokens[kind].Color = portrait.Colors.Ink4
		}
		for _, kind := range []editor.TokenKind{editor.TokenCode, editor.TokenKeyword, editor.TokenString, editor.TokenTag} {
			th.Tokens[kind].Color = t.Accent
		}
		for _, kind := range []editor.TokenKind{editor.TokenNumber, editor.TokenConstant} {
			th.Tokens[kind].Color = t.Warning
		}
		th.Tokens[editor.TokenInvalid].Color = t.Danger
		th.Tokens[editor.TokenComment] = editor.TokenStyle{Color: t.TextMuted, Italic: true}
	}
	return &th
}
