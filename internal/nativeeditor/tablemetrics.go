package nativeeditor

import (
	"math"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"golang.org/x/text/width"
)

const (
	tableColumnMin  float32 = 112
	tableColumnMax  float32 = 340
	tableFieldInset float32 = 10
	tableCellInset          = tableFieldInset + 14
	narrowAdvance   float32 = 0.56
)

var tableRowMin = portrait.Sizes.Large + 4

func textAdvance(s string, size float32) float32 {
	w := float32(0)
	for _, r := range s {
		switch width.LookupRune(r).Kind() {
		case width.EastAsianWide, width.EastAsianFullwidth:
			w += size
		default:
			w += size * narrowAdvance
		}
	}
	return w
}

func cellLines(text string, columnWidth float32) int {
	room := max(1, columnWidth-tableCellInset)
	lines := 0
	for _, line := range strings.Split(text, "\n") {
		lines += max(1, int(math.Ceil(float64(textAdvance(line, portrait.Type.Small.Size)/room))))
	}
	return max(1, lines)
}

func (m *readingTable) rowHeight(cells []string) float32 {
	lines := 1
	for k, text := range cells {
		if k < len(m.widths) {
			lines = max(lines, cellLines(text, m.widths[k]))
		}
	}
	return max(tableRowMin, float32(lines)*portrait.Type.Small.Leading+tableRowMin-portrait.Type.Small.Leading)
}

func (m *readingTable) height(rows [][]string) float32 {
	h := float32(0)
	for _, cells := range rows {
		h += m.rowHeight(cells)
	}
	return h
}
