package nativegenui_test

import (
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativegenui"
	"github.com/egoist/mygo/ui"
)

func boxOf(t *testing.T, tt *ui.Tester, text string) ui.Rect {
	t.Helper()
	box, ok := tt.Find(text)
	if !ok {
		t.Fatalf("%q is not shown in %q", text, tt.Texts())
	}
	return box
}

func level(a, b float32) bool {
	return a-b < 0.5 && b-a < 0.5
}

func TestRowKeepsItsTitleWordsWholeAndMovesTheRestBelowWhenTheyDoNotFit(t *testing.T) {
	items := `[Title("Recommendation", "md"), Spacer(), Badge("Search team: Qdrant 1.12", "success"), Badge("Not final", "warning")]`
	for name, row := range map[string]string{
		"Row": `Row(` + items + `, 2, "center")`,
		"Box": `Box(` + items + `, "row", 2)`,
	} {
		t.Run(name, func(t *testing.T) {
			view := statementView(t, `root = Col([Card([`+row+`, Text("The search team recommends Qdrant 1.12.", "sm")])])`)
			roomy, _ := shown(t, view, 760, nil)
			whole := boxOf(t, roomy, "Recommendation")
			for _, badge := range []string{"Search team: Qdrant 1.12", "Not final"} {
				if beside := boxOf(t, roomy, badge); beside.Y >= whole.Y+whole.H || beside.X < whole.X+whole.W {
					t.Errorf("with room to spare %q at %v is not beside the title at %v", badge, beside, whole)
				}
			}
			for width, moved := range map[int][]string{400: {"Not final"}, 320: {"Search team: Qdrant 1.12", "Not final"}} {
				tight, _ := shown(t, view, width, nil)
				title := boxOf(t, tight, "Recommendation")
				if !level(title.H, whole.H) || title.W < whole.W-0.5 {
					t.Errorf("width %d: the one-word title takes %v, on one line it takes %v", width, title, whole)
				}
				for _, badge := range moved {
					if below := boxOf(t, tight, badge); below.Y < title.Y+title.H {
						t.Errorf("width %d: %q at %v is not below the title at %v", width, badge, below, title)
					}
				}
			}
		})
	}
}

func TestRowKeepsWhatLeadsItsTitleBesideItWhenTheTitleTakesSeveralLines(t *testing.T) {
	view := statementView(t, `root = Col([Card([Row([Badge("Step 2", "info"), Title("Move the read traffic to the replica cluster", "md"), Spacer(), Badge("Draft", "warning")], 2)])])`)
	roomy, _ := shown(t, view, 760, nil)
	line := boxOf(t, roomy, "Move the read traffic to the replica cluster")
	tight, _ := shown(t, view, 320, nil)
	lead, title, rest := boxOf(t, tight, "Step 2"), boxOf(t, tight, "Move the read traffic to the replica cluster"), boxOf(t, tight, "Draft")
	if title.H < 2*line.H {
		t.Fatalf("the title takes %v in a narrow window, one line is %v high", title, line.H)
	}
	if lead.X+lead.W > title.X || lead.Y+lead.H <= title.Y || lead.Y >= title.Y+title.H {
		t.Errorf("the leading badge at %v is not beside the title at %v", lead, title)
	}
	if rest.Y < title.Y+title.H {
		t.Errorf("the trailing badge at %v is not below the title at %v", rest, title)
	}
}

func TestRowKeepsATitleWholeInsideTheColumnThatCaptionsIt(t *testing.T) {
	view := statementView(t, `root = Col([Card([Row([Col([Title("Recommendation", "md"), Caption("Search team")], 1), Spacer(), Badge("Search team: Qdrant 1.12", "success"), Badge("Not final", "warning")], 2, "center")])])`)
	roomy, _ := shown(t, view, 760, nil)
	whole := boxOf(t, roomy, "Recommendation")
	if beside := boxOf(t, roomy, "Not final"); beside.Y >= whole.Y+whole.H {
		t.Fatalf("with room to spare the badge at %v is not beside the title at %v", beside, whole)
	}
	tight, _ := shown(t, view, 320, nil)
	title, caption := boxOf(t, tight, "Recommendation"), boxOf(t, tight, "Search team")
	if !level(title.H, whole.H) {
		t.Errorf("the one-word title takes %v, on one line it takes %v", title, whole)
	}
	for _, badge := range []string{"Search team: Qdrant 1.12", "Not final"} {
		if below := boxOf(t, tight, badge); below.Y < caption.Y+caption.H {
			t.Errorf("%q at %v is not below the caption at %v", badge, below, caption)
		}
	}
}

