package nativereview

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/egoist/mygo/ui"
)

// diffSides are the two texts a diff was computed from. A review shows them
// so the view can mark whitespace the lines themselves cannot show: a tab,
// spaces at a line's end, a CRLF against a LF, or a missing last newline.
type diffSides struct{ before, after string }

func (s diffSides) known() bool { return s.before != "" || s.after != "" }

// diffMarks are what one changed line adds to its row and its description.
type diffMarks struct {
	glyphs string
	eof    bool
	words  []string
}

func (m diffMarks) describe(desc string) string {
	if len(m.words) == 0 {
		return desc
	}
	return desc + "; " + strings.Join(m.words, ", ")
}

func (m *diffMarks) add(glyphs string, word string) {
	m.glyphs += glyphs
	if word != "" {
		m.words = append(m.words, word)
	}
}

func drawDiff(c *ui.Context, hunks []review.DiffHunk, decisions map[string]review.HunkDecision) {
	drawDiffSides(c, hunks, decisions, diffSides{})
}

func drawDiffSides(c *ui.Context, hunks []review.DiffHunk, decisions map[string]review.HunkDecision, sides diffSides) {
	if len(hunks) == 0 {
		portrait.Type.UI.Apply(ui.Text(c, "No changes.")).TextColor(portrait.Colors.Ink)
		return
	}
	ui.Scroll(c).MaxHeight(portrait.Diff.PanelMax).MinHeight(portrait.Diff.PanelMin).Label("Document diff").Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			for _, h := range hunks {
				drawHunk(c.Key(h.ID), h, decisions, sides)
			}
		})
	})
}

func drawHunk(c *ui.Context, h review.DiffHunk, decisions map[string]review.HunkDecision, sides diffSides) {
	border := portrait.Colors.Line
	if h.Conflict {
		border = portrait.Colors.Danger
	}
	marks := hunkMarks(h.Lines, sides)
	ui.Column(c).Border(0.5, border).Radius(portrait.Diff.Radius).Clip().Background(portrait.Colors.Paper).Children(func() {
		ui.Row(c).Wrap().Gap(6).Padding(5, 8).AlignItems(ui.Center).Justify(ui.SpaceBetween).
			BorderWidth(0, 0, 1, 0).BorderColor(portrait.Colors.Line).Children(func() {
			portrait.Diff.Range.Apply(ui.Text(c, h.Header)).TextColor(portrait.Colors.Ink3).FontFeatures("tnum")
			if h.Conflict {
				conflict := "Conflict"
				if h.ConflictReason != "" {
					conflict += ": " + h.ConflictReason
				}
				portrait.Diff.Conflict.Apply(ui.Text(c, conflict)).Wrap().Role(ui.RoleStatus).FontWeight(500).TextColor(portrait.Colors.Danger)
			}
			if decisions != nil {
				hunkButtons(c, h, decisions)
			}
		})
		ui.Column(c).Role(ui.RoleGroup).Label("Hunk " + h.Header).Children(func() {
			for i, line := range h.Lines {
				diffLine(c.Key(i), line, marks[i])
			}
		})
	})
}

