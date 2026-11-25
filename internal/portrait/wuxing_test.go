package portrait_test

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func appearanceName(dark bool) string {
	if dark {
		return "Dark"
	}
	return "Light"
}

func eachLook(t *testing.T, check func(t *testing.T, space portrait.Space, dark bool)) {
	t.Helper()
	t.Cleanup(func() { portrait.Use(portrait.DefaultSpace, false) })
	for _, space := range portrait.Spaces {
		for _, dark := range []bool{false, true} {
			portrait.Use(space, dark)
			t.Run(space.Name+"/"+appearanceName(dark), func(t *testing.T) { check(t, space, dark) })
		}
	}
}

type lockedScheme struct {
	from, to, paper, code, accent, mark, success, warn, danger string
}

func TestSpacesMatchTheirLockedValues(t *testing.T) {
	lightSuccess, lightWarn, lightDanger := "#1A6840", "#815F25", "#AB372F"
	darkSuccess, darkWarn, darkDanger := "#A4C2A8", "#D8C37C", "#F6AD8F"
	want := []struct {
		name, label string
		light, dark lockedScheme
	}{
		{"FiveElements", "Five Elements",
			lockedScheme{"#F2E6CE", "#C6E3E1", "#FFFFFF", "#FBF7EE", "#2E5D8C", "#AB372F", "#1A6840", "#815F25", "#AB372F"},
			lockedScheme{"#0E100F", "#1A3A5F", "#1A1A1A", "#25211C", "#B0D8E8", "#F6AD8F", "#A4C2A8", "#D8C37C", "#F6AD8F"}},
		{"MetalWater", "Metal–Water",
			lockedScheme{"#D0E7F9", "#CCCCD6", "#FFFFFF", "#F3F4F6", "#2E5D8C", "#2E5D8C", lightSuccess, "#685E48", lightDanger},
			lockedScheme{"#0F1A20", "#22202E", "#16181C", "#1E2228", "#A8C9E4", "#A8C9E4", "#A4C2A8", "#B5AA90", darkDanger}},
		{"WoodFire", "Wood–Fire",
			lockedScheme{"#EEF7F2", "#F7CFBA", "#FFFFFF", "#FBF6EE", "#AB372F", "#AB372F", "#1A6840", "#964D22", lightDanger},
			lockedScheme{"#141E1B", "#481E1C", "#1B1817", "#25201E", "#F6AD8F", "#F6AD8F", "#A0D6B4", "#E2C17C", darkDanger}},
		{"Earth", "Earth",
			lockedScheme{"#F9E9CD", "#F6DEAD", "#FFFFFF", "#FAF6ED", "#815F25", "#815F25", lightSuccess, lightWarn, lightDanger},
			lockedScheme{"#1D1B1C", "#503E2A", "#1A1816", "#24201C", "#E2C17C", "#E2C17C", darkSuccess, darkWarn, darkDanger}},
		{"Metal", "Metal",
			lockedScheme{"#F2F3F4", "#E2E1E4", "#FFFFFF", "#F5F5F6", "#3F5E7D", "#4B6C8C", lightSuccess, lightWarn, lightDanger},
			lockedScheme{"#0E100F", "#2B333E", "#16181A", "#1F2226", "#D0E7F9", "#D0E7F9", darkSuccess, darkWarn, darkDanger}},
		{"Wood", "Wood",
			lockedScheme{"#EEF7F2", "#C6E3E1", "#FFFFFF", "#F3F7F6", "#126E82", "#126E82", lightSuccess, lightWarn, lightDanger},
			lockedScheme{"#141E1B", "#134857", "#151B1A", "#1D2625", "#A0D6B4", "#A0D6B4", darkSuccess, darkWarn, darkDanger}},
		{"Water", "Water",
			lockedScheme{"#EDE3E7", "#D8BFD8", "#FFFFFF", "#F5F4F7", "#2E317C", "#2E317C", lightSuccess, lightWarn, lightDanger},
			lockedScheme{"#0E100F", "#302F4B", "#16151C", "#1F1E28", "#B8A5D2", "#B8A5D2", darkSuccess, darkWarn, darkDanger}},
		{"Fire", "Fire",
			lockedScheme{"#F8EBE6", "#F2CAC9", "#FFFFFF", "#FAF4F3", "#9E2F2F", "#9E2F2F", lightSuccess, lightWarn, lightDanger},
			lockedScheme{"#1D1B1C", "#481E1C", "#1B1717", "#251F1F", "#F0ADA0", "#F0ADA0", darkSuccess, darkWarn, darkDanger}},
	}
	if len(portrait.Spaces) != len(want) {
		t.Fatalf("%d Spaces, want the closed set of %d", len(portrait.Spaces), len(want))
	}
	scheme := func(l lockedScheme) portrait.Scheme {
		return portrait.Scheme{
			From: ui.Hex(l.from), To: ui.Hex(l.to), Paper: ui.Hex(l.paper), Code: ui.Hex(l.code),
			Accent: ui.Hex(l.accent), Mark: ui.Hex(l.mark),
			Success: ui.Hex(l.success), Warn: ui.Hex(l.warn), Danger: ui.Hex(l.danger),
		}
	}
	for i, w := range want {
		got := portrait.Spaces[i]
		if got.Name != w.name || got.Label != w.label {
			t.Errorf("Spaces[%d] = %q (%q), want %q (%q)", i, got.Name, got.Label, w.name, w.label)
		}
		if l := scheme(w.light); got.Light != l {
			t.Errorf("%s light = %+v, want %+v", w.name, got.Light, l)
		}
		if d := scheme(w.dark); got.Dark != d {
			t.Errorf("%s dark = %+v, want %+v", w.name, got.Dark, d)
		}
	}
	if portrait.DefaultSpace.Name != "FiveElements" {
		t.Errorf("DefaultSpace = %s, want FiveElements", portrait.DefaultSpace.Name)
	}
}

