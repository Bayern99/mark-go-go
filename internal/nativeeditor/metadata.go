package nativeeditor

import (
	"regexp"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

var referenceStart = regexp.MustCompile(`^ {0,3}\[(?:\\.|[^\]\\])+\]:\s*\S`)

func (p *previewLayout) frontmatter(snap editor.Snapshot) int {
	opener := strings.TrimSpace(snap.Line(0))
	if opener != "---" && opener != "+++" && opener != "---toml" {
		return 0
	}
	for line := 1; line < snap.LineCount(); line++ {
		mark := strings.TrimSpace(snap.Line(line))
		if mark == opener || (opener != "+++" && (mark == "---" || mark == "...")) {
			end := snap.LineEnd(line)
			p.specialBlock(snap, 0, end, "frontmatter", "Frontmatter", snap.Slice(snap.LineStart(1), snap.LineStart(line)))
			return end
		}
	}
	return 0
}
func frontmatterRows(c *ui.Context, src string) {
	var pairs [][2]string
	for _, line := range strings.Split(src, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if ok && key == strings.TrimSpace(key) {
			value = strings.TrimSpace(value)
			if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
				value = value[1 : len(value)-1]
			}
			pairs = append(pairs, [2]string{key, value})
		} else if len(pairs) > 0 {
			pairs[len(pairs)-1][1] = strings.TrimSpace(pairs[len(pairs)-1][1] + "\n" + strings.TrimSpace(line))
		} else {
			pairs = append(pairs, [2]string{"", line})
		}
	}
	for _, pair := range pairs {
		ui.Row(c).Gap(12).Children(func() { ui.Text(c, pair[0]).FontWeight(600); ui.Text(c, pair[1]).Wrap() })
	}
}
func (p *previewLayout) referenceDefinitions(snap editor.Snapshot, frontEnd int) {
	scan := func(a, z int) {
		for a < z {
			line := snap.LineOf(a)
			end := min(z, snap.LineEnd(line))
			if end > a && referenceStart.MatchString(snap.Slice(a, end)) {
				folded := false
				for _, h := range p.hiddenRanges {
					if h[0] <= a && a < h[1] {
						folded = true
						break
					}
				}
				if !folded {
					p.specialBlock(snap, a, end, "reference definition", "Reference definition", snap.Slice(a, end))
				}
			}
			a = end + 1
		}
	}
	last := frontEnd
	for i := range p.document.Children(0) {
		b := p.document.Block(i)
		if b.End <= frontEnd {
			continue
		}
		start := snap.LineStart(snap.LineOf(b.Start))
		if start > last {
			scan(last, start)
		}
		last = min(snap.Len(), snap.LineEnd(snap.LineOf(b.End))+1)
	}
	scan(last, snap.Len())
}
