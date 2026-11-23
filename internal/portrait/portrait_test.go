package portrait_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestFontStacksMatchTheWebUIBaseline(t *testing.T) {
	for _, tc := range []struct{ name, got, want string }{
		{"sans", portrait.FontSans, "IBM Plex Sans, IBM Plex Sans SC"},
		{"display", portrait.FontDisplay, "Barlow Semi Condensed, IBM Plex Sans, IBM Plex Sans SC"},
		{"mono", portrait.FontMono, "IBM Plex Mono, IBM Plex Sans SC"},
		{"icons", portrait.FontIcons, "Phosphor"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s stack = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

func TestTypeScaleMatchesTheWebUIBaseline(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		got                     portrait.TypeStyle
		size, leading, tracking float32
		family                  string
	}{
		{"display", portrait.Type.Display, 28, 36, 0, portrait.FontSans},
		{"title", portrait.Type.Title, 20, 28, 0, portrait.FontSans},
		{"reading", portrait.Type.Reading, 16, 28, 0.16, portrait.FontSans},
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

func TestDisplayFamilyIsSizeAdjustedLikeMGGCondensed(t *testing.T) {
	h1 := portrait.Type.Display.In(portrait.FontDisplay).Font(600)
	if h1.Family != portrait.FontDisplay || !near(h1.Size, 29.4) || h1.Weight != 600 {
		t.Fatalf("display h1 font %+v, want 29.4px Barlow stack", h1)
	}
	if got := portrait.Type.Display.In(portrait.FontDisplay).Leading; got != 36 {
		t.Fatalf("display leading %v, want 36", got)
	}
	if got := portrait.Type.Display.In(portrait.FontDisplay).LineHeight(); !near(got, 36/29.4) {
		t.Fatalf("display line height multiple %v, want %v", got, 36/29.4)
	}
	h2 := portrait.Type.Title.In(portrait.FontDisplay).Font(600)
	if !near(h2.Size, 21) {
		t.Fatalf("display h2 size %v, want 21", h2.Size)
	}
	if body := portrait.Type.Reading.Font(400); body.Size != 16 || body.Family != portrait.FontSans {
		t.Fatalf("reading font %+v", body)
	}
	if code := portrait.Type.Code.LineHeight(); !near(code, 22.0/13) {
		t.Fatalf("code line height %v, want 22/13", code)
	}
}

func TestColorsMatchTheWebUIBaseline(t *testing.T) {
	c := portrait.Colors
	for _, tc := range []struct {
		name string
		got  ui.Color
		want ui.Color
	}{
		{"chrome", c.Chrome, ui.Hex("#f2f2f4")},
		{"paper", c.Paper, ui.Hex("#ffffff")},
		{"code", c.Code, ui.Hex("#f6f6f8")},
		{"ink", c.Ink, ui.Hex("#1d1d1f")},
		{"ink-2", c.Ink2, ui.Hex("#48484d")},
		{"ink-3", c.Ink3, ui.Hex("#6e6e73")},
		{"ink-4", c.Ink4, ui.Hex("#a1a1a6")},
		{"line", c.Line, ui.RGBA(0, 0, 0, 0.08)},
		{"line-strong", c.LineStrong, ui.RGBA(0, 0, 0, 0.16)},
		{"fill", c.Fill, ui.RGBA(0, 0, 0, 0.045)},
		{"fill-hover", c.FillHover, ui.RGBA(0, 0, 0, 0.07)},
		{"fill-strong", c.FillStrong, ui.RGBA(0, 0, 0, 0.11)},
		{"accent", c.Accent, ui.Hex("#2f6f62")},
		{"on-accent", c.OnAccent, ui.Hex("#ffffff")},
		{"accent-soft", c.AccentSoft, ui.RGBA(47, 111, 98, 0.1)},
		{"warn", c.Warn, ui.Hex("#9a4d12")},
		{"warn-soft", c.WarnSoft, ui.RGBA(154, 77, 18, 0.09)},
		{"danger", c.Danger, ui.Hex("#a83a3a")},
		{"danger-soft", c.DangerSoft, ui.RGBA(168, 58, 58, 0.08)},
		{"content-note", c.Note, ui.Hex("#2f6f62")},
		{"content-note-soft", c.NoteSoft, ui.RGBA(47, 111, 98, 0.1)},
		{"content-warning", c.Warning, ui.Hex("#9a4d12")},
		{"content-warning-soft", c.WarningSoft, ui.RGBA(154, 77, 18, 0.09)},
		{"content-caution", c.Caution, ui.Hex("#a83a3a")},
		{"content-caution-soft", c.CautionSoft, ui.RGBA(168, 58, 58, 0.08)},
		{"focus", c.Focus, ui.RGBA(47, 111, 98, 0.34)},
		{"scrim", c.Scrim, ui.RGBA(29, 29, 31, 0.08)},
	} {
		if !sameColor(tc.got, tc.want) {
			t.Errorf("%s = %+v, want %+v", tc.name, tc.got, tc.want)
		}
	}
}

func TestMaterialsMatchTheWebUIBaseline(t *testing.T) {
	black := func(a float32) ui.Color { return ui.RGBA(0, 0, 0, a) }
	white := func(a float32) ui.Color { return ui.RGBA(255, 255, 255, a) }
	in := func(x, y, blur, spread float32, c ui.Color) portrait.Shadow {
		return portrait.Shadow{Inset: true, X: x, Y: y, Blur: blur, Spread: spread, Color: c}
	}
	out := func(x, y, blur, spread float32, c ui.Color) portrait.Shadow {
		return portrait.Shadow{X: x, Y: y, Blur: blur, Spread: spread, Color: c}
	}
	m := portrait.Materials
	for _, tc := range []struct {
		name     string
		got      portrait.Surface
		from, to ui.Color
		shadows  []portrait.Shadow
	}{
		{"raised", m.Raised, ui.Hex("#ffffff"), ui.Hex("#f6f6f8"), []portrait.Shadow{
			in(0, 1, 0, 0, white(0.94)), in(0, -0.5, 0, 0, black(0.05)), out(0, 0, 0, 0.5, black(0.13)), out(0, 1, 2, 0, black(0.055)), out(0, 3, 8, -3, black(0.075))}},
		{"raised-hover", m.RaisedHover, ui.Hex("#ffffff"), ui.Hex("#f6f6f8"), []portrait.Shadow{
			in(0, 1, 0, 0, ui.Hex("#ffffff")), in(0, -0.5, 0, 0, black(0.06)), out(0, 0, 0, 0.5, black(0.15)), out(0, 2, 4, 0, black(0.065)), out(0, 6, 14, -4, black(0.095))}},
		{"raised-press", m.RaisedPress, ui.Hex("#ededf0"), ui.Hex("#f4f4f6"), []portrait.Shadow{
			in(0, 1, 2, 0, black(0.1)), in(0, -0.5, 0, 0, white(0.85)), out(0, 0, 0, 0.5, black(0.15)), out(0, 0.5, 1, 0, black(0.04)), out(0, 0, 0, 0, ui.Transparent)}},
		{"accent-raised", m.AccentRaised, ui.Hex("#37796b"), ui.Hex("#2c6a5d"), []portrait.Shadow{
			in(0, 1, 0, 0, white(0.24)), in(0, -1, 0, 0, black(0.18)), out(0, 0, 0, 0.5, ui.RGBA(18, 52, 45, 0.42)), out(0, 1, 2, 0, ui.RGBA(18, 52, 45, 0.16)), out(0, 4, 10, -3, ui.RGBA(47, 111, 98, 0.34))}},
		{"accent-raised-hover", m.AccentRaisedHover, ui.Hex("#3c8273"), ui.Hex("#2e6e60"), []portrait.Shadow{
			in(0, 1, 0, 0, white(0.28)), in(0, -1, 0, 0, black(0.2)), out(0, 0, 0, 0.5, ui.RGBA(18, 52, 45, 0.48)), out(0, 2, 4, 0, ui.RGBA(18, 52, 45, 0.18)), out(0, 7, 16, -4, ui.RGBA(47, 111, 98, 0.38))}},
		{"accent-raised-press", m.AccentRaisedPress, ui.Hex("#2a6357"), ui.Hex("#317063"), []portrait.Shadow{
			in(0, 1, 2, 0, black(0.28)), in(0, -0.5, 0, 0, white(0.14)), out(0, 0, 0, 0.5, ui.RGBA(18, 52, 45, 0.48)), out(0, 0.5, 1, 0, ui.RGBA(18, 52, 45, 0.14)), out(0, 0, 0, 0, ui.Transparent)}},
		{"row", m.Row, ui.Hex("#ffffff"), ui.Hex("#f6f6f8"), []portrait.Shadow{
			in(0, 1, 0, 0, white(0.92)), out(0, 0, 0, 0.5, black(0.095)), out(0, 1, 2, 0, black(0.06))}},
		{"sunken", m.Sunken, black(0.045), black(0.045), []portrait.Shadow{
			in(0, 1, 2, 0, black(0.085)), in(0, 0, 0, 0.5, black(0.055)), out(0, 1, 0, 0, white(0.72))}},
		{"track", m.Track, black(0.045), black(0.045), []portrait.Shadow{
			in(0, 1, 1.5, 0, black(0.045)), in(0, 0, 0, 0.5, black(0.055))}},
		{"sheet", m.Sheet, ui.Hex("#ffffff"), ui.Hex("#ffffff"), []portrait.Shadow{
			out(0, 0, 0, 0.5, black(0.09)), out(0, 1, 3, 0, black(0.05))}},
		{"pop", m.Pop, ui.Hex("#ffffff"), ui.Hex("#ffffff"), []portrait.Shadow{
			out(0, 0, 0, 0.5, black(0.13)), out(0, 4, 12, 0, black(0.08)), out(0, 16, 36, -8, black(0.14))}},
	} {
		g := tc.got
		if !sameColor(g.From, tc.from) || !sameColor(g.To, tc.to) {
			t.Errorf("%s fill %+v → %+v, want %+v → %+v", tc.name, g.From, g.To, tc.from, tc.to)
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
}

func TestGlassMatchesTheWebUIBaseline(t *testing.T) {
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
		{"glass", portrait.Materials.Glass, white(0.72), 14, 1.6, 1.02, []portrait.Shadow{
			in(0, 1, 0, 0, white(0.96)), in(1, 0, 0, 0, white(0.68)), in(0, -0.5, 0, 0, black(0.03)), out(0, 0, 0, 0.5, black(0.075)), out(0, 2, 6, 0, black(0.035)), out(0, 12, 28, -8, black(0.07))}},
		{"glass-overlay", portrait.Materials.GlassOverlay, white(0.86), 22, 1.85, 1.03, []portrait.Shadow{
			in(0, 1, 0, 0, white(0.98)), in(1, 0, 0, 0, white(0.76)), in(0, -0.5, 0, 0, black(0.045)), out(0, 0, 0, 0.5, black(0.10)), out(0, 4, 12, 0, black(0.07)), out(0, 18, 40, -10, black(0.13))}},
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
		h := g.Highlight
		if h.Center != (ui.Point{X: 0.12, Y: 0}) || len(h.Stops) != 2 || !sameColor(h.Stops[0].Color, white(0.42)) || h.Stops[0].At != 0 || !sameColor(h.Stops[1].Color, ui.Transparent) || !near(h.Stops[1].At, 0.56) {
			t.Errorf("%s highlight %+v", tc.name, h)
		}
	}
	if s := portrait.Materials.GlassSelected; !sameColor(s, white(0.92)) {
		t.Errorf("glass-selected %+v", s)
	}
	if s := portrait.Materials.GlassHover; !sameColor(s, white(0.84)) {
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
