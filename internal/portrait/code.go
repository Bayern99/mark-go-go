package portrait

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// CodeTheme is the shared syntax and editor-chrome palette for documents and
// chat. Keeping it here prevents the two Markdown renderers from drifting to
// different accent, diagnostic and diff colors.
func CodeTheme(t *ui.Theme) *editor.Theme {
	// A dark field needs a stronger tint for the same visible difference, so
	// the current find match stands apart from the other matches.
	match, currentMatch := float32(0.18), float32(0.45)
	if IsDark() {
		match, currentMatch = 0.22, 0.62
	}
	th := editor.Theme{
		Background: Colors.Paper, Text: Colors.Ink,
		LineNumber: Colors.Ink3, ActiveLineNumber: Colors.Ink,
		ActiveLine: Colors.Fill, Selection: t.Selection, InactiveSelection: t.Selection.Alpha(0.55), Cursor: Colors.Accent,
		MatchingBracket: Colors.LineStrong,
		Match:           Colors.Accent.Alpha(match), CurrentMatch: Colors.Accent.Alpha(currentMatch), Occurrence: Colors.Accent.Alpha(0.10),
		IndentGuide: Colors.Line, ActiveIndentGuide: Colors.LineStrong, WrapGuide: Colors.Line, Whitespace: Colors.Ink4,
		Error: Colors.Danger, Warning: Colors.Warn, Info: Colors.Accent, Hint: Colors.Ink3,
		Added: Colors.Success, Deleted: Colors.Danger, Modified: Colors.Accent,
	}
	for i := range th.Tokens {
		th.Tokens[i].Color = Colors.Ink
	}
	set := func(color ui.Color, kinds ...editor.TokenKind) {
		for _, kind := range kinds {
			th.Tokens[kind].Color = color
		}
	}
	set(Colors.Accent, editor.TokenKeyword, editor.TokenString, editor.TokenCode, editor.TokenTag)
	set(Colors.Warn, editor.TokenNumber, editor.TokenConstant, editor.TokenType, editor.TokenAttribute)
	set(Colors.Ink2, editor.TokenFunction, editor.TokenProperty, editor.TokenRegexp, editor.TokenEscape)
	set(Colors.Ink3, editor.TokenComment, editor.TokenPunctuation, editor.TokenOperator, editor.TokenMeta)
	set(Colors.Success, editor.TokenInserted)
	set(Colors.Danger, editor.TokenInvalid)
	th.Tokens[editor.TokenDeleted].Color = Colors.Ink3
	th.Tokens[editor.TokenHeading].Bold = true
	th.Tokens[editor.TokenStrong].Bold = true
	th.Tokens[editor.TokenEmphasis].Italic = true
	th.Tokens[editor.TokenLink].Color = Colors.Accent
	th.Tokens[editor.TokenComment].Italic = true
	return &th
}
