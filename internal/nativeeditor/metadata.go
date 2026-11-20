package nativeeditor

import (
	"regexp"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
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
			p.specialBlock(snap, 0, end, "frontmatter", "Frontmatter", "", snap.Slice(snap.LineStart(1), snap.LineStart(line)))
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
	p.scanReferenceDefinitions(snap, frontEnd, func(a, end int) bool {
		folded := false
		for _, h := range p.hiddenRanges {
			if h[0] <= a && a < h[1] {
				folded = true
				break
			}
		}
		if !folded {
			p.specialBlock(snap, a, end, "reference definition", "Reference definition", "", snap.Slice(a, end))
		}
		return false
	})
}

func (p *previewLayout) referenceDestination(snap editor.Snapshot, target string) (start, end int, ok bool) {
	p.scanReferenceDefinitions(snap, p.frontEnd, func(a, z int) bool {
		label, destStart, destEnd, found := referenceDefinitionRange(snap.Slice(a, z))
		if !found || parser.NormalizeLabel(label) != target {
			return false
		}
		start, end, ok = a+destStart, a+destEnd, true
		return true
	})
	return start, end, ok
}

func (p *previewLayout) scanReferenceDefinitions(snap editor.Snapshot, frontEnd int, visit func(start, end int) bool) bool {
	scan := func(a, z int) bool {
		for a < z {
			line := snap.LineOf(a)
			start, end := snap.LineStart(line), min(z, snap.LineEnd(line))
			if end > start && referenceStart.MatchString(snap.Slice(start, end)) && visit(start, end) {
				return true
			}
			a = end + 1
		}
		return false
	}
	last := frontEnd
	for i := range p.document.Children(0) {
		b := p.document.Block(i)
		if b.End <= frontEnd {
			continue
		}
		start := snap.LineStart(snap.LineOf(b.Start))
		if start > last && scan(last, start) {
			return true
		}
		last = min(snap.Len(), snap.LineEnd(snap.LineOf(b.End))+1)
	}
	return scan(last, snap.Len())
}

func referenceDefinitionRange(line string) (label string, destinationStart, destinationEnd int, ok bool) {
	start := 0
	for start < len(line) && line[start] == ' ' {
		start++
	}
	if start > 3 || start >= len(line) || line[start] != '[' {
		return "", 0, 0, false
	}
	close := closingLinkLabel(line[start:])
	if close < 0 {
		return "", 0, 0, false
	}
	label = line[start+1 : start+close]
	i := start + close + 1
	if i >= len(line) || line[i] != ':' {
		return "", 0, 0, false
	}
	i++
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i >= len(line) {
		return "", 0, 0, false
	}
	if line[i] == '<' {
		i++
		destinationStart = i
		for i < len(line) {
			if line[i] == '\\' && i+1 < len(line) {
				i += 2
				continue
			}
			if line[i] == '>' {
				return label, destinationStart, i, true
			}
			i++
		}
		return "", 0, 0, false
	}
	destinationStart = i
	depth := 0
	for i < len(line) {
		if line[i] == '\\' && i+1 < len(line) {
			i += 2
			continue
		}
		switch line[i] {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return label, destinationStart, i, true
			}
			depth--
		case ' ', '\t':
			if depth == 0 {
				return label, destinationStart, i, true
			}
		}
		i++
	}
	return label, destinationStart, i, true
}
