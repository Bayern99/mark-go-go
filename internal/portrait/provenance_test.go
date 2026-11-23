package portrait_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const (
	sourceWebUI   = "webui@c6504af"
	sourceCOSS    = "coss@b2336c4"
	sourceDerived = "portrait-derived"
)

var tokenProvenance = func() map[string]string {
	provenance := make(map[string]string)
	register := func(source, prefix string, fields ...string) {
		for _, field := range fields {
			provenance[prefix+"."+field] = source
		}
	}
	register(sourceWebUI, "Colors",
		"Chrome", "Paper", "Code",
		"Ink", "Ink2", "Ink3", "Ink4",
		"Line", "LineStrong",
		"Fill", "FillHover", "FillStrong",
		"Accent", "OnAccent", "AccentSoft",
		"Warn", "WarnSoft", "Danger", "DangerSoft",
		"Note", "NoteSoft", "Warning", "WarningSoft",
		"Caution", "CautionSoft",
		"Focus", "Scrim",
	)
	register(sourceDerived, "Colors", "ModalScrim")
	for _, name := range []string{"Neutral", "Accent", "Success", "Info", "Warning", "Danger"} {
		register(sourceDerived, "Tones."+name, "Solid", "OnSolid", "Ink", "Soft", "Wash", "Border", "Ring")
	}
	register(sourceDerived, "Radii", "XS", "S", "M", "L", "XL", "Pill", "Track", "Segment")
	register(sourceCOSS, "Sizes", "Compact", "Regular", "Large")
	register(sourceWebUI, "Type", "Display", "Title", "Reading", "Small", "UI", "Control", "Meta", "Code", "H4", "H5", "H6")
	register(sourceDerived, "Type", "Heading")
	return provenance
}()

func TestEveryTokenDeclaresProvenance(t *testing.T) {
	actual := make(map[string]struct{})
	for _, root := range []struct {
		name  string
		value any
	}{
		{"Colors", portrait.Colors},
		{"Tones", portrait.Tones},
		{"Radii", portrait.Radii},
		{"Sizes", portrait.Sizes},
		{"Type", portrait.Type},
	} {
		collectTokenPaths(root.name, reflect.ValueOf(root.value), actual)
	}

	allowed := map[string]bool{sourceWebUI: true, sourceCOSS: true, sourceDerived: true}
	var failures []string
	for path := range actual {
		source, ok := tokenProvenance[path]
		if !ok {
			failures = append(failures, path+" has no provenance")
		} else if !allowed[source] {
			failures = append(failures, path+" has unknown source "+source)
		}
	}
	for path := range tokenProvenance {
		if _, ok := actual[path]; !ok {
			failures = append(failures, path+" is registered but no longer exists")
		}
	}
	sort.Strings(failures)
	for _, failure := range failures {
		t.Error(failure)
	}
}

func TestTokensMatchTheirSource(t *testing.T) {
	if got, want := portrait.Radii, (portrait.RadiusScale{XS: 4, S: 6, M: 8, L: 12, XL: 14, Pill: 999, Track: 9, Segment: 7}); got != want {
		t.Errorf("Radii = %+v, want %+v", got, want)
	}
	if got, want := portrait.Sizes, (portrait.SizeScale{Compact: 22, Regular: 28, Large: 32}); got != want {
		t.Errorf("Sizes = %+v, want %+v", got, want)
	}
	if got, want := portrait.Colors.ModalScrim, ui.RGBA(29, 29, 31, 0.12); got != want {
		t.Errorf("ModalScrim = %+v, want %+v", got, want)
	}
	if got, want := portrait.Type.Heading, portrait.Type.Reading.In(portrait.FontDisplay); got != want {
		t.Errorf("Heading = %+v, want Reading.In(FontDisplay) = %+v", got, want)
	}

	accent := expectedTone(portrait.Colors.Accent, portrait.Colors.Accent)
	wantTones := portrait.ToneScale{
		Neutral: expectedTone(portrait.Colors.Ink, portrait.Colors.Ink2),
		Accent:  accent,
		Success: accent,
		Info:    expectedTone(ui.Hex("#2b6cb0"), ui.Hex("#1e4e8c")),
		Warning: expectedTone(portrait.Colors.Warn, portrait.Colors.Warn),
		Danger:  expectedTone(portrait.Colors.Danger, portrait.Colors.Danger),
	}
	if portrait.Tones != wantTones {
		t.Errorf("Tones = %+v, want %+v", portrait.Tones, wantTones)
	}

	assertTokenSources(t, sourceCOSS, "Sizes.Compact", "Sizes.Regular", "Sizes.Large")
	assertTokenSources(t, sourceDerived,
		"Colors.ModalScrim", "Type.Heading",
		"Radii.XS", "Radii.S", "Radii.M", "Radii.L", "Radii.XL", "Radii.Pill", "Radii.Track", "Radii.Segment",
	)
}

func expectedTone(solid, ink ui.Color) portrait.Tone {
	alpha := func(c ui.Color, a float32) ui.Color { return ui.RGBA(c.R, c.G, c.B, a) }
	return portrait.Tone{
		Solid:   solid,
		OnSolid: portrait.Colors.OnAccent,
		Ink:     ink,
		Soft:    alpha(solid, 0.16),
		Wash:    alpha(solid, 0.08),
		Border:  alpha(ink, 0.72),
		Ring:    alpha(ink, 0.85),
	}
}

func assertTokenSources(t *testing.T, want string, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if got := tokenProvenance[path]; got != want {
			t.Errorf("%s source = %q, want %q", path, got, want)
		}
	}
}

func collectTokenPaths(path string, value reflect.Value, paths map[string]struct{}) {
	if value.Kind() == reflect.Struct && value.Type() != reflect.TypeOf(ui.Color{}) && value.Type() != reflect.TypeOf(portrait.TypeStyle{}) {
		typ := value.Type()
		for i := 0; i < value.NumField(); i++ {
			if typ.Field(i).PkgPath == "" {
				collectTokenPaths(path+"."+typ.Field(i).Name, value.Field(i), paths)
			}
		}
		return
	}
	paths[path] = struct{}{}
}