func diffLine(c *ui.Context, line review.DiffLine, marks diffMarks) {
	sigil, sigilInk, tint := " ", portrait.Colors.Ink3, ui.Transparent
	switch line.Kind {
	case review.DiffLineInsert:
		sigil, sigilInk, tint = "+", portrait.Colors.Success, portrait.Diff.Insert
	case review.DiffLineDelete:
		sigil, sigilInk, tint = "−", portrait.Colors.Danger, portrait.Diff.Delete
	}
	text := func(s string) ui.Element {
		return portrait.Diff.Lines.Apply(ui.Text(c, s))
	}
	number := func(n int) {
		label := ""
		if n > 0 {
			label = strconv.Itoa(n)
		}
		text(label).Width(portrait.Diff.NumberWidth).TextAlign(ui.End).FontFeatures("tnum").TextColor(portrait.Colors.Ink2).AccessHidden(true)
	}
	desc := "Unchanged, lines " + strconv.Itoa(line.OldLine) + "→" + strconv.Itoa(line.NewLine)
	switch line.Kind {
	case review.DiffLineInsert:
		desc = "Added, line " + strconv.Itoa(line.NewLine)
	case review.DiffLineDelete:
		desc = "Removed, line " + strconv.Itoa(line.OldLine)
	}
	desc = marks.describe(desc)
	ui.Row(c).Gap(4).Padding(1, 6).Background(tint).Children(func() {
		number(line.OldLine)
		number(line.NewLine)
		text(sigil).Width(portrait.Diff.SigilWidth).TextAlign(ui.Center).TextColor(sigilInk).AccessHidden(true)
		content := text(line.Text).Wrap().Grow(1).Shrink(1).MinWidth(0).TextColor(portrait.Colors.Ink)
		if line.Text == "" {
			content.Role(ui.RoleText).Label(desc)
		} else {
			content.Selectable().Description(desc)
		}
		if marks.glyphs != "" {
			text(marks.glyphs).Shrink(0).TextColor(portrait.Colors.Ink3).AccessHidden(true)
		}
		if marks.eof {
			text("no newline at end").Shrink(0).TextColor(portrait.Colors.Ink3).AccessHidden(true)
		}
	})
}

// hunkMarks marks each changed line of a hunk: a delete and the insert that
// replaces it are compared, so the row can show the whitespace they differ by
// rather than two identical-looking lines.
func hunkMarks(lines []review.DiffLine, sides diffSides) []diffMarks {
	marks := make([]diffMarks, len(lines))
	before, after := rawLines(sides.before), rawLines(sides.after)
	for i := 0; i < len(lines); {
		if lines[i].Kind == review.DiffLineContext {
			i++
			continue
		}
		deletes := i
		for i < len(lines) && lines[i].Kind == review.DiffLineDelete {
			i++
		}
		inserts := i
		for i < len(lines) && lines[i].Kind == review.DiffLineInsert {
			i++
		}
		for k := deletes; k < i; k++ {
			marks[k] = textMarks(lines[k].Text)
		}
		if !sides.known() {
			continue
		}
		for k := 0; k < inserts-deletes && k < i-inserts; k++ {
			del, ins := lines[deletes+k], lines[inserts+k]
			deleted, delOK := rawAt(before, del.OldLine-1)
			inserted, insOK := rawAt(after, ins.NewLine-1)
			if !delOK || !insOK {
				continue
			}
			marks[deletes+k], marks[inserts+k] = pairMarks(deleted, inserted)
		}
	}
	return marks
}

// rawLines splits text the way the diff does, keeping each line's terminator.
func rawLines(text string) []string {
	var out []string
	for text != "" {
		i := strings.IndexAny(text, "\r\n")
		if i < 0 {
			out = append(out, text)
			break
		}
		end := i + 1
		if text[i] == '\r' && end < len(text) && text[end] == '\n' {
			end++
		}
		out = append(out, text[:end])
		text = text[end:]
	}
	return out
}

func rawAt(lines []string, index int) (string, bool) {
	if index < 0 || index >= len(lines) {
		return "", false
	}
	return lines[index], true
}

// splitEnding separates a raw line's text from its terminator, which is empty
// on a last line that has no newline.
func splitEnding(raw string) (body, ending string) {
	switch {
	case strings.HasSuffix(raw, "\r\n"):
		return raw[:len(raw)-2], "\r\n"
	case strings.HasSuffix(raw, "\n"):
		return raw[:len(raw)-1], "\n"
	case strings.HasSuffix(raw, "\r"):
		return raw[:len(raw)-1], "\r"
	}
	return raw, ""
}

