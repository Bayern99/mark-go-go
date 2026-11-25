package genui

import "github.com/Bayern99/mark-go-go/internal/markdown"

func SectionHash(section, document string, runsToDocumentEnd bool) string {
	return markdown.SectionHash(section, document, runsToDocumentEnd)
}

func SectionHashAsWritten(section string) string {
	return markdown.SectionHashAsWritten(section)
}
