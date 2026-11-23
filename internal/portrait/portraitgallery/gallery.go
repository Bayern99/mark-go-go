// Package portraitgallery builds the Portrait component state matrix for
// structural tests, relational pixel tests, and local visual review.
package portraitgallery

import (
	"fmt"
	"image"
	"image/draw"
	"slices"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

// Component identifies a registered gallery component.
type Component string

// State identifies one column in a component sheet.
type State string

const (
	Rest         State = "rest"
	Hover        State = "hover"
	Pressed      State = "pressed"
	FocusVisible State = "focus-visible"
	Disabled     State = "disabled"
	Loading      State = "loading"
	Selected     State = "selected"
	Invalid      State = "invalid"
	LongCJK      State = "long-cjk"
)

// States is the fixed column order used by sheets and PNG exports.
var States = []State{Rest, Hover, Pressed, FocusVisible, Disabled, Loading, Selected, Invalid, LongCJK}

// Preference selects the system preferences used to render a cell.
type Preference string

const (
	Default            Preference = "default"
	HighContrast       Preference = "high-contrast"
	ReduceTransparency Preference = "reduce-transparency"
)

// Preferences is the preference axis of the gallery matrix.
var Preferences = []Preference{Default, HighContrast, ReduceTransparency}

// Surface identifies the material behind a component.
type Surface string

const (
	Paper  Surface = "paper"
	Chrome Surface = "chrome"
	Glass  Surface = "glass"
)

// Surfaces is the surface axis of the gallery matrix.
var Surfaces = []Surface{Paper, Chrome, Glass}

// Cell is one component/state/preference/surface combination.
type Cell struct {
	Component  Component
	State      State
	Preference Preference
	Surface    Surface

	// drive is set only by the headless renderer. Live previews must not
	// steal focus from the person inspecting the other cells.
	drive bool
}

// Definition registers a component and its structural contract.
type Definition struct {
	Name             Component
	Role             ui.Role
	Label, LongLabel string
	AllowedSurfaces  []Surface

	build func(*ui.Context, Cell) ui.Element
}

var registry = struct {
	order  []Component
	byName map[Component]Definition
}{byName: make(map[Component]Definition)}

func register(d Definition) {
	if d.Name == "" || d.Role == ui.RoleNone || d.Label == "" || d.LongLabel == "" || d.build == nil {
		panic("portraitgallery: incomplete definition " + string(d.Name))
	}
	if _, exists := registry.byName[d.Name]; exists {
		panic("portraitgallery: duplicate component " + string(d.Name))
	}
	d.AllowedSurfaces = slices.Clone(d.AllowedSurfaces)
	registry.order = append(registry.order, d.Name)
	registry.byName[d.Name] = d
}

// Components returns the registered components in registration order.
func Components() []Definition {
	out := make([]Definition, 0, len(registry.order))
	for _, name := range registry.order {
		d, _ := Lookup(name)
		out = append(out, d)
	}
	return out
}

// Lookup returns a component definition without exposing registry storage.
func Lookup(name Component) (Definition, bool) {
	d, ok := registry.byName[name]
	d.AllowedSurfaces = slices.Clone(d.AllowedSurfaces)
	return d, ok
}

func normalize(cell Cell) Cell {
	if cell.State == "" {
		cell.State = Rest
	}
	if cell.Preference == "" {
		cell.Preference = Default
	}
	if cell.Surface == "" {
		cell.Surface = Paper
	}
	return cell
}

// ValidateSurface checks a component's surface constraint. A nil or empty
// AllowedSurfaces list permits every surface.
func (d Definition) ValidateSurface(surface Surface) error {
	if !slices.Contains(Surfaces, surface) {
		return fmt.Errorf("portraitgallery: unknown surface %q", surface)
	}
	if len(d.AllowedSurfaces) != 0 && !slices.Contains(d.AllowedSurfaces, surface) {
		return fmt.Errorf("portraitgallery: %s allows only %v, not %s", d.Name, d.AllowedSurfaces, surface)
	}
	return nil
}

// Validate checks all cell axes, including the component's surface contract.
func Validate(cell Cell) error {
	cell = normalize(cell)
	d, ok := Lookup(cell.Component)
	if !ok {
		return fmt.Errorf("portraitgallery: unknown component %q", cell.Component)
	}
	if !slices.Contains(States, cell.State) {
		return fmt.Errorf("portraitgallery: unknown state %q", cell.State)
	}
	if !slices.Contains(Preferences, cell.Preference) {
		return fmt.Errorf("portraitgallery: unknown preference %q", cell.Preference)
	}
	return d.ValidateSurface(cell.Surface)
}

// Build constructs the production control for cell. Invalid cells are
// programming errors and panic before any content is built.
func Build(c *ui.Context, cell Cell) ui.Element {
	cell = normalize(cell)
	if err := Validate(cell); err != nil {
		panic(err)
	}
	d, _ := Lookup(cell.Component)
	return d.build(c, cell).MaxWidth(cellWidth - 24)
}

func labelFor(cell Cell) string {
	d, _ := Lookup(cell.Component)
	if cell.State == LongCJK {
		return d.LongLabel
	}
	return d.Label
}

func controlState(e ui.Element, cell Cell) ui.Element {
	if cell.State == Disabled {
		e.Disabled(true)
	}
	if cell.State == FocusVisible && cell.drive {
		e.Focus()
	}
	return e
}

const (
	cellWidth    = 144
	cellHeight   = 104
	headerHeight = 28
)

func cellName(cell Cell) string {
	return fmt.Sprintf("%s / %s / %s / %s", cell.Component, cell.State, cell.Preference, cell.Surface)
}

func sheetHeader(c *ui.Context, component Component, preference Preference, surface Surface) ui.Element {
	theme := portrait.Theme()
	e := ui.Themed(c, &theme).Size(float32(cellWidth*len(States)), headerHeight).Background(portrait.Colors.Chrome).Padding(5, 10)
	e.Children(func() {
		portrait.Type.Meta.Apply(ui.Text(c, fmt.Sprintf("%s · %s · %s", component, preference, surface))).TextColor(portrait.Colors.Ink2)
	})
	return e
}

func cellView(c *ui.Context, cell Cell) ui.Element {
	cell = normalize(cell)
	theme := portrait.Theme()
	root := ui.Themed(c, &theme).Size(cellWidth, cellHeight).Role(ui.RoleGroup).Label(cellName(cell))
	root.Children(func() {
		if cell.Surface == Glass {
			root.Background(portrait.Colors.Chrome)
			ui.Box(c).Absolute().Left(12).Top(32).Size(58, 60).Background(portrait.Tones.Accent.Soft).IgnoreInput(true).AccessHidden(true)
			ui.Box(c).Absolute().Right(8).Top(24).Size(48, 68).Background(portrait.Tones.Info.Wash).IgnoreInput(true).AccessHidden(true)
		}
		panel := ui.Column(c).Fill().Padding(7, 9).Gap(5)
		switch cell.Surface {
		case Paper:
			panel.Background(portrait.Colors.Paper)
		case Chrome:
			panel.Background(portrait.Colors.Chrome)
		case Glass:
			portrait.Materials.Glass.Apply(panel)
		}
		panel.Children(func() {
			portrait.Type.Meta.Apply(ui.Text(c, string(cell.State))).TextColor(portrait.Colors.Ink3).MaxLines(1)
			ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				Build(c, cell)
			})
		})
	})
	return root
}

