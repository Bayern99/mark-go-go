package portrait_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestFontStacksMatchTheirLockedValues(t *testing.T) {
	for _, tc := range []struct{ name, got, want string }{
		{"sans", portrait.FontSans, "Inter, IBM Plex Sans SC"},
		{"display", portrait.FontDisplay, "Inter Display, Inter, IBM Plex Sans SC"},
		{"mono", portrait.FontMono, "IBM Plex Mono, IBM Plex Sans SC"},
		{"icons", portrait.FontIcons, "Phosphor"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s stack = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

func TestTypeScaleMatchesItsLockedValues(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		got                     portrait.TypeStyle
		size, leading, tracking float32
		family                  string
	}{
		{"display", portrait.Type.Display, 28, 36, -0.56, portrait.FontSans},
		{"title", portrait.Type.Title, 20, 28, -0.24, portrait.FontSans},
		{"reading", portrait.Type.Reading, 16, 28, 0, portrait.FontSans},
		{"small", portrait.Type.Small, 14, 22, 0, portrait.FontSans},
		{"ui", portrait.Type.UI, 13, 18, 0, portrait.FontSans},
		{"control", portrait.Type.Control, 13, 20, 0, portrait.FontSans},
		{"meta", portrait.Type.Meta, 12, 16, 0, portrait.FontSans},
		{"code", portrait.Type.Code, 13, 22, 0, portrait.FontMono},
		{"h4", portrait.Type.H4, 15, 28, 0, portrait.FontSans},
		{"h5", portrait.Type.H5, 14, 28, 0, portrait.FontSans},
		{"h6", portrait.Type.H6, 13, 28, 0, portrait.FontSans},
	} {
		s := tc.got
		if s.Size != tc.size || s.Leading != tc.leading || !near(s.Tracking, tc.tracking) || s.Family != tc.family {
			t.Errorf("%s = %+v, want %v/%v tracking %v in %q", tc.name, s, tc.size, tc.leading, tc.tracking, tc.family)
		}
	}
}

func TestNoTextStyleIsSmallerThanTwelve(t *testing.T) {
	typeStyle := reflect.TypeOf(portrait.TypeStyle{})
	typeScale := reflect.ValueOf(portrait.Type)
	for i := 0; i < typeScale.NumField(); i++ {
		field := typeScale.Field(i)
		if field.Type() != typeStyle {
			continue
		}
		if size := field.FieldByName("Size").Float(); size < 12 {
			t.Errorf("Type.%s.Size = %v, want at least 12px", typeScale.Type().Field(i).Name, size)
		}
	}

	reader := reflect.ValueOf(portrait.Reader)
	for i := 0; i < reader.NumField(); i++ {
		field := reader.Field(i)
		if field.Kind() != reflect.Float32 && field.Kind() != reflect.Float64 {
			continue
		}
		if size := field.Float(); size < 12 {
			t.Errorf("Reader.%s = %v, want at least 12px", reader.Type().Field(i).Name, size)
		}
	}
}

func TestDisplayFamilyKeepsItsSizeAndTightensTracking(t *testing.T) {
	h1 := portrait.Type.Display.In(portrait.FontDisplay)
	if font := h1.Font(600); font.Family != portrait.FontDisplay || font.Size != 28 || font.Weight != 600 {
		t.Fatalf("display h1 font %+v, want 28px Inter Display stack", font)
	}
	if h1.RenderedSize() != h1.Size || h1.Leading != 36 || !near(h1.LineHeight(), 36.0/28) {
		t.Fatalf("display h1 %+v renders at %v with line height %v, want 28/36 unadjusted", h1, h1.RenderedSize(), h1.LineHeight())
	}
	h2 := portrait.Type.Title.In(portrait.FontDisplay)
	if h2.RenderedSize() != 20 || h2.Leading != 28 {
		t.Fatalf("display h2 %+v, want 20/28", h2)
	}
	if !(h1.Tracking < h2.Tracking && h2.Tracking < 0) || !near(h1.Tracking/h1.Size, -0.02) || !near(h2.Tracking/h2.Size, -0.012) {
		t.Fatalf("tracking h1 %v h2 %v, want -0.02em then -0.012em: tighter as the size grows", h1.Tracking, h2.Tracking)
	}
	if body := portrait.Type.Reading; body.Font(400).Size != 16 || body.Family != portrait.FontSans || body.Tracking != 0 {
		t.Fatalf("reading style %+v, want untracked 16px Inter", body)
	}
	if code := portrait.Type.Code.LineHeight(); !near(code, 22.0/13) {
		t.Fatalf("code line height %v, want 22/13", code)
	}
}

func TestColorsMatchTheirLockedValues(t *testing.T) {
	c := portrait.Colors
	accent, warn, danger := ui.Hex("#2E5D8C"), ui.Hex("#815F25"), ui.Hex("#AB372F")
	for _, tc := range []struct {
		name string
		got  ui.Color
		want ui.Color
	}{
		{"chrome", c.Chrome, ui.Hex("#FFFFFF").Mix(ui.Hex("#1D1B1C"), 0.05)},
		{"paper", c.Paper, ui.Hex("#FFFFFF")},
		{"code", c.Code, ui.Hex("#FBF7EE")},
		{"ink", c.Ink, ui.Hex("#1D1B1C")},
		{"ink-2", c.Ink2, ui.Hex("#48484D")},
		{"ink-3", c.Ink3, ui.Hex("#6E6E73")},
		{"ink-4", c.Ink4, ui.Hex("#A1A1A6")},
		{"line", c.Line, ui.RGBA(0, 0, 0, 0.08)},
		{"line-strong", c.LineStrong, ui.RGBA(0, 0, 0, 0.16)},
		{"fill", c.Fill, ui.RGBA(0, 0, 0, 0.045)},
		{"fill-hover", c.FillHover, ui.RGBA(0, 0, 0, 0.07)},
		{"fill-strong", c.FillStrong, ui.RGBA(0, 0, 0, 0.11)},
		{"accent", c.Accent, accent},
		{"on-accent", c.OnAccent, ui.Hex("#FFFFFF")},
		{"accent-soft", c.AccentSoft, accent.Alpha(0.1)},
		{"success", c.Success, ui.Hex("#1A6840")},
		{"warn", c.Warn, warn},
		{"warn-soft", c.WarnSoft, warn.Alpha(0.09)},
		{"danger", c.Danger, danger},
		{"danger-soft", c.DangerSoft, danger.Alpha(0.08)},
		{"content-note", c.Note, accent},
		{"content-note-soft", c.NoteSoft, accent.Alpha(0.1)},
		{"content-warning", c.Warning, warn},
		{"content-warning-soft", c.WarningSoft, warn.Alpha(0.09)},
		{"content-caution", c.Caution, danger},
		{"content-caution-soft", c.CautionSoft, danger.Alpha(0.08)},
		{"focus", c.Focus, accent.Alpha(0.28)},
		{"scrim", c.Scrim, ui.RGBA(29, 27, 28, 0.08)},
	} {
		if !sameColor(tc.got, tc.want) {
			t.Errorf("%s = %+v, want %+v", tc.name, tc.got, tc.want)
		}
	}
}

func TestDarkColorsMatchTheirLockedValues(t *testing.T) {
	t.Cleanup(func() { portrait.Use(portrait.DefaultSpace, false) })
	portrait.Use(portrait.DefaultSpace, true)
	c := portrait.Colors
	accent := ui.Hex("#B0D8E8")
	for _, tc := range []struct {
		name string
		got  ui.Color
		want ui.Color
	}{
		{"paper", c.Paper, ui.Hex("#1A1A1A")},
		{"code", c.Code, ui.Hex("#25211C")},
		{"ink", c.Ink, ui.Hex("#F2F3F4")},
		{"ink-2", c.Ink2, ui.Hex("#CDD1D3")},
		{"ink-3", c.Ink3, ui.Hex("#959DA2")},
		{"line", c.Line, ui.RGBA(255, 255, 255, 0.1)},
		{"line-strong", c.LineStrong, ui.RGBA(255, 255, 255, 0.2)},
		{"accent", c.Accent, accent},
		{"on-accent", c.OnAccent, ui.Hex("#0E100F")},
		{"accent-soft", c.AccentSoft, accent.Alpha(0.16)},
		{"success", c.Success, ui.Hex("#A4C2A8")},
		{"warn", c.Warn, ui.Hex("#D8C37C")},
		{"danger", c.Danger, ui.Hex("#F6AD8F")},
		{"focus", c.Focus, accent.Alpha(0.4)},
	} {
		if !sameColor(tc.got, tc.want) {
			t.Errorf("%s = %+v, want %+v", tc.name, tc.got, tc.want)
		}
	}
	if g := portrait.Materials.GlassSelected; !sameColor(g, ui.Hex("#1A1A1A").Alpha(0.85)) {
		t.Errorf("dark glass-selected %+v", g)
	}
}

func TestMaterialsMatchTheirLockedValues(t *testing.T) {
	black := func(a float32) ui.Color { return ui.RGBA(0, 0, 0, a) }
	white := func(a float32) ui.Color { return ui.RGBA(255, 255, 255, a) }
	accent := ui.Hex("#2E5D8C")
	glow := func(a float32) ui.Color { return accent.Alpha(a) }
	edge := func(a float32) ui.Color { return accent.Mix(ui.RGB(0, 0, 0), 0.35).Alpha(a) }
	in := func(x, y, blur, spread float32, c ui.Color) portrait.Shadow {
		return portrait.Shadow{Inset: true, X: x, Y: y, Blur: blur, Spread: spread, Color: c}
	}
	out := func(x, y, blur, spread float32, c ui.Color) portrait.Shadow {
		return portrait.Shadow{X: x, Y: y, Blur: blur, Spread: spread, Color: c}
	}
	paper := ui.Hex("#ffffff")
	m := portrait.Materials
	for _, tc := range []struct {
		name    string
		got     portrait.Surface
		fill    ui.Color
		shadows []portrait.Shadow
	}{
		{"raised", m.Raised, paper, []portrait.Shadow{
			out(0, 0, 0, 0.5, black(0.12)), out(0, 1, 2, 0, black(0.05)), out(0, 2, 6, -2, black(0.04))}},
		{"raised-hover", m.RaisedHover, paper, []portrait.Shadow{
			out(0, 0, 0, 0.5, black(0.16)), out(0, 1, 3, 0, black(0.07)), out(0, 4, 10, -3, black(0.07))}},
		{"raised-press", m.RaisedPress, paper.Mix(ui.Hex("#1D1B1C"), 0.07), []portrait.Shadow{
			out(0, 0, 0, 0.5, black(0.18)), out(0, 0.5, 1, 0, black(0.03)), out(0, 0, 0, 0, ui.Transparent)}},
		{"accent-raised", m.AccentRaised, accent, []portrait.Shadow{
			in(0, 1, 0, 0, white(0.18)), out(0, 0, 0, 0.5, edge(0.5)), out(0, 1, 2, 0, glow(0.25)), out(0, 4, 12, -4, glow(0.35))}},
		{"accent-raised-hover", m.AccentRaisedHover, accent.Mix(paper, 0.08), []portrait.Shadow{
			in(0, 1, 0, 0, white(0.22)), out(0, 0, 0, 0.5, edge(0.5)), out(0, 2, 4, 0, glow(0.28)), out(0, 6, 16, -4, glow(0.42))}},
		{"accent-raised-press", m.AccentRaisedPress, accent.Mix(ui.RGB(0, 0, 0), 0.1), []portrait.Shadow{
			in(0, 1, 2, 0, black(0.22)), out(0, 0, 0, 0.5, edge(0.55)), out(0, 0.5, 1, 0, glow(0.2)), out(0, 0, 0, 0, ui.Transparent)}},
		{"row", m.Row, black(0.055), nil},
		{"sunken", m.Sunken, black(0.05), []portrait.Shadow{in(0, 0, 0, 0.5, black(0.05))}},
		{"track", m.Track, black(0.055), nil},
		{"sheet", m.Sheet, paper, []portrait.Shadow{
			out(0, 0, 0, 0.5, black(0.08)), out(0, 1, 2, 0, black(0.04))}},
		{"pop", m.Pop, paper, []portrait.Shadow{
			out(0, 0, 0, 0.5, black(0.10)), out(0, 4, 14, 0, black(0.08)), out(0, 18, 44, -10, black(0.16))}},
		{"page", m.Page, paper, []portrait.Shadow{
			out(0, 0, 0, 0.5, black(0.10)), out(0, 1, 2, 0, black(0.05)), out(0, 6, 20, -6, black(0.10))}},
	} {
		g := tc.got
		if !sameColor(g.From, tc.fill) || !sameColor(g.To, tc.fill) {
			t.Errorf("%s fill %+v → %+v, want one flat %+v", tc.name, g.From, g.To, tc.fill)
		}
		if len(g.Shadows) != len(tc.shadows) {
			t.Errorf("%s has %d shadow layers, want %d", tc.name, len(g.Shadows), len(tc.shadows))
			continue
		}
		for i, s := range tc.shadows {
			if !sameShadow(g.Shadows[i], s) {
				t.Errorf("%s layer %d = %+v, want %+v", tc.name, i+1, g.Shadows[i], s)
			}
		}
	}
	for name, states := range map[string][3]portrait.Surface{
		"raised": {m.Raised, m.RaisedHover, m.RaisedPress},
		"accent": {m.AccentRaised, m.AccentRaisedHover, m.AccentRaisedPress},
	} {
		if len(states[0].Shadows) != len(states[1].Shadows) || len(states[1].Shadows) != len(states[2].Shadows) {
			t.Errorf("%s rest, hover and press have different shadow layers, so they cannot blend layer by layer", name)
		}
	}
}

func TestGlassMatchesItsLockedValues(t *testing.T) {
	black := func(a float32) ui.Color { return ui.RGBA(0, 0, 0, a) }
	white := func(a float32) ui.Color { return ui.RGBA(255, 255, 255, a) }
	in := func(x, y, blur, spread float32, c ui.Color) portrait.Shadow {
		return portrait.Shadow{Inset: true, X: x, Y: y, Blur: blur, Spread: spread, Color: c}
	}
	out := func(x, y, blur, spread float32, c ui.Color) portrait.Shadow {
		return portrait.Shadow{X: x, Y: y, Blur: blur, Spread: spread, Color: c}
	}
	for _, tc := range []struct {
		name                       string
		got                        portrait.Glass
		tint                       ui.Color
		blur, saturate, brightness float32
		rim                        []portrait.Shadow
	}{
		{"glass", portrait.Materials.Glass, white(0.70), 20, 1.8, 1, []portrait.Shadow{
			in(0, 0, 0, 0.5, white(0.55)), out(0, 0, 0, 0.5, black(0.08)), out(0, 8, 32, -8, black(0.12))}},
		{"glass-overlay", portrait.Materials.GlassOverlay, white(0.85), 32, 1.8, 1, []portrait.Shadow{
			in(0, 0, 0, 0.5, white(0.60)), out(0, 0, 0, 0.5, black(0.10)), out(0, 4, 12, 0, black(0.06)), out(0, 24, 60, -12, black(0.22))}},
	} {
		g := tc.got
		if !sameColor(g.Tint, tc.tint) || g.Blur != tc.blur || !near(g.Saturate, tc.saturate) || !near(g.Brightness, tc.brightness) {
			t.Errorf("%s = tint %+v blur %v saturate %v brightness %v", tc.name, g.Tint, g.Blur, g.Saturate, g.Brightness)
		}
		if len(g.Rim) != len(tc.rim) {
			t.Errorf("%s rim has %d layers, want %d", tc.name, len(g.Rim), len(tc.rim))
			continue
		}
		for i, s := range tc.rim {
			if !sameShadow(g.Rim[i], s) {
				t.Errorf("%s rim layer %d = %+v, want %+v", tc.name, i+1, g.Rim[i], s)
			}
		}
		if len(g.Highlight.Stops) != 0 {
			t.Errorf("%s highlight %+v, want plain frosted glass without a sheen", tc.name, g.Highlight)
		}
	}
	if s := portrait.Materials.GlassSelected; !sameColor(s, white(0.85)) {
		t.Errorf("glass-selected %+v", s)
	}
	if s := portrait.Materials.GlassHover; !sameColor(s, white(0.50)) {
		t.Errorf("glass-hover %+v", s)
	}
}

func TestMotionMatchesTheWebUIBaseline(t *testing.T) {
	m := portrait.Motion
	for _, tc := range []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"press", m.Press, 100 * time.Millisecond},
		{"switch", m.Switch, 160 * time.Millisecond},
		{"expand", m.Expand, 150 * time.Millisecond},
		{"enter", m.Enter, 200 * time.Millisecond},
		{"menu", m.Menu, 150 * time.Millisecond},
		{"jump", m.Jump, 420 * time.Millisecond},
	} {
		if tc.got != tc.want {
			t.Errorf("duration-%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
	if got, want := m.EaseOut, (portrait.CubicBezier{X1: 0.23, Y1: 1, X2: 0.32, Y2: 1}); got != want {
		t.Errorf("ease-out %+v, want %+v", got, want)
	}
	if got, want := m.EaseInOut, (portrait.CubicBezier{X1: 0.32, Y1: 0.72, X2: 0, Y2: 1}); got != want {
		t.Errorf("ease-in-out %+v, want %+v", got, want)
	}
}

func TestCurvesEaseLikeTheirCSSCounterparts(t *testing.T) {
	ease := portrait.Motion.EaseOut.Easing()
	if ease(0) != 0 || ease(1) != 1 {
		t.Fatalf("endpoints %v %v", ease(0), ease(1))
	}
	if got := ease(0.5); got < 0.955 || got > 0.975 {
		t.Fatalf("ease-out(0.5) = %v, want ≈0.966", got)
	}
}

func near(a, b float32) bool { d := a - b; return d < 1e-3 && d > -1e-3 }

func sameColor(a, b ui.Color) bool { return a == b }

func sameShadow(a, b portrait.Shadow) bool {
	return a.Inset == b.Inset && near(a.X, b.X) && near(a.Y, b.Y) && near(a.Blur, b.Blur) && near(a.Spread, b.Spread) && sameColor(a.Color, b.Color)
}