func TestSharedInksFollowTheAppearance(t *testing.T) {
	eachLook(t, func(t *testing.T, space portrait.Space, dark bool) {
		c := portrait.Colors
		ink, ink2, ink3, on := "#1D1B1C", "#48484D", "#6E6E73", "#FFFFFF"
		if dark {
			ink, ink2, ink3, on = "#F2F3F4", "#CDD1D3", "#959DA2", "#0E100F"
		}
		for _, tc := range []struct {
			name      string
			got, want ui.Color
		}{
			{"Ink", c.Ink, ui.Hex(ink)}, {"Ink2", c.Ink2, ui.Hex(ink2)}, {"Ink3", c.Ink3, ui.Hex(ink3)}, {"OnAccent", c.OnAccent, ui.Hex(on)},
			{"Paper", c.Paper, space.In(dark).Paper}, {"Code", c.Code, space.In(dark).Code},
			{"Accent", c.Accent, space.In(dark).Accent}, {"Note", c.Note, space.In(dark).Accent},
			{"Success", c.Success, space.In(dark).Success}, {"Warn", c.Warn, space.In(dark).Warn}, {"Danger", c.Danger, space.In(dark).Danger},
			{"Tones.Accent", portrait.Tones.Accent.Solid, space.In(dark).Accent},
			{"Focus", c.Focus, space.In(dark).Accent.Alpha(float32(c.Focus.A) / 255)},
		} {
			if !sameColor(tc.got, tc.want) {
				t.Errorf("%s = %+v, want %+v", tc.name, tc.got, tc.want)
			}
		}
		if th := portrait.Theme(); th.Dark != dark || th.Background != c.Paper || th.Accent != c.Accent {
			t.Errorf("Theme dark %v background %+v accent %+v, want the Space's", th.Dark, th.Background, th.Accent)
		}
		if got, d := portrait.ActiveSpace(); got.Name != space.Name || d != dark || portrait.IsDark() != dark {
			t.Errorf("ActiveSpace = %s/%v, want %s/%v", got.Name, d, space.Name, dark)
		}
	})
}

