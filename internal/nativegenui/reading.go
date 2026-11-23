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

	"github.com/Bayern99/mark-go-go/internal/openuilang"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

type Reading struct {
	remaining                    int
	surface                      *Surface
	scroll                       ui.ScrollState
	Replacing                    []string
	states                       map[string]*componentState
	Sources                      func(*ui.Context, Node)
	SelectItem                   func(Node, string)
	CanSelect                    func(Node) bool
	LoadImage                    func(src string) (ui.ImageSource, error)
	images                       sync.Map
	form                         string
	fields                       map[string][]checkedField
	problems                     map[string]string
	drawingProblems              map[string]string
	presses                      []press
	changed                      bool
	pressable                    int
	touched                      map[string]bool
	fits                         map[measuredText]textFit
	session                      *openuilang.Session
	created                      []string
	loads, cards                 uint64
	inCard                       bool
	halo, haloShown              uint64
	haloAt                       time.Time
	jumpID                       string
	jumpRun, jumpDrawn, jumpDone uint64
	jumpAt                       time.Time
	jumpElement                  *openuilang.Element
}

type cardIn struct{ load uint64 }
type haloScale struct{ run uint64 }
type haloFade struct{ run uint64 }
type rimTurn struct{ run uint64 }
type rimFade struct{ run uint64 }
type shimmer struct{}

type jumpFill struct {
	base, tint ui.Color
}

func (f jumpFill) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	if f.base.A > 0 {
		p.FillRadii(box, radii, f.base)
	}
	if f.tint.A > 0 {
		p.FillRadii(box, radii, f.tint)
	}
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
	pointerPressed     bool
	spanned            spannedWidth
}

func (r *Reading) Load(s *Surface) {
	r.surface = s
	if s != nil && s.Session != nil {
		if r.session != nil && slices.Equal(r.created, s.created) {
			s.resume(r.session)
		} else {
			r.touched = nil
			r.loads++
		}
		r.session, r.created = s.Session, s.created
	} else if s != nil {
		r.loads++
	}
	r.images.Clear()
	r.problems, r.presses, r.changed, r.form = map[string]string{}, nil, false, ""
	if r.touched == nil {
		r.touched = map[string]bool{}
	}
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
	r.cards = 0
	r.jumpDrawn = 0
	ui.Column(c).Grow(1).MinHeight(0).Clip().Children(func() {
		ui.Scroll(c).TrackScroll(&r.scroll).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Padding(24).Gap(portrait.GenUI.CardGap).Children(func() {
				if r.surface.Session != nil {
					r.fields = map[string][]checkedField{}
					r.jumpElement = r.surface.jumpTarget(r.jumpID)
					r.statements(c)
					r.settle(c)
					return
				}
				r.node(c, "root", map[string]bool{})
			})
		})
		r.drawHalo(c)
	})
}

func (r *Reading) Halo() { r.halo++ }

// Highlight fades the current instance of a component's card once.
func (r *Reading) Highlight(id string) {
	r.jumpRun++
	r.jumpID = id
	r.jumpAt = time.Time{}
	r.jumpElement = nil
}

func (r *Reading) jump(c *ui.Context, e ui.Element, base ui.Color, target bool) ui.Element {
	if !target || r.jumpID == "" || r.jumpDrawn == r.jumpRun {
		return e
	}
	r.jumpDrawn = r.jumpRun
	if !portrait.Animates(c, false) {
		r.jumpDone, r.jumpAt = r.jumpRun, time.Time{}
		return e
	}
	if r.jumpDone == r.jumpRun {
		return e
	}
	if r.jumpAt.IsZero() {
		r.jumpAt = c.Now()
	}
	h := portrait.Highlights.Jump
	elapsed := c.Now().Sub(r.jumpAt)
	if elapsed >= h.Duration {
		r.jumpDone, r.jumpAt = r.jumpRun, time.Time{}
		return e
	}
	c.AnimationFrame()
	return e.Material(jumpFill{base: base, tint: h.At(elapsed)})
}

func (r *Reading) jumpNode(c *ui.Context, e ui.Element, n Node, framed bool) ui.Element {
	target := n.ComponentID == r.jumpID || n.ID == r.jumpID
	base := ui.Transparent
	if framed {
		base = portrait.GenUI.Card.From
	}
	return r.jump(c, e, base, target)
}

