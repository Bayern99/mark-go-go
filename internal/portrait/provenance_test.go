package portrait_test

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const (
	sourceWebUI   = "webui@c6504af"
	sourceCOSS    = "coss@b2336c4"
	sourceArc     = "arc@adr-0013"
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
		"OnAccent",
		"Warn", "WarnSoft", "Danger", "DangerSoft",
		"Warning", "WarningSoft",
		"Caution", "CautionSoft",
		"Scrim",
	)
	// The accent is Arc's brand blue; what follows it is derived by fixed alpha.
	register(sourceArc, "Colors", "Accent")
	register(sourceDerived, "Colors", "AccentSoft", "Note", "NoteSoft", "Focus", "Success", "ModalScrim")
	for _, name := range []string{"Neutral", "Accent", "Success", "Info", "Warning", "Danger"} {
		register(sourceDerived, "Tones."+name, "Solid", "OnSolid", "Ink", "Soft", "Wash", "Border", "Ring")
	}
	register(sourceDerived, "Radii", "XS", "S", "M", "L", "XL", "Pill", "Track", "Segment")
	// Sizes reference coss's three desktop control tiers, tuned to Portrait's 22/28/32 density.
	register(sourceDerived, "Sizes", "Compact", "Regular", "Large")
	register(sourceWebUI, "Type", "Small", "UI", "Control", "Meta", "Code", "H4", "H5", "H6")
	// Display, Title and Reading keep the WebUI sizes and take Arc's tracking rule.
	register(sourceDerived, "Type", "Display", "Title", "Reading", "Heading")
	register(sourceArc, "Spaces", "Mist", "Sky", "Dawn", "Twilight", "Aurora", "Sand")
	register(sourceDerived, "DefaultSkeleton", "Period", "Base", "Glint")
	register(sourceDerived, "Frame", "Inset", "InnerRadius", "OuterRadius")
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
		{"DefaultSkeleton", portrait.DefaultSkeleton},
		{"Frame", struct {
			Inset, InnerRadius, OuterRadius float32
		}{portrait.FrameInset, portrait.FrameInnerRadius, portrait.FrameOuterRadius}},
	} {
		collectTokenPaths(root.name, reflect.ValueOf(root.value), actual)
	}

	for _, space := range portrait.Spaces {
		actual["Spaces."+space.Name] = struct{}{}
	}

	allowed := map[string]bool{sourceWebUI: true, sourceCOSS: true, sourceArc: true, sourceDerived: true}
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
	if got, want := portrait.Radii, (portrait.RadiusScale{XS: 4, S: 6, M: 8, L: 12, XL: 16, Pill: 999, Track: 10, Segment: 8}); got != want {
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
	if got, want := portrait.DefaultSkeleton, (portrait.Skeleton{
		Period: 1300 * time.Millisecond,
		Base:   portrait.Colors.Fill,
		Glint:  ui.RGBA(253, 253, 253, 0.828),
	}); got != want {
		t.Errorf("DefaultSkeleton = %+v, want %+v", got, want)
	}
	if portrait.FrameInset != 4 || portrait.FrameInnerRadius != portrait.Radii.M || portrait.FrameOuterRadius != portrait.Radii.L {
		t.Errorf("Frame tokens = (%v, %v, %v), want (4, %v, %v)", portrait.FrameInset, portrait.FrameInnerRadius, portrait.FrameOuterRadius, portrait.Radii.M, portrait.Radii.L)
	}

	if got, want := portrait.Colors.Accent, ui.Hex("#3139fb"); got != want {
		t.Errorf("Accent = %+v, want Arc's brand blue %+v", got, want)
	}
	if got, want := portrait.Colors.Success, ui.Hex("#197a49"); got != want {
		t.Errorf("Success = %+v, want %+v", got, want)
	}
	if got, want := portrait.Spaces, [...]portrait.Space{
		{Name: "Mist", From: ui.Hex("#eeeef2"), To: ui.Hex("#e2e2e8"), Mark: ui.Hex("#8e8e98")},
		{Name: "Sky", From: ui.Hex("#cfdcff"), To: ui.Hex("#ddd3ff"), Mark: ui.Hex("#5b7cfa")},
		{Name: "Dawn", From: ui.Hex("#ffd9c7"), To: ui.Hex("#ffc9d6"), Mark: ui.Hex("#ff7e5f")},
		{Name: "Twilight", From: ui.Hex("#ddd4ff"), To: ui.Hex("#f6cfe6"), Mark: ui.Hex("#9a6cf0")},
		{Name: "Aurora", From: ui.Hex("#c8efdc"), To: ui.Hex("#c5e2fa"), Mark: ui.Hex("#22b08a")},
		{Name: "Sand", From: ui.Hex("#fff6d6"), To: ui.Hex("#f3e6c4"), Mark: ui.Hex("#d9a441")},
	}; got != want {
		t.Errorf("Spaces = %+v, want %+v", got, want)
	}

	accent := expectedTone(portrait.Colors.Accent, portrait.Colors.Accent)
	wantTones := portrait.ToneScale{
		Neutral: expectedTone(portrait.Colors.Ink, portrait.Colors.Ink2),
		Accent:  accent,
		Success: expectedTone(portrait.Colors.Success, portrait.Colors.Success),
		Info:    accent,
		Warning: expectedTone(portrait.Colors.Warn, portrait.Colors.Warn),
		Danger:  expectedTone(portrait.Colors.Danger, portrait.Colors.Danger),
	}
	if portrait.Tones != wantTones {
		t.Errorf("Tones = %+v, want %+v", portrait.Tones, wantTones)
	}

	assertTokenSources(t, sourceArc, "Colors.Accent", "Spaces.Sky", "Spaces.Dawn", "Spaces.Twilight", "Spaces.Aurora")
	assertTokenSources(t, sourceDerived,
		"Colors.ModalScrim", "Colors.Success", "Type.Heading", "Type.Display", "Type.Title", "Type.Reading",
		"Sizes.Compact", "Sizes.Regular", "Sizes.Large",
		"DefaultSkeleton.Period", "DefaultSkeleton.Base", "DefaultSkeleton.Glint",
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
