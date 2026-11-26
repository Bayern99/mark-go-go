package markdown

import md "github.com/Bayern99/mark-go-go/internal/markdown"

func WithoutNotes(source string, notes []md.Note, stripAll bool) string {
	return withoutNotes(source, notes, stripAll)
}

func StripResidualNotes(source string) string {
	data := []byte(source)
	if !maskResidualNoteComments(data) {
		return source
	}
	return string(data)
}
