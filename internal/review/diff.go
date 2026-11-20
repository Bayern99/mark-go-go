package review

import (
	"fmt"
	"strings"
)

const DefaultContextLines = 3

type DiffLineKind string

const (
	DiffLineContext DiffLineKind = "context"
	DiffLineDelete  DiffLineKind = "delete"
	DiffLineInsert  DiffLineKind = "insert"
)

type HunkDecision string

const (
	HunkPending  HunkDecision = "pending"
	HunkAccepted HunkDecision = "accepted"
	HunkRejected HunkDecision = "rejected"
)

type DiffLine struct {
	Kind    DiffLineKind `json:"kind"`
	Text    string       `json:"text"`
	OldLine int          `json:"oldLine,omitempty"`
	NewLine int          `json:"newLine,omitempty"`
}

type DiffHunk struct {
	ID             string       `json:"id"`
	Header         string       `json:"header"`
	OldStart       int          `json:"oldStart"`
	OldLines       int          `json:"oldLines"`
	NewStart       int          `json:"newStart"`
	NewLines       int          `json:"newLines"`
	ChangeOldLine  int          `json:"changeOldLine"`
	ChangeOldCount int          `json:"changeOldCount"`
	ChangeNewLine  int          `json:"changeNewLine"`
	ChangeNewCount int          `json:"changeNewCount"`
	Lines          []DiffLine   `json:"lines"`
	Decision       HunkDecision `json:"decision"`
	Conflict       bool         `json:"conflict,omitempty"`
	ConflictReason string       `json:"conflictReason,omitempty"`
}

type textLine struct {
	raw     string
	content string
}

func splitTextLines(text string) []textLine {
	var out []textLine
	for text != "" {
		i := strings.IndexAny(text, "\r\n")
		if i < 0 {
			out = append(out, textLine{raw: text, content: text})
			break
		}
		end := i + 1
		if text[i] == '\r' && end < len(text) && text[end] == '\n' {
			end++
		}
		out = append(out, textLine{raw: text[:end], content: text[:i]})
		text = text[end:]
	}
	return out
}

func endsLine(s string) bool {
	return strings.HasSuffix(s, "\n") || strings.HasSuffix(s, "\r")
}

func defaultLineEnding(text string) string {
	if strings.Contains(text, "\r\n") {
		return "\r\n"
	}
	if !strings.Contains(text, "\n") && strings.Contains(text, "\r") {
		return "\r"
	}
	return "\n"
}

type editOpKind int

const (
	opEqual editOpKind = iota
	opDelete
	opInsert
)

type editOp struct {
	kind   editOpKind
	oldIdx int
	newIdx int
	text   string
	raw    string
}

type changeCluster struct {
	oldStart int
	oldEnd   int
	newStart int
	newEnd   int
	deletes  []editOp
	inserts  []editOp
}

func myersEditScript(a, b []textLine) []editOp {
	n := len(a)
	m := len(b)
	equal := func(x, y int) bool {
		return a[x].raw == b[y].raw
	}

	prefix := 0
	for prefix < n && prefix < m && equal(prefix, prefix) {
		prefix++
	}
	suffix := 0
	for suffix < (n-prefix) && suffix < (m-prefix) && equal(n-1-suffix, m-1-suffix) {
		suffix++
	}

	var ops []editOp
	for i := 0; i < prefix; i++ {
		ops = append(ops, editOp{
			kind:   opEqual,
			oldIdx: i,
			newIdx: i,
			text:   a[i].content,
			raw:    a[i].raw,
		})
	}

	midA := a[prefix : n-suffix]
	midB := b[prefix : m-suffix]
	if len(midA) > 0 || len(midB) > 0 {
		midOps := myersCore(midA, midB, prefix, prefix)
		ops = append(ops, midOps...)
	}

	for i := 0; i < suffix; i++ {
		oldIdx := n - suffix + i
		newIdx := m - suffix + i
		ops = append(ops, editOp{
			kind:   opEqual,
			oldIdx: oldIdx,
			newIdx: newIdx,
			text:   a[oldIdx].content,
			raw:    a[oldIdx].raw,
		})
	}
	return ops
}

const maxMyersTraceEntries = 512 * 1024