func TestEverySpaceMeetsWCAGAA(t *testing.T) {
	eachLook(t, func(t *testing.T, space portrait.Space, dark bool) {
		c := portrait.Colors
		paper, code := wcagColor(c.Paper), wcagColor(c.Code)
		for _, tc := range []struct {
			name   string
			fg, bg [4]float64
			want   float64
		}{
			{"Ink on Paper", wcagColor(c.Ink), paper, 7},
			{"Ink2 on Paper", wcagColor(c.Ink2), paper, 4.5},
			{"Ink3 on Paper", wcagColor(c.Ink3), paper, 4.5},
			{"Ink3 on Code", wcagColor(c.Ink3), code, 4.5},
			{"Accent on Paper", wcagColor(c.Accent), paper, 4.5},
			{"Accent on Code", wcagColor(c.Accent), code, 4.5},
			{"Accent on Fill", wcagColor(c.Accent), wcagOver(wcagColor(c.Fill), paper), 4.5},
			{"Accent on Scenery", wcagColor(c.Accent), wcagColor(space.In(dark).From.Mix(space.In(dark).To, 0.64)), 4.5},
			{"Accent on Settings selected row", wcagColor(c.Accent), wcagOver(wcagColor(portrait.Materials.Raised.From), paper), 4.5},
			{"Accent on AccentSoft", wcagColor(c.Accent), wcagOver(wcagColor(c.AccentSoft), paper), 4.5},
			{"Ink on Selection", wcagColor(c.Ink), wcagOver(wcagColor(portrait.Theme().Selection), paper), 4.5},
			{"Ink on Diff insert", wcagColor(c.Ink), wcagOver(wcagColor(portrait.Diff.Insert), paper), 4.5},
			{"Ink on Diff delete", wcagColor(c.Ink), wcagOver(wcagColor(portrait.Diff.Delete), paper), 4.5},
			{"Ink on Raised", wcagColor(c.Ink), wcagOver(wcagColor(portrait.Materials.Raised.From), paper), 4.5},
			{"Ink on Pop", wcagColor(c.Ink), wcagOver(wcagColor(portrait.Materials.Pop.From), paper), 4.5},
			{"Ink2 on Glass overlay", wcagColor(c.Ink2), wcagOver(wcagColor(portrait.Materials.GlassOverlay.Tint), wcagColor(space.In(dark).To)), 4.5},
			{"LineStrong on Paper", wcagColor(c.LineStrong), paper, 1.2},
			{"Reader purple on Paper", wcagColor(portrait.Reader.Purple), paper, 4.5},
			{"Reader pink on Paper", wcagColor(portrait.Reader.Pink), paper, 4.5},
			{"Reader success on Paper", wcagColor(portrait.Reader.Success), paper, 4.5},
		} {
			assertWCAGContrast(t, tc.name, wcagContrast(tc.fg, tc.bg), tc.want)
		}
		accentHover, accentPress := portrait.Materials.AccentRaisedHover.From, portrait.Materials.AccentRaisedPress.From
		for name, solid := range map[string]ui.Color{"rest": c.Accent, "hover": accentHover, "press": accentPress} {
			assertWCAGContrast(t, "OnAccent on Accent "+name, wcagContrast(wcagColor(c.OnAccent), wcagColor(solid)), 4.5)
		}
	})
}

func TestElementColoursStayApartInEverySpace(t *testing.T) {
	eachLook(t, func(t *testing.T, _ portrait.Space, _ bool) {
		c := portrait.Colors
		for _, pair := range []struct {
			name string
			a, b ui.Color
		}{
			{"Success/Danger", c.Success, c.Danger},
			{"Success/Warn", c.Success, c.Warn},
		} {
			if d := tokenHueDistance(pair.a, pair.b); d < 30 {
				t.Errorf("%s hue distance %.1f, want at least 30", pair.name, d)
			}
		}
	})
}

func TestAppearanceResolvesAgainstTheSystem(t *testing.T) {
	for _, tc := range []struct {
		name         string
		systemDark   bool
		want         bool
		storedOrZero string
	}{
		{"system light", false, false, "System"},
		{"system dark", true, true, ""},
		{"forced light", true, false, "Light"},
		{"forced dark", false, true, "Dark"},
		{"unknown falls back to system", true, true, "Sepia"},
	} {
		if got := portrait.AppearanceNamed(tc.storedOrZero).IsDark(tc.systemDark); got != tc.want {
			t.Errorf("%s: dark = %v, want %v", tc.name, got, tc.want)
		}
	}
	if len(portrait.Appearances) != 3 {
		t.Errorf("Appearances = %v, want System, Light and Dark", portrait.Appearances)
	}
}

func TestUseSwapsTokensAndReturns(t *testing.T) {
	t.Cleanup(func() { portrait.Use(portrait.DefaultSpace, false) })
	light := portrait.Colors
	fire, _ := portrait.SpaceNamed("Fire")
	portrait.Use(fire, true)
	if portrait.Colors.Accent != fire.Dark.Accent || portrait.Materials.Page.From != fire.Dark.Paper {
		t.Fatalf("Use(Fire, dark) left accent %+v and page %+v", portrait.Colors.Accent, portrait.Materials.Page.From)
	}
	portrait.Use(portrait.DefaultSpace, false)
	if portrait.Colors != light {
		t.Error("returning to Five Elements light did not restore its palette")
	}
}

// TestCalloutTonesStayDistinct checks that Note and Important, the pair the
// reviews found alike, are colors a reader can tell apart in every Space.
func TestCalloutTonesStayDistinct(t *testing.T) {
	distance := func(a, b ui.Color) int {
		diff := func(x, y uint8) int {
			if x < y {
				return int(y - x)
			}
			return int(x - y)
		}
		return diff(a.R, b.R) + diff(a.G, b.G) + diff(a.B, b.B)
	}
	for _, space := range portrait.Spaces {
		for _, dark := range []bool{false, true} {
			portrait.Use(space, dark)
			note, important := portrait.Colors.Note, portrait.Colors.Important
			if d := distance(note, important); d < 48 {
				t.Errorf("%s %v: Note %v and Important %v differ by %d, too close to read apart", space.Name, dark, note, important, d)
			}
		}
	}
	portrait.Use(portrait.DefaultSpace, false)
}