func TestGridGivesACardLeftAloneOnTheLastRowTheWholeRow(t *testing.T) {
	for name, cards := range map[string]string{
		"cards":         `Card([Title("pgvector", "md")]), Card([Title("Qdrant", "md")]), Card([Title("Milvus", "md")])`,
		"sourced cards": `Source(["s"], [Card([Title("pgvector", "md")])]), Source(["s"], [Card([Title("Qdrant", "md")])]), Source(["s"], [Card([Title("Milvus", "md")])])`,
	} {
		t.Run(name, func(t *testing.T) {
			three, _ := shown(t, statementView(t, `root = Col([Grid([`+cards+`], 3, 240, 4)])`), 1100, nil)
			first, second, third := boxOf(t, three, "pgvector"), boxOf(t, three, "Qdrant"), boxOf(t, three, "Milvus")
			if !level(first.Y, third.Y) || second.X <= first.X || third.X <= second.X || !level(third.W, first.W) {
				t.Fatalf("three columns fit, the cards' titles are at %v, %v and %v", first, second, third)
			}
			two, _ := shown(t, statementView(t, `root = Col([Grid([`+cards+`], 3, 260, 4)])`), 1100, nil)
			first, second, third = boxOf(t, two, "pgvector"), boxOf(t, two, "Qdrant"), boxOf(t, two, "Milvus")
			if !level(first.Y, second.Y) || third.Y <= first.Y {
				t.Fatalf("two columns fit, the cards' titles are at %v, %v and %v", first, second, third)
			}
			if !level(third.X, first.X) || !level(third.X+third.W, second.X+second.W) {
				t.Errorf("the third card's title spans %v to %v, the row above spans %v to %v", third.X, third.X+third.W, first.X, second.X+second.W)
			}
			one, _ := shown(t, statementView(t, `root = Col([Grid([`+cards+`], 3, 260, 4)])`), 380, nil)
			first, second, third = boxOf(t, one, "pgvector"), boxOf(t, one, "Qdrant"), boxOf(t, one, "Milvus")
			if second.Y <= first.Y || third.Y <= second.Y || !level(second.X, first.X) || !level(third.X, first.X) || !level(second.W, first.W) || !level(third.W, first.W) {
				t.Errorf("one column fits, the cards' titles are at %v, %v and %v", first, second, third)
			}
		})
	}
}

func TestCardsSideBySideAreEquallyHighAndTheirSourcesLineUp(t *testing.T) {
	short := `Source(["s"], [Card([Title("Trade-offs", "md")])])`
	tall := `Source(["s"], [Card([Title("Recommendation", "md"), Text("The search team recommends Qdrant 1.12. It is the only option that meets both targets with a single new service.", "sm")])])`
	for name, statement := range map[string]string{
		"grid": `Grid([` + short + `, ` + tall + `], 2, 240, 4)`,
		"row":  `Row([` + short + `, ` + tall + `], 4)`,
	} {
		t.Run(name, func(t *testing.T) {
			tt, _ := shown(t, statementView(t, `root = Col([`+statement+`])`), 800, func(r *nativegenui.Reading) {
				r.Sources = func(c *ui.Context, n nativegenui.Node) { ui.Text(c, "From "+n.ID) }
			})
			var entries []ui.Rect
			for _, text := range tt.Texts() {
				if strings.HasPrefix(text, "From ") {
					entries = append(entries, boxOf(t, tt, text))
				}
			}
			if len(entries) != 2 || entries[0].X >= entries[1].X {
				t.Fatalf("the source entries are at %v", entries)
			}
			if !level(entries[0].Y, entries[1].Y) {
				t.Errorf("the source entry under the short card is at %v, under the tall card at %v", entries[0], entries[1])
			}
		})
	}
}

func TestCardsSideBySideEndAtTheSameHeight(t *testing.T) {
	short := `Card([Title("Trade-offs", "md"), Spacer(), Caption("Three in all")])`
	tall := `Card([Title("Recommendation", "md"), Text("The search team recommends Qdrant 1.12. It is the only option that meets both targets with a single new service.", "sm"), Spacer(), Caption("Not final")])`
	for name, statement := range map[string]string{
		"grid":           `Grid([` + short + `, ` + tall + `], 2, 240, 4)`,
		"row":            `Row([` + short + `, ` + tall + `], 4)`,
		"row aligned":    `Row([` + short + `, ` + tall + `], 4, "center")`,
		"row tall first": `Row([` + tall + `, ` + short + `], 4)`,
	} {
		t.Run(name, func(t *testing.T) {
			tt, _ := shown(t, statementView(t, `root = Col([`+statement+`])`), 800, nil)
			less, more := boxOf(t, tt, "Three in all"), boxOf(t, tt, "Not final")
			if level(less.X, more.X) {
				t.Fatalf("the cards are not side by side: their last lines are at %v and %v", less, more)
			}
			if !level(less.Y+less.H, more.Y+more.H) {
				t.Errorf("the short card's last line ends at %v, the tall card's at %v", less.Y+less.H, more.Y+more.H)
			}
		})
	}
}
