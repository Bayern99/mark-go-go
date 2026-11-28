package portrait_test

import (
	"strconv"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
)

func TestEditorTableTokens(t *testing.T) {
	tokens := portrait.EditorFor(16, 13, 624)
	if tokens.TableCellPadX != 8 || tokens.TableCellPadY != 4 {
		t.Errorf("table cell padding = %v / %v, want 8 / 4", tokens.TableCellPadX, tokens.TableCellPadY)
	}
	if tokens.TableRadius != portrait.Radii.M {
		t.Errorf("table radius = %v, want %v", tokens.TableRadius, portrait.Radii.M)
	}
	if tokens.TableColumnMin != 112 || tokens.TableColumnMax != 340 || tokens.TableColumnResizeMin != 72 {
		t.Errorf("table column bounds = %v / %v, resize minimum %v; want 112 / 340 and 72", tokens.TableColumnMin, tokens.TableColumnMax, tokens.TableColumnResizeMin)
	}
	if want := portrait.Type.Small.Leading + 2*tokens.TableCellPadY; tokens.TableRowMin != want {
		t.Errorf("table row minimum = %v, want one 14/22 line plus vertical padding = %v", tokens.TableRowMin, want)
	}
	if tokens.TableEdgeFade != 24 {
		t.Errorf("table edge fade = %v, want 24", tokens.TableEdgeFade)
	}
	assertTokenSources(t, sourceDerived,
		"Editor.TableCellPadX", "Editor.TableCellPadY", "Editor.TableRadius",
		"Editor.TableColumnMin", "Editor.TableColumnMax", "Editor.TableColumnResizeMin", "Editor.TableRowMin", "Editor.TableEdgeFade",
	)
}

func TestEditorVerticalRhythmTokens(t *testing.T) {
	tests := []struct {
		size                         float32
		list, nested, paragraph      float32
		headingBefore, headingAfter  [6]float32
		codeBottom, rule, quoteWidth float32
	}{
		{size: 15, list: 6, nested: 5, paragraph: 15,
			headingBefore: [6]float32{36, 28, 23, 21, 19, 19}, headingAfter: [6]float32{13, 9, 9, 6, 5, 4},
			codeBottom: 11, rule: 26, quoteWidth: 3},
		{size: 16, list: 6, nested: 5, paragraph: 16,
			headingBefore: [6]float32{38, 30, 24, 22, 20, 20}, headingAfter: [6]float32{14, 10, 10, 6, 5, 4},
			codeBottom: 12, rule: 28, quoteWidth: 3},
		{size: 17, list: 6, nested: 5, paragraph: 17,
			headingBefore: [6]float32{40, 32, 26, 23, 21, 21}, headingAfter: [6]float32{15, 11, 11, 6, 5, 4},
			codeBottom: 13, rule: 30, quoteWidth: 3},
		{size: 18, list: 7, nested: 6, paragraph: 18,
			headingBefore: [6]float32{43, 34, 27, 25, 23, 23}, headingAfter: [6]float32{16, 11, 11, 7, 6, 5},
			codeBottom: 14, rule: 32, quoteWidth: 3},
	}
	for _, tc := range tests {
		t.Run(strconv.Itoa(int(tc.size)), func(t *testing.T) {
			tokens := portrait.EditorFor(tc.size, 13, 624)
			if tokens.ListGap != tc.list || tokens.NestedListGap != tc.nested || tokens.ParagraphGap != tc.paragraph {
				t.Errorf("list / nested / paragraph gaps = %v / %v / %v, want %v / %v / %v", tokens.ListGap, tokens.NestedListGap, tokens.ParagraphGap, tc.list, tc.nested, tc.paragraph)
			}
			if tokens.HeadingBefore != tc.headingBefore || tokens.HeadingAfter != tc.headingAfter {
				t.Errorf("heading before / after = %v / %v, want %v / %v", tokens.HeadingBefore, tokens.HeadingAfter, tc.headingBefore, tc.headingAfter)
			}
			if tokens.CodeBlockBottom != tc.codeBottom || tokens.ThematicBreakHeight != tc.rule || tokens.QuoteBarWidth != tc.quoteWidth {
				t.Errorf("code bottom / rule / quote width = %v / %v / %v, want %v / %v / %v", tokens.CodeBlockBottom, tokens.ThematicBreakHeight, tokens.QuoteBarWidth, tc.codeBottom, tc.rule, tc.quoteWidth)
			}
			if tokens.Body.Leading != tc.rule {
				t.Errorf("rule height %v does not follow body leading %v", tokens.ThematicBreakHeight, tokens.Body.Leading)
			}
			for level, before := range tokens.HeadingBefore {
				if !(tokens.ListGap < tokens.ParagraphGap && tokens.ParagraphGap < before) {
					t.Errorf("level %d does not satisfy ListGap < ParagraphGap < HeadingBefore: %v < %v < %v", level+1, tokens.ListGap, tokens.ParagraphGap, before)
				}
			}
		})
	}
	assertTokenSources(t, sourceDerived,
		"Editor.ListGap", "Editor.NestedListGap", "Editor.ParagraphGap",
		"Editor.HeadingBefore", "Editor.HeadingAfter", "Editor.CodeBlockBottom",
		"Editor.ThematicBreakHeight", "Editor.QuoteBarWidth",
	)
}