func myersCore(a, b []textLine, offsetA, offsetB int) []editOp {
	n := len(a)
	m := len(b)
	lines := make(map[string]struct{}, n)
	for _, line := range a {
		lines[line.raw] = struct{}{}
	}
	shared := false
	for _, line := range b {
		if _, ok := lines[line.raw]; ok {
			shared = true
			break
		}
	}
	if !shared {
		ops := make([]editOp, 0, n+m)
		for i, line := range a {
			ops = append(ops, editOp{kind: opDelete, oldIdx: offsetA + i, newIdx: offsetB, text: line.content, raw: line.raw})
		}
		for i, line := range b {
			ops = append(ops, editOp{kind: opInsert, oldIdx: offsetA + n, newIdx: offsetB + i, text: line.content, raw: line.raw})
		}
		return ops
	}
	maxD := n + m
	size := 2*maxD + 1
	v := make([]int, size)
	var trace [][]int

	equal := func(x, y int) bool {
		return a[x].raw == b[y].raw
	}

	foundD := maxD
	for d := 0; d <= maxD; d++ {
		if d >= maxMyersTraceEntries/size {
			return linearSpaceDiff(a, b, offsetA, offsetB)
		}
		snapshot := make([]int, size)
		copy(snapshot, v)
		trace = append(trace, snapshot)

		for k := -d; k <= d; k += 2 {
			idx := k + maxD
			var x int
			if k == -d || (k != d && v[idx-1] < v[idx+1]) {
				x = v[idx+1]
			} else {
				x = v[idx-1] + 1
			}
			y := x - k
			for x < n && y < m && equal(x, y) {
				x++
				y++
			}
			v[idx] = x
			if x >= n && y >= m {
				foundD = d
				break
			}
		}
		if foundD == d && v[n-m+maxD] >= n {
			break
		}
	}

	var rev []editOp
	x := n
	y := m
	for d := foundD; d > 0; d-- {
		prevV := trace[d]
		k := x - y
		idx := k + maxD
		var prevK int
		if k == -d || (k != d && prevV[idx-1] < prevV[idx+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := prevV[prevK+maxD]
		prevY := prevX - prevK

		for x > prevX && y > prevY {
			x--
			y--
			rev = append(rev, editOp{
				kind:   opEqual,
				oldIdx: offsetA + x,
				newIdx: offsetB + y,
				text:   a[x].content,
				raw:    a[x].raw,
			})
		}

		if x == prevX {
			y--
			rev = append(rev, editOp{
				kind:   opInsert,
				oldIdx: offsetA + x,
				newIdx: offsetB + y,
				text:   b[y].content,
				raw:    b[y].raw,
			})
		} else {
			x--
			rev = append(rev, editOp{
				kind:   opDelete,
				oldIdx: offsetA + x,
				newIdx: offsetB + y,
				text:   a[x].content,
				raw:    a[x].raw,
			})
		}
	}

	for x > 0 && y > 0 {
		x--
		y--
		rev = append(rev, editOp{
			kind:   opEqual,
			oldIdx: offsetA + x,
			newIdx: offsetB + y,
			text:   a[x].content,
			raw:    a[x].raw,
		})
	}

	ops := make([]editOp, len(rev))
	for i := range rev {
		ops[i] = rev[len(rev)-1-i]
	}
	return ops
}

func linearSpaceDiff(a, b []textLine, offsetA, offsetB int) []editOp {
	forward, backward := make([]int, len(b)+1), make([]int, len(b)+1)
	ops := make([]editOp, 0, len(a)+len(b))
	old, next := 0, 0
	consume := func(oldEnd, nextEnd int) {
		for old < oldEnd {
			line := a[old]
			ops = append(ops, editOp{kind: opDelete, oldIdx: offsetA + old, newIdx: offsetB + next, text: line.content, raw: line.raw})
			old++
		}
		for next < nextEnd {
			line := b[next]
			ops = append(ops, editOp{kind: opInsert, oldIdx: offsetA + old, newIdx: offsetB + next, text: line.content, raw: line.raw})
			next++
		}
	}

	var align func(int, int, int, int)
	align = func(left, right, start, end int) {
		if left == right || start == end {
			return
		}
		if right-left == 1 {
			for j := start; j < end; j++ {
				if a[left].raw == b[j].raw {
					consume(left, j)
					line := a[left]
					ops = append(ops, editOp{kind: opEqual, oldIdx: offsetA + old, newIdx: offsetB + next, text: line.content, raw: line.raw})
					old++
					next++
					break
				}
			}
			return
		}
		mid, width := (left+right)/2, end-start
		clear(forward[:width+1])
		clear(backward[:width+1])
		row := func(scores []int, first, last, step, edge int) {
			for i := first; i != last; i += step {
				prev := 0
				for j := 1; j <= width; j++ {
					above := scores[j]
					if a[i].raw == b[edge+(j-1)*step].raw {
						scores[j] = prev + 1
					} else {
						scores[j] = max(above, scores[j-1])
					}
					prev = above
				}
			}
		}
		row(forward, left, mid, 1, start)
		row(backward, right-1, mid-1, -1, end-1)
		split, best := 0, 0
		for j := 0; j <= width; j++ {
			if score := forward[j] + backward[width-j]; score > best {
				split, best = j, score
			}
		}
		if best == 0 {
			return
		}
		align(left, mid, start, start+split)
		align(mid, right, start+split, end)
	}
	align(0, len(a), 0, len(b))
	consume(len(a), len(b))
	return ops
}

func extractClusters(before, after string) ([]textLine, []textLine, []editOp, []changeCluster) {
	if before == after {
		return nil, nil, nil, nil
	}
	a := splitTextLines(before)
	b := splitTextLines(after)
	ops := myersEditScript(a, b)

	var clusters []changeCluster
	i := 0
	for i < len(ops) {
		if ops[i].kind == opEqual {
			i++
			continue
		}
		c := changeCluster{
			oldStart: ops[i].oldIdx,
			newStart: ops[i].newIdx,
		}
		for i < len(ops) && ops[i].kind != opEqual {
			if ops[i].kind == opDelete {
				c.deletes = append(c.deletes, ops[i])
			} else {
				c.inserts = append(c.inserts, ops[i])
			}
			i++
		}
		if len(c.deletes) > 0 {
			c.oldStart = c.deletes[0].oldIdx
			c.oldEnd = c.deletes[len(c.deletes)-1].oldIdx + 1
		} else {
			c.oldEnd = c.oldStart
		}
		if len(c.inserts) > 0 {
			c.newStart = c.inserts[0].newIdx
			c.newEnd = c.inserts[len(c.inserts)-1].newIdx + 1
		} else {
			c.newEnd = c.newStart
		}
		clusters = append(clusters, c)
	}
	return a, b, ops, clusters
}

func ComputeDiff(before, after string) []DiffHunk {
	return BuildHunks(before, after, DefaultContextLines)
}

func BuildHunks(before, after string, contextLines int) []DiffHunk {
	if contextLines < 0 {
		contextLines = DefaultContextLines
	}
	_, _, ops, clusters := extractClusters(before, after)
	if len(clusters) == 0 {
		return nil
	}

	type segment struct {
		equals  []editOp
		cluster *changeCluster
	}
	var equalRuns [][]editOp
	var curEquals []editOp
	for _, op := range ops {
		if op.kind == opEqual {
			curEquals = append(curEquals, op)
		} else {
			if curEquals != nil || len(equalRuns) == 0 {
				equalRuns = append(equalRuns, curEquals)
				curEquals = nil
			}
		}
	}
	equalRuns = append(equalRuns, curEquals)

	hunks := make([]DiffHunk, 0, len(clusters))
	for idx, c := range clusters {
		var preEquals []editOp
		if idx < len(equalRuns) {
			run := equalRuns[idx]
			if len(run) > contextLines {
				preEquals = run[len(run)-contextLines:]
			} else {
				preEquals = run
			}
		}
		var postEquals []editOp
		if idx+1 < len(equalRuns) {
			run := equalRuns[idx+1]
			if len(run) > contextLines {
				postEquals = run[:contextLines]
			} else {
				postEquals = run
			}
		}

		lines := make([]DiffLine, 0, len(preEquals)+len(c.deletes)+len(c.inserts)+len(postEquals))
		for _, eq := range preEquals {
			lines = append(lines, DiffLine{
				Kind:    DiffLineContext,
				Text:    eq.text,
				OldLine: eq.oldIdx + 1,
				NewLine: eq.newIdx + 1,
			})
		}
		for _, del := range c.deletes {
			lines = append(lines, DiffLine{
				Kind:    DiffLineDelete,
				Text:    del.text,
				OldLine: del.oldIdx + 1,
			})
		}
		for _, ins := range c.inserts {
			lines = append(lines, DiffLine{
				Kind:    DiffLineInsert,
				Text:    ins.text,
				NewLine: ins.newIdx + 1,
			})
		}
		for _, eq := range postEquals {
			lines = append(lines, DiffLine{
				Kind:    DiffLineContext,
				Text:    eq.text,
				OldLine: eq.oldIdx + 1,
				NewLine: eq.newIdx + 1,
			})
		}

		oldStart := c.oldStart + 1
		oldCount := 0
		newStart := c.newStart + 1
		newCount := 0
		foundOld := false
		foundNew := false
		for _, l := range lines {
			if l.Kind != DiffLineInsert {
				if !foundOld && l.OldLine > 0 {
					oldStart = l.OldLine
					foundOld = true
				}
				oldCount++
			}
			if l.Kind != DiffLineDelete {
				if !foundNew && l.NewLine > 0 {
					newStart = l.NewLine
					foundNew = true
				}
				newCount++
			}
		}

		hunk := DiffHunk{
			ID:             fmt.Sprintf("hunk-%d", idx+1),
			Header:         fmt.Sprintf("@@ -%d,%d +%d,%d @@", oldStart, oldCount, newStart, newCount),
			OldStart:       oldStart,
			OldLines:       oldCount,
			NewStart:       newStart,
			NewLines:       newCount,
			ChangeOldLine:  c.oldStart + 1,
			ChangeOldCount: len(c.deletes),
			ChangeNewLine:  c.newStart + 1,
			ChangeNewCount: len(c.inserts),
			Lines:          lines,
			Decision:       HunkPending,
		}
		hunks = append(hunks, hunk)
	}
	return hunks
}
