package markdown

import (
	"fmt"
	"hash/fnv"
	"strings"
)

const blankAtDocumentEnd = " \t\r\n"

func SectionHash(section, document string, runsToDocumentEnd bool) string {
	if runsToDocumentEnd {
		section = sectionTextAtDocumentEnd(section, document)
	}
	return SectionHashAsWritten(section)
}

func SectionHashAsWritten(section string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(section))
	return fmt.Sprintf("fnv1a32:%08x", h.Sum32())
}

func sectionTextAtDocumentEnd(section, document string) string {
	lineEnd := "\n"
	if strings.Contains(strings.TrimRight(document, blankAtDocumentEnd), "\r\n") {
		lineEnd = "\r\n"
	}
	return strings.TrimRight(section, blankAtDocumentEnd) + lineEnd
}
