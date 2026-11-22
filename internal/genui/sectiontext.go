package genui

import "strings"

const blankAtDocumentEnd = " \t\r\n"

func SectionTextAtDocumentEnd(section, document string) string {
	lineEnd := "\n"
	if strings.Contains(strings.TrimRight(document, blankAtDocumentEnd), "\r\n") {
		lineEnd = "\r\n"
	}
	return strings.TrimRight(section, blankAtDocumentEnd) + lineEnd
}