// pairMarks compares a delete with the insert replacing it. Lines that differ
// only in whitespace are marked with what tells them apart; lines whose text
// differs keep only the trailing whitespace marker, which is always visible.
func pairMarks(deleted, inserted string) (diffMarks, diffMarks) {
	deletedBody, deletedEnd := splitEnding(deleted)
	insertedBody, insertedEnd := splitEnding(inserted)
	if normalizeLine(deletedBody) != normalizeLine(insertedBody) {
		return textMarks(deletedBody), textMarks(insertedBody)
	}
	left, right := whitespaceMarks(deletedBody), whitespaceMarks(insertedBody)
	if deletedLead, insertedLead := leadingRun(deletedBody), leadingRun(insertedBody); deletedLead != insertedLead {
		if deletedLead != "" {
			left.add(runGlyphs(deletedLead), describeRun(deletedLead, "leading"))
		}
		if insertedLead != "" {
			right.add(runGlyphs(insertedLead), describeRun(insertedLead, "leading"))
		}
	}
	if deletedEnd != insertedEnd {
		switch {
		case deletedEnd == "":
			left.eof = true
			left.add("", "no newline at end of file")
			right.add("", "the other side has no newline at the end")
		case insertedEnd == "":
			right.eof = true
			right.add("", "no newline at end of file")
			left.add("", "the other side has no newline at the end")
		}
		if deletedEnd == "\r\n" && insertedEnd == "\n" {
			left.add("␍", "CRLF line ending, the other side uses LF")
		}
		if insertedEnd == "\r\n" && deletedEnd == "\n" {
			right.add("␍", "CRLF line ending, the other side uses LF")
		}
	}
	return left, right
}

// normalizeLine folds the whitespace a monospace row cannot show as a change.
func normalizeLine(body string) string {
	body = strings.ReplaceAll(body, "\t", "    ")
	return strings.Trim(body, " ")
}

func leadingRun(body string) string {
	return body[:len(body)-len(strings.TrimLeft(body, " \t"))]
}

func runGlyphs(run string) string {
	glyphs := ""
	for _, r := range run {
		if r == '\t' {
			glyphs += "→"
			continue
		}
		glyphs += "·"
	}
	return glyphs
}

func describeRun(run, where string) string {
	spaces, tabs := strings.Count(run, " "), strings.Count(run, "\t")
	var parts []string
	if spaces > 0 {
		word := "spaces"
		if spaces == 1 {
			word = "space"
		}
		parts = append(parts, fmt.Sprintf("%d %s %s", spaces, where, word))
	}
	if tabs > 0 {
		word := "tabs"
		if tabs == 1 {
			word = "tab"
		}
		parts = append(parts, fmt.Sprintf("%d %s %s", tabs, where, word))
	}
	return strings.Join(parts, " and ")
}

func textMarks(body string) diffMarks {
	var marks diffMarks
	trailing := trailingRun(body)
	if trailing == "" {
		return marks
	}
	marks.add(runGlyphs(trailing), describeRun(trailing, "trailing"))
	return marks
}

func whitespaceMarks(body string) diffMarks {
	marks := textMarks(body)
	if tabs := strings.Count(strings.TrimRight(body, " \t"), "\t"); tabs > 0 {
		word := "tabs"
		if tabs == 1 {
			word = "tab"
		}
		marks.add(strings.Repeat("→", tabs), fmt.Sprintf("%d %s", tabs, word))
	}
	return marks
}

func trailingRun(body string) string {
	trimmed := strings.TrimRight(body, " \t")
	return body[len(trimmed):]
}

func hunkButtons(c *ui.Context, h review.DiffHunk, decisions map[string]review.HunkDecision) {
	portrait.Group(c, "Hunk actions", func(g *portrait.GroupBuilder) {
		if decide(c, g, "Accept", "Accept hunk", decisions[h.ID] == review.HunkAccepted, h.Conflict) {
			decisions[h.ID] = review.HunkAccepted
		}
		if decide(c, g, "Reject", "Reject hunk", decisions[h.ID] == review.HunkRejected, false) {
			decisions[h.ID] = review.HunkRejected
		}
		if decisions[h.ID] != "" && decide(c, g, "Clear", "Clear choice", false, false) {
			delete(decisions, h.ID)
		}
	})
}

func decide(c *ui.Context, g *portrait.GroupBuilder, title, label string, selected, disabled bool) bool {
	b := portrait.ButtonWith(c, title, label, portrait.ButtonSpec{Look: portrait.Quiet, Selected: selected}).Disabled(disabled)
	if disabled {
		b.Opacity(0.5)
	}
	g.Add(b)
	return b.Clicked()
}
