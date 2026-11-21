package nativegenui

import (
	"cmp"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

type Reading struct {
	remaining  int
	surface    *Surface
	scroll     ui.ScrollState
	Replacing  []string
	states     map[string]*componentState
	Sources    func(*ui.Context, Node)
	SelectItem func(Node, string)
	CanSelect  func(Node) bool
	LoadImage  func(src string) (ui.ImageSource, error)
	images     sync.Map
	form       string
	fields     map[string][]checkedField
	problems   map[string]string
	presses    []press
	changed    bool
	pressable  int
	touched    map[string]bool
}

type componentState struct {
	frame              ui.Handle
	height             float32
	tab, column, order int
	expanded, open     bool
	scroll             ui.ScrollState
	text, shown, note  string
	number             float64
	date               time.Time
	editing, started   bool
	running, flipped   bool
}

func (r *Reading) Load(s *Surface) {
	r.surface = s
	r.images.Clear()
	r.problems, r.presses, r.changed, r.form = map[string]string{}, nil, false, ""
	r.touched = map[string]bool{}
	for _, state := range r.states {
		state.text, state.shown, state.note, state.number, state.date = "", "", "", 0, time.Time{}
		state.editing, state.started, state.running, state.flipped = false, false, false, false
	}
	if r.states == nil {
		r.states = map[string]*componentState{}
	}
}
func (r *Reading) View(c *ui.Context) {
	if r.surface == nil {
		return
	}
	r.remaining = max(4096, len(r.surface.Nodes)*2)
	ui.Scroll(c).TrackScroll(&r.scroll).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Padding(24).Gap(14).Children(func() {
			if r.surface.Session != nil {
				r.fields = map[string][]checkedField{}
				r.statements(c)
				r.settle(c)
				return
			}
			r.node(c, "root", map[string]bool{})
		})
	})
}
func (r *Reading) node(c *ui.Context, id string, parents map[string]bool) {
	if r.remaining <= 0 {
		if r.remaining == 0 {
			ui.Text(c, "This layout repeats too many components to display.").Role(ui.RoleAlert)
			r.remaining--
		}
		return
	}
	r.remaining--
	n, ok := r.surface.Nodes[id]
	if !ok || parents[id] || len(parents) > 64 {
		ui.Text(c, "Invalid component reference: "+id).Role(ui.RoleAlert)
		return
	}
	parents[id] = true
	defer delete(parents, id)
	c = c.Key(id)
	state := r.states[id]
	if state == nil {
		state = &componentState{}
		r.states[id] = state
	}
	children := func() {
		for _, child := range n.Children.IDs {
			r.node(c, child, parents)
		}
	}
	switch n.Component {
	case "Column":
		layout(ui.Column(c).Gap(12), n).Children(children)
	case "Row":
		layout(ui.Row(c).Wrap().Gap(12), n).Children(children)
	case "Tabs":
		if len(n.Tabs) == 0 {
			return
		}
		names := make([]string, len(n.Tabs))
		for i, t := range n.Tabs {
			names[i] = t.Title.String()
		}
		state.tab = min(state.tab, len(names)-1)
		layout(ui.Column(c).Gap(8), n).Children(func() {
			ui.Tabs(c, &state.tab, names...).Label("Reading sections")
			r.node(c, n.Tabs[state.tab].Child, parents)
		})
	case "Card":
		layout(ui.Column(c).Gap(8).Padding(16).Border(1, c.Theme().Border).Radius(10), n).Children(func() { r.node(c, n.Child, parents) })
	case "Text":
		layout(ui.Column(c), n).Children(func() {
			opts := &readOnlyMarkdown
			if n.Variant == "caption" {
				opts = &captionMarkdown
			}
			markdown.View(c, n.Text.String(), opts)
		})
	default:
		busy := slices.Contains(r.Replacing, id) || (n.ComponentID != "" && slices.Contains(r.Replacing, n.ComponentID))
		if !busy {
			state.height = state.frame.Bounds(c).H
		}
		frame := layout(ui.Column(c).Gap(8), n).Bind(&state.frame).Busy(busy)
		if busy {
			frame.MinHeight(max(64, state.height)).Children(func() {
				ui.Text(c, "Updating card…").Role(ui.RoleStatus)
				skeleton(c)
			})
			return
		}
		frame.Children(func() {
			if n.Title != "" {
				ui.Text(c, n.Title).Bold().FontSize(18)
			}
			switch n.Component {
			case "KeyPoints":
				count := len(n.Items)
				if !state.expanded {
					count = min(count, 4)
				}
				for i, raw := range n.Items[:count] {
					var s string
					if json.Unmarshal(raw, &s) == nil {
						ui.Text(c, fmt.Sprintf("%d. %s", i+1, s)).Selectable()
					}
				}
				if len(n.Items) > 4 {
					label := fmt.Sprintf("Show all %d", len(n.Items))
					if state.expanded {
						label = "Show fewer"
					}
					if ui.Button(c, label).Clicked() {
						state.expanded = !state.expanded
					}
				}
			case "MetricGrid":
				ui.Row(c).Gap(16).Wrap().Children(func() {
					for _, m := range n.Metrics {
						ui.Column(c).Padding(8).Children(func() {
							ui.Text(c, m.Label)
							ui.Text(c, m.Value+m.Unit).FontSize(22).Bold()
							if m.Change != "" {
								ui.Text(c, strings.TrimSpace(m.Trend+" "+m.Change))
							}
						})
					}
				})
			case "ComparisonTable":
				comparison(c, n, state)
			case "Timeline":
				for _, e := range n.Events {
					ui.Text(c, e.When).TextColor(c.Theme().TextMuted)
					ui.Text(c, e.Title).Bold()
					if e.Detail != "" {
						ui.Text(c, e.Detail).Selectable()
					}
				}
			case "Checklist":
				for i, raw := range n.Items {
					var item struct {
						Text  string
						Quote *string
						Done  bool
					}
					if json.Unmarshal(raw, &item) != nil {
						continue
					}
					label, status := "☐ ", "Incomplete. Show in source."
					if item.Done {
						label, status = "☑ ", "Completed. Show in source."
					}
					if ui.Button(c.Key(i), label+item.Text).Label(item.Text).Description(status).Disabled(r.SelectItem == nil || (r.CanSelect != nil && !r.CanSelect(n))).Clicked() {
						quote := item.Text
						if item.Quote != nil {
							quote = *item.Quote
						}
						r.SelectItem(n, quote)
					}
				}
			default:
				ui.Text(c, fmt.Sprintf("“%s” is not a component this app can show.", n.Component)).Role(ui.RoleAlert)
			}
			if r.Sources != nil && len(n.Sources) > 0 {
				r.Sources(c, n)
			}
		})
	}
}