// Sheet builds a live, inspectable row of every state for one component.
// Pointer and keyboard input operate on the real controls in the preview.
func Sheet(c *ui.Context, component Component, preference Preference, surface Surface) ui.Element {
	cell := normalize(Cell{Component: component, Preference: preference, Surface: surface})
	if err := Validate(cell); err != nil {
		panic(err)
	}
	return ui.Column(c.Key("sheet:" + string(component) + ":" + string(preference) + ":" + string(surface))).AlignItems(ui.Start).Children(func() {
		sheetHeader(c, component, cell.Preference, cell.Surface)
		ui.Row(c).Children(func() {
			for _, state := range States {
				cellView(c, Cell{Component: component, State: state, Preference: cell.Preference, Surface: cell.Surface})
			}
		})
	})
}

type cellRender struct {
	image  *image.RGBA
	tester *ui.Tester
	cell   Cell
	def    Definition
	target ui.AccessInfo
}

func preferences(p Preference) ui.Preferences {
	return ui.Preferences{HighContrast: p == HighContrast, ReduceTransparency: p == ReduceTransparency}
}

func renderCell(cell Cell) (cellRender, error) {
	cell = normalize(cell)
	cell.drive = true
	if err := Validate(cell); err != nil {
		return cellRender{}, err
	}
	tt := ui.NewTesterWith(func(c *ui.Context) { cellView(c, cell) }, cellWidth, cellHeight, ui.TesterOptions{Preferences: preferences(cell.Preference)})
	d, _ := Lookup(cell.Component)
	target, ok := tt.AccessByRole(d.Role, labelFor(cell))
	if !ok {
		return cellRender{}, fmt.Errorf("portraitgallery: %s has no %v named %q", cellName(cell), d.Role, labelFor(cell))
	}
	cx, cy := target.Bounds.X+target.Bounds.W/2, target.Bounds.Y+target.Bounds.H/2
	switch cell.State {
	case Hover:
		tt.Move(cx, cy)
		tt.Advance(time.Second)
	case Pressed:
		tt.Move(cx, cy)
		tt.Press(cx, cy)
		tt.Advance(300 * time.Millisecond)
	case FocusVisible:
		// Build keeps focus on this cell; Tab makes that focus visible.
		tt.Key(0, ui.KeyTab)
		tt.Frame()
	case Loading:
		tt.Advance(225 * time.Millisecond)
	}
	return cellRender{image: tt.Image(), tester: tt, cell: cell, def: d, target: target}, nil
}

func renderSheet(component Component, preference Preference, surface Surface, inspect func(cellRender)) (*image.RGBA, error) {
	cell := normalize(Cell{Component: component, Preference: preference, Surface: surface})
	if err := Validate(cell); err != nil {
		return nil, err
	}
	width := cellWidth * len(States)
	header := ui.NewTesterWith(func(c *ui.Context) { sheetHeader(c, component, cell.Preference, cell.Surface) }, width, headerHeight, ui.TesterOptions{Preferences: preferences(cell.Preference)})
	sheet := image.NewRGBA(image.Rect(0, 0, width, headerHeight+cellHeight))
	draw.Draw(sheet, image.Rect(0, 0, width, headerHeight), header.Image(), image.Point{}, draw.Src)
	for i, state := range States {
		rendered, err := renderCell(Cell{Component: component, State: state, Preference: cell.Preference, Surface: cell.Surface})
		if err != nil {
			return nil, err
		}
		if inspect != nil {
			inspect(rendered)
		}
		at := image.Rect(i*cellWidth, headerHeight, (i+1)*cellWidth, headerHeight+cellHeight)
		draw.Draw(sheet, at, rendered.image, image.Point{}, draw.Src)
	}
	return sheet, nil
}

// RenderSheet renders one component across all states for a preference and
// surface. Each state is driven independently so hover, press, focus, and
// tooltips can all appear in the same returned image.
func RenderSheet(component Component, preference Preference, surface Surface) (*image.RGBA, error) {
	return renderSheet(component, preference, surface, nil)
}
