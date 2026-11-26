package genuikit

import (
	_ "embed"
	"strconv"
	"strings"
	"sync"
)

//go:embed icons.tsv
var iconTable string

var icons = sync.OnceValue(func() map[string]string {
	table := map[string]string{}
	for _, line := range strings.Split(iconTable, "\n") {
		name, code, ok := strings.Cut(line, "\t")
		if point, err := strconv.ParseUint(code, 16, 32); ok && err == nil {
			table[name] = string(rune(point))
		}
	}
	return table
})

func IconGlyph(name string) (string, bool) {
	glyph, ok := icons()[strings.ToLower(strings.TrimSpace(name))]
	return glyph, ok
}