func skeleton(c *ui.Context) {
	for _, width := range []float32{82, 64, 38} {
		ui.Row(c).Height(7).WidthPercent(width).Radius(3).Background(c.Theme().Border).AccessHidden(true)
	}
}

var readOnlyMarkdown = markdown.Options{OnLink: func(string) {}}
var captionMarkdown = markdown.Options{OnLink: func(string) {}, Theme: &markdown.Theme{FontSize: 12}}

func layout(box ui.Element, n Node) ui.Element {
	align := map[string]ui.Align{"center": ui.Center, "end": ui.End, "start": ui.Start, "stretch": ui.Stretch, "spaceBetween": ui.SpaceBetween, "spaceAround": ui.SpaceAround, "spaceEvenly": ui.SpaceEvenly}
	box.MinWidth(0)
	a := n.Accessibility
	if a.Label.String() != "" {
		box.Role(ui.RoleGroup).Label(a.Label.String())
	}
	if a.Description.String() != "" {
		box.Description(a.Description.String())
	}
	if string(a.Hidden) == "true" {
		box.AccessHidden(true)
	}
	switch a.Live {
	case "polite":
		box.Role(ui.RoleStatus)
	case "assertive":
		box.Role(ui.RoleAlert)
	}
	if n.Weight > 0 {
		box.Grow(float32(n.Weight)).Basis(0)
	}
	if a, ok := align[n.Align]; ok {
		box.AlignItems(a)
	}
	if a, ok := align[n.Justify]; ok {
		box.Justify(a)
	}
	return box
}

var decimal = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)$`)

func number(cell string) (float64, bool) {
	clean := strings.Map(func(r rune) rune {
		if strings.ContainsRune(",%$€£¥", r) || unicode.IsSpace(r) {
			return -1
		}
		return r
	}, cell)
	if !decimal.MatchString(clean) {
		return 0, false
	}
	v, err := strconv.ParseFloat(clean, 64)
	return v, err == nil
}

func comparison(c *ui.Context, n Node, st *componentState) {
	name := strings.TrimSpace(n.Title + " table")
	if n.Title == "" {
		name = "Comparison table"
	}
	ui.ScrollHorizontal(c).TrackScroll(&st.scroll).Role(ui.RoleGroup).Label(name + ", scrollable").Focusable().MinWidth(0).Children(func() {
		ui.Column(c).Shrink(0).Role(ui.RoleTable).Label(name).RowCount(len(n.Rows) + 1).ColumnCount(len(n.Columns)).Children(func() {
			ui.Row(c).Role(ui.RoleRow).Children(func() {
				for i, label := range n.Columns {
					if strings.TrimSpace(label) == "" {
						label = fmt.Sprintf("Column %d", i+1)
					}
					order := ui.SortNone
					mark := " ↕"
					if st.column == i && st.order != 0 {
						if st.order == 1 {
							order, mark = ui.SortAscending, " ↑"
						} else {
							order, mark = ui.SortDescending, " ↓"
						}
					}
					ui.Column(c.Key(i)).Width(128).Shrink(0).Role(ui.RoleColumnHeader).Label(label).Sort(order).Children(func() {
						if ui.Button(c, label+mark).Label("Sort by " + label).Clicked() {
							if st.column != i {
								st.order = 0
							}
							st.column = i
							st.order = (st.order + 1) % 3
						}
					})
				}
			})
			rows := slices.Clone(n.Rows)
			if st.order != 0 && st.column < len(n.Columns) {
				collator := collate.New(language.English, collate.Numeric, collate.IgnoreCase)
				slices.SortStableFunc(rows, func(a, b []string) int {
					x, y := a[st.column], b[st.column]
					left, lok := number(x)
					right, rok := number(y)
					var result int
					if lok && rok {
						result = cmp.Compare(left, right)
					} else {
						result = collator.CompareString(x, y)
					}
					if st.order == 2 {
						return -result
					}
					return result
				})
			}
			for _, row := range rows {
				ui.Row(c).Role(ui.RoleRow).Children(func() {
					for i, value := range row {
						role := ui.RoleCell
						if i == 0 {
							role = ui.RoleRowHeader
						}
						ui.Text(c, value).Width(128).Shrink(0).Padding(5).Role(role).Label(value).Selectable()
					}
				})
			}
		})
	})
}