func (r *Reading) drawHalo(c *ui.Context) {
	if r.halo == r.haloShown {
		return
	}
	h, rim := portrait.GenUI.Halo, portrait.GenUI.Rim
	if !portrait.Animates(c, false) {
		r.haloShown = r.halo
		return
	}
	if r.haloAt.IsZero() {
		r.haloAt = c.Now()
	}
	if c.Now().Sub(r.haloAt) >= max(h.Duration, rim.Duration) {
		r.haloShown, r.haloAt = r.halo, time.Time{}
		return
	}
	run := r.halo
	glow := ui.Box(c.Key("halo")).Absolute().Top(-h.Inset).Left(-h.Inset).Right(-h.Inset).Bottom(-h.Inset).
		Radius(12).RadialGradient(h.Fill).IgnoreInput(true).AccessHidden(true).ZIndex(2)
	scale := glow.Keyframes(haloScale{run}, []ui.Keyframe{{At: 0, Value: h.From}, {At: h.Duration, Value: h.To, Ease: h.Ease.Easing()}})
	glow.Opacity(glow.Keyframes(haloFade{run}, h.Opacity())).TransformOrigin(h.Center.X, h.Center.Y).Scale(scale, scale)
	edge := ui.Box(c.Key("rim")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(portrait.GenUI.CardRadius).
		Border(rim.Width, ui.Color{}).IgnoreInput(true).AccessHidden(true).ZIndex(2)
	angle := edge.Keyframes(rimTurn{run}, []ui.Keyframe{{At: 0, Value: rim.From}, {At: rim.Duration, Value: rim.To, Ease: rim.Ease.Easing()}})
	edge.BorderConicGradient(ui.ConicGradient{Stops: rim.Stops, Angle: angle}).Opacity(edge.Keyframes(rimFade{run}, rim.Opacity()))
}

func (r *Reading) card(c *ui.Context, e ui.Element, framed bool) ui.Element {
	g := portrait.GenUI
	if framed {
		g.Card.Apply(e.Radius(g.CardRadius).Padding(g.CardPadding))
	}
	i := r.cards
	r.cards++
	if !portrait.Animates(c, false) {
		return e
	}
	m := g.Enter
	p := e.Keyframes(cardIn{r.loads}, []ui.Keyframe{{At: 0, Value: 0}, {At: m.Duration, Value: 1, Ease: m.Ease.Easing()}}, ui.KeyframeOptions{Delay: m.Delay(int(i))})
	if p < 1 {
		e.Opacity(p).Translate(0, (1-p)*m.Rise)
		if blur := (1 - p) * m.Blur; blur > 0.01 {
			e.Blur(blur)
		}
	}
	return e
}

const (
	rowCardBasis   = 260
	metricMinWidth = 140
	metricGap      = 12
)

func cardTitle(c *ui.Context, title string) {
	portrait.Type.Meta.In(portrait.FontDisplay).Apply(ui.Text(c, strings.ToUpper(title))).
		FontWeight(600).LetterSpacing(0.07 * portrait.Type.Meta.Size).TextColor(portrait.Colors.Ink3).Role(ui.RoleHeading).Label(title)
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
		g := portrait.GenUI
		card := layout(ui.Column(c).Gap(8).Padding(g.CardPadding).Radius(g.CardRadius), n)
		if n.Weight > 0 {
			card.Basis(rowCardBasis)
		}
		g.Card.Apply(card)
		r.jumpNode(c, card, n, true).Children(func() {
			outer := r.inCard
			r.inCard = true
			r.node(c, n.Child, parents)
			r.inCard = outer
		})
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
		frame := r.card(c, layout(ui.Column(c).Gap(8), n).Bind(&state.frame).Busy(busy), !r.inCard)
		frame = r.jumpNode(c, frame, n, !r.inCard)
		if busy {
			frame.Background(portrait.GenUI.Replacing).MinHeight(max(64, state.height)).Children(func() {
				if n.Title != "" {
					cardTitle(c, n.Title)
				}
				updatingStatus(c)
				cardSkeleton(c)
			})
			return
		}
		frame.Children(func() {
			if n.Title != "" {
				cardTitle(c, n.Title)
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
						ui.Row(c.Key(i)).Gap(8).AlignItems(ui.Start).Children(func() {
							portrait.Type.Meta.In(portrait.FontDisplay).Apply(ui.Text(c, fmt.Sprintf("%02d", i+1))).FontWeight(600).FontFeatures("tnum").
								TextColor(portrait.Colors.Ink3).Width(24).Shrink(0).Padding(4, 0, 0, 0).AccessHidden(true)
							portrait.Type.Reading.Apply(ui.Text(c, s)).Grow(1).MinWidth(0).Selectable()
						})
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
				width := state.spanned.lastDrawn(c)
				columns := len(n.Metrics)
				if width > 0 {
					columns = max(1, min(columns, int((width+metricGap)/(metricMinWidth+metricGap))))
				}
				ui.Grid(c).Bind(&state.spanned.frame).Columns(columns).Gap(metricGap).AlignItems(ui.Stretch).Children(func() {
					for i, m := range n.Metrics {
						ui.Column(c.Key(i)).Gap(2).MinWidth(0).Children(func() {
							portrait.Type.Meta.Apply(ui.Text(c, m.Label)).TextColor(portrait.Colors.Ink3)
							ui.Row(c).Gap(3).AlignItems(ui.Baseline).Children(func() {
								portrait.Type.Title.In(portrait.FontDisplay).Apply(ui.Text(c, m.Value)).FontWeight(600).FontFeatures("lnum", "tnum")
								if m.Unit != "" {
									portrait.Type.UI.Apply(ui.Text(c, m.Unit)).TextColor(portrait.Colors.Ink3)
								}
							})
							if m.Change != "" {
								change := portrait.Colors.Ink3
								switch m.Trend {
								case "up":
									change = portrait.Colors.Accent
								case "down":
									change = portrait.Colors.Danger
								}
								mark := map[string]string{"up": "▲ ", "down": "▼ ", "flat": "■ "}[m.Trend]
								portrait.Type.Meta.Apply(ui.Text(c, mark+m.Change)).Label(strings.TrimSpace(m.Trend + " " + m.Change)).TextColor(change)
							}
						})
					}
				})
			case "ComparisonTable":
				comparison(c, n, state)
			case "Timeline":
				ui.Column(c).Children(func() {
					for i, e := range n.Events {
						ui.Row(c.Key(i)).Gap(12).AlignItems(ui.Stretch).Children(func() {
							portrait.Type.Meta.Apply(ui.Text(c, e.When)).FontWeight(600).TextColor(portrait.Colors.Accent).MinWidth(54).Shrink(0).Padding(6, 0, 0, 0)
							ui.Column(c).Grow(1).MinWidth(0).Padding(0, 0, 12, 12).InsetShadow(2, 0, 0, 0, portrait.Colors.LineStrong).Children(func() {
								portrait.Type.Reading.Apply(ui.Text(c, e.Title)).FontWeight(600)
								if e.Detail != "" {
									portrait.Type.Reading.Apply(ui.Text(c, e.Detail)).TextColor(portrait.Colors.Ink3).Selectable()
								}
							})
						})
					}
				})
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
					status := "Incomplete. Show in source."
					if item.Done {
						status = "Completed. Show in source."
					}
					disabled := r.SelectItem == nil || (r.CanSelect != nil && !r.CanSelect(n))
					b := ui.ButtonBase(c.Key(i)).Label(item.Text).Description(status).Disabled(disabled).DimDisabled(false).Gap(8).Radius(6).Padding(4, 6).AlignItems(ui.Start)
					if b.Hovered() && !disabled {
						b.Background(portrait.Colors.AccentSoft)
					}
					b.Children(func() {
						box := ui.Row(c).Size(16, 16).Shrink(0).Margin(6, 0, 0, 0).Radius(4).Border(1, portrait.Colors.Ink3).AlignItems(ui.Center).Justify(ui.Center)
						ink := portrait.Colors.Ink
						if item.Done {
							ink = portrait.Colors.Ink3
							box.Background(portrait.Colors.Accent).Border(1, portrait.Colors.Accent).Children(func() {
								portrait.Glyph(c, portrait.IconCheck, 11, portrait.Colors.OnAccent)
							})
						}
						portrait.Type.Reading.Apply(ui.Text(c, item.Text)).TextColor(ink).Grow(1).MinWidth(0)
					})
					if b.Clicked() {
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

func updatingStatus(c *ui.Context) ui.Element {
	return portrait.Type.Meta.Apply(ui.Text(c, "Updating card…")).TextColor(portrait.Colors.Ink3).Role(ui.RoleStatus)
}

func skeleton(c *ui.Context, bars int) {
	for i, width := range []float32{82, 64, 38}[:bars] {
		shimmering(c, ui.Box(c.Key(i)).WidthPercent(width).Height(7))
	}
}

func shimmering(c *ui.Context, e ui.Element) ui.Element {
	s := portrait.GenUI.Shimmer
	e.Radius(portrait.GenUI.SkeletonRadius).AccessHidden(true)
	if !portrait.Loops(c) {
		return e.Background(s.Base)
	}
	return e.LinearGradient(s.Gradient(e.Loop(shimmer{}, s.Period, ui.Linear)))
}

func viewSkeleton(c *ui.Context) {
	ui.Column(c.Key("skeleton")).Gap(12).AccessHidden(true).Children(func() {
		shimmering(c, ui.Box(c).WidthPercent(60).Height(14))
		ui.Row(c).Wrap().Gap(12).Children(func() {
			for i := range 2 {
				shimmering(c, ui.Box(c.Key(i)).Grow(1).Basis(220).Height(96))
			}
		})
		shimmering(c, ui.Box(c).FillWidth().Height(96))
	})
}

func cardSkeleton(c *ui.Context) {
	ui.Column(c.Key("skeleton")).Gap(8).Grow(1).AccessHidden(true).Children(func() {
		shimmering(c, ui.Box(c).WidthPercent(60).Height(14))
		shimmering(c, ui.Box(c).Grow(1).MinHeight(54))
	})
}

var readOnlyMarkdown = markdown.Options{OnLink: func(string) {}, Theme: &markdown.Theme{FontSize: portrait.Type.Small.Size, CodeFont: portrait.FontMono, CodeSize: portrait.Type.Meta.Size / portrait.Type.Small.Size}}
var captionMarkdown = markdown.Options{OnLink: func(string) {}, Theme: &markdown.Theme{FontSize: portrait.Type.Meta.Size, CodeFont: portrait.FontMono, CodeSize: portrait.Type.Meta.Size / portrait.Type.Meta.Size}}

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
