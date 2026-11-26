package portrait

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// CodeTheme is the shared syntax and editor-chrome palette for documents and
// chat. Keeping it here prevents the two Markdown renderers from drifting to
// different accent, diagnostic and diff colors.
func CodeTheme(t *ui.Theme) *editor.Theme {
	return codeThemeFor(Colors, IsDark(), t.Selection)
}

func codeThemeFor(colors Palette, dark bool, selection ui.Color) *editor.Theme {
	// A dark field needs a stronger tint for the same visible difference, so
	// the current find match stands apart from the other matches.
	match, currentMatch := float32(0.18), float32(0.45)
	if dark {
		match, currentMatch = 0.22, 0.62
	}
	th := editor.Theme{
		Background: colors.Paper, Text: colors.Ink,
		LineNumber: colors.Ink3, ActiveLineNumber: colors.Ink,
		ActiveLine: colors.Fill, Selection: selection, InactiveSelection: selection.Alpha(0.55), Cursor: colors.Accent,
		MatchingBracket: colors.LineStrong,
		Match:           colors.Accent.Alpha(match), CurrentMatch: colors.Accent.Alpha(currentMatch), Occurrence: colors.Accent.Alpha(0.10),
		IndentGuide: colors.Line, ActiveIndentGuide: colors.LineStrong, WrapGuide: colors.Line, Whitespace: colors.Ink4,
		Error: colors.Danger, Warning: colors.Warn, Info: colors.Accent, Hint: colors.Ink3,
		Added: colors.Success, Deleted: colors.Danger, Modified: colors.Accent,
	}
	for i := range th.Tokens {
		th.Tokens[i].Color = colors.Ink
	}
	set := func(color ui.Color, kinds ...editor.TokenKind) {
		for _, kind := range kinds {
			th.Tokens[kind].Color = color
		}
	}
	set(colors.Accent, editor.TokenKeyword, editor.TokenString, editor.TokenCode, editor.TokenTag)
	set(colors.Warn, editor.TokenNumber, editor.TokenConstant, editor.TokenType, editor.TokenAttribute)
	set(colors.Ink2, editor.TokenFunction, editor.TokenProperty, editor.TokenRegexp, editor.TokenEscape)
	set(colors.Ink3, editor.TokenComment, editor.TokenPunctuation, editor.TokenOperator, editor.TokenMeta)
	set(colors.Success, editor.TokenInserted)
	set(colors.Danger, editor.TokenInvalid)
	th.Tokens[editor.TokenDeleted].Color = colors.Ink3
	th.Tokens[editor.TokenHeading].Bold = true
	th.Tokens[editor.TokenStrong].Bold = true
	th.Tokens[editor.TokenEmphasis].Italic = true
	th.Tokens[editor.TokenLink].Color = colors.Accent
	th.Tokens[editor.TokenComment].Italic = true
	return &th
}
