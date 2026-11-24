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
	sourceWuxing  = "wuxing@adr-0014"
	sourceDerived = "portrait-derived"
)

var tokenProvenance = func() map[string]string {
	provenance := make(map[string]string)
	register := func(source, prefix string, fields ...string) {
		for _, field := range fields {
			provenance[prefix+"."+field] = source
		}
	}
	// The Space owns its accent, page and element colours; the inks are the
	// shared Wu Xing defaults in each appearance.
	register(sourceWuxing, "Colors", "Paper", "Accent", "Ink", "OnAccent", "Success", "Warn", "Danger")
	register(sourceWebUI, "Colors", "Ink2", "Ink3", "Ink4")
	register(sourceDerived, "Colors",
		"Chrome", "Code",
		"Line", "LineStrong",
		"Fill", "FillHover", "FillStrong",
		"AccentSoft", "WarnSoft", "DangerSoft",
		"Note", "NoteSoft", "Tip", "TipSoft", "Important", "ImportantSoft", "Warning", "WarningSoft",
		"Caution", "CautionSoft",
		"Focus", "Scrim", "ModalScrim",
	)
	for _, name := range []string{"Neutral", "Accent", "Success", "Info", "Warning", "Danger"} {
		register(sourceDerived, "Tones."+name, "Solid", "OnSolid", "Ink", "Soft", "Wash", "Border", "Ring")
	}
	register(sourceDerived, "Radii", "XS", "S", "M", "L", "XL", "Pill", "Track", "Segment")
	// Sizes reference coss's three desktop control tiers, tuned to Portrait's 22/28/32 density.
	register(sourceDerived, "Sizes", "Compact", "Regular", "Large")
	register(sourceWebUI, "Type", "Small", "UI", "Control", "Meta", "Code", "H4", "H5", "H6")
	register(sourceDerived, "Type", "H3", "CodeSmall")
	// Display, Title and Reading keep the WebUI sizes and take Arc's tracking rule.
	register(sourceDerived, "Type", "Display", "Title", "Reading", "Heading")
	register(sourceWuxing, "Spaces", "FiveElements", "MetalWater", "WoodFire", "Earth", "Metal", "Wood", "Water", "Fire")
	register(sourceDerived, "DefaultSkeleton", "Period", "Base", "Glint")
	register(sourceDerived, "Frame", "Inset", "InnerRadius", "OuterRadius")
	register(sourceDerived, "Form", "Field", "FieldCompact", "Radius", "PadX", "Gap", "AreaMin", "AreaCompact", "AreaPadY", "LabelGap", "HelpGap", "SelectMin", "SelectMax", "MenuRow", "MenuRadius", "MenuPad", "MenuMin", "MenuMax", "PopoverPad", "PopoverGap", "PopoverHeader", "PopoverWide", "DialogWidth", "DialogPad", "DialogGap", "ListMax", "BubbleRatio", "BubbleMax")
	register(sourceDerived, "Settings", "Width", "Height", "Margin", "Radius", "Rail", "Header", "Column", "ColumnPadX", "ColumnPadTop", "ColumnPadBottom", "GroupGap", "HeadingGap", "NavRow", "NavSectionGap", "Collapse", "Row", "RowCompact", "RowPadX", "RowPadY", "RowGap", "DescriptionMax", "SpacePreview")
	register(sourceDerived, "Editor", "Body", "Code", "Column", "GutterLeft", "GutterRight", "BlockRadius", "BlockPadX", "BlockPadY", "TableCellPadX", "TableCellPadY", "Hairline")
	register(sourceDerived, "IconSizes", "XS", "S", "M", "L")
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
		{"Form", portrait.Form},
		{"Settings", portrait.Settings},
		{"Editor", portrait.Editor},
		{"IconSizes", portrait.IconSizes},
	} {
		collectTokenPaths(root.name, reflect.ValueOf(root.value), actual)
	}

	for _, space := range portrait.Spaces {
		actual["Spaces."+space.Name] = struct{}{}
	}

	allowed := map[string]bool{sourceWebUI: true, sourceCOSS: true, sourceArc: true, sourceWuxing: true, sourceDerived: true}
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
	if got, want := portrait.Colors.ModalScrim, ui.RGBA(29, 27, 28, 0.12); got != want {
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

	if got, want := portrait.Colors.Accent, ui.Hex("#2E5D8C"); got != want {
		t.Errorf("Accent = %+v, want Five Elements' 石青 %+v", got, want)
	}
	if got, want := portrait.Colors.Success, ui.Hex("#1A6840"); got != want {
		t.Errorf("Success = %+v, want 荷叶绿 %+v", got, want)
	}

	accent := expectedTone(portrait.Colors.Accent, portrait.Colors.Accent)
	neutral := expectedTone(portrait.Colors.Ink, portrait.Colors.Ink2)
	neutral.Border = portrait.Colors.Line
	wantTones := portrait.ToneScale{
		Neutral: neutral,
		Accent:  accent,
		Success: expectedTone(portrait.Colors.Success, portrait.Colors.Success),
		Info:    accent,
		Warning: expectedTone(portrait.Colors.Warn, portrait.Colors.Warn),
		Danger:  expectedTone(portrait.Colors.Danger, portrait.Colors.Danger),
	}
	if portrait.Tones != wantTones {
		t.Errorf("Tones = %+v, want %+v", portrait.Tones, wantTones)
	}

	assertTokenSources(t, sourceWuxing, "Colors.Accent", "Colors.Paper", "Colors.Success", "Spaces.FiveElements", "Spaces.MetalWater", "Spaces.WoodFire", "Spaces.Earth")
	assertTokenSources(t, sourceDerived,
		"Colors.ModalScrim", "Colors.Code", "Type.Heading", "Type.Display", "Type.Title", "Type.Reading",
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
