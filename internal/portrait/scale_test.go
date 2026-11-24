package portrait_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestScalesAreClosed(t *testing.T) {
	assertClosedScale(t, "Sizes", reflect.ValueOf(portrait.Sizes), []float32{22, 28, 32})
	assertClosedScale(t, "Radii", reflect.ValueOf(portrait.Radii), []float32{4, 6, 8, 12, 16, 999, 10, 8})

	typography := reflect.ValueOf(portrait.Type)
	for i := 0; i < typography.NumField(); i++ {
		style := typography.Field(i).Interface().(portrait.TypeStyle)
		if style.Size < 12 {
			t.Errorf("Type.%s size = %v, want at least 12", typography.Type().Field(i).Name, style.Size)
		}
	}
}

func TestDerivedTokenInvariants(t *testing.T) {
	if portrait.Tones.Info != portrait.Tones.Accent {
		t.Error("Info must have exactly the Accent token values")
	}
	for _, tc := range []struct {
		name string
		tone portrait.Tone
	}{
		{"Neutral", portrait.Tones.Neutral},
		{"Accent", portrait.Tones.Accent},
		{"Success", portrait.Tones.Success},
		{"Info", portrait.Tones.Info},
		{"Warning", portrait.Tones.Warning},
		{"Danger", portrait.Tones.Danger},
	} {
		t.Run(tc.name, func(t *testing.T) {
			solid := tc.tone.Solid
			if got, want := tc.tone.Soft, ui.RGBA(solid.R, solid.G, solid.B, 0.16); got != want {
				t.Errorf("Soft = %+v, want Solid at alpha 0.16 = %+v", got, want)
			}
			if got, want := tc.tone.Wash, ui.RGBA(solid.R, solid.G, solid.B, 0.08); got != want {
				t.Errorf("Wash = %+v, want Solid at alpha 0.08 = %+v", got, want)
			}
		})
	}
	if got, want := portrait.Radii.Segment, portrait.Radii.Track-2; got != want {
		t.Errorf("Segment radius = %v, want Track - 2 = %v", got, want)
	}
	if portrait.FrameInset != 4 {
		t.Errorf("Frame inset = %v, want 4", portrait.FrameInset)
	}
	if got, want := portrait.FrameOuterRadius, portrait.FrameInnerRadius+portrait.FrameInset; got != want {
		t.Errorf("Frame outer radius = %v, want inner radius + inset = %v", got, want)
	}
	if alpha := float32(portrait.Colors.ModalScrim.A) / 255; alpha > 0.16 {
		t.Errorf("ModalScrim alpha = %v, want at most 0.16", alpha)
	}
	if distance := tokenHueDistance(portrait.Tones.Success.Solid, portrait.Tones.Accent.Solid); distance < 60 {
		t.Errorf("Success and Accent hue distance = %.3f degrees, want at least 60", distance)
	}
	if portrait.Shell.PageRadius != portrait.Radii.L || portrait.Shell.Row != portrait.Sizes.Large {
		t.Errorf("Shell page radius %v and row %v, want Radii.L and Sizes.Large", portrait.Shell.PageRadius, portrait.Shell.Row)
	}
	if portrait.DefaultSpace != portrait.Spaces[1] || portrait.SpaceFor("") != portrait.DefaultSpace {
		t.Error("a window without a folder takes the default scenery")
	}
	for _, root := range []string{"/a", "/Users/ada/Field Notes", "C:\\notes"} {
		space := portrait.SpaceFor(root)
		if space != portrait.SpaceFor(root) || space == portrait.Spaces[0] {
			t.Errorf("SpaceFor(%q) = %s, want one stable coloured scenery", root, space.Name)
		}
		if named, ok := portrait.SpaceNamed(space.Name); !ok || named != space {
			t.Errorf("SpaceNamed(%q) does not return the scenery it names", space.Name)
		}
	}
	if _, ok := portrait.SpaceNamed("Nowhere"); ok {
		t.Error("SpaceNamed accepted a name outside the closed set")
	}
}

func assertClosedScale(t *testing.T, name string, scale reflect.Value, values []float32) {
	t.Helper()
	want := make(map[float32]int, len(values))
	for _, value := range values {
		want[value]++
	}
	got := make(map[float32]int)
	for i := 0; i < scale.NumField(); i++ {
		field := scale.Field(i)
		if field.Kind() != reflect.Float32 {
			t.Fatalf("%s.%s has kind %s, want float32", name, scale.Type().Field(i).Name, field.Kind())
		}
		got[float32(field.Float())]++
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s values = %v, want closed set %v", name, got, want)
	}
}

func tokenHueDistance(a, b ui.Color) float64 {
	difference := math.Abs(tokenHue(a) - tokenHue(b))
	return math.Min(difference, 360-difference)
}

func tokenHue(c ui.Color) float64 {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	maximum := math.Max(r, math.Max(g, b))
	minimum := math.Min(r, math.Min(g, b))
	delta := maximum - minimum
	if delta == 0 {
		return 0
	}
	var hue float64
	switch maximum {
	case r:
		hue = math.Mod((g-b)/delta, 6)
	case g:
		hue = (b-r)/delta + 2
	default:
		hue = (r-g)/delta + 4
	}
	if hue < 0 {
		hue += 6
	}
	return hue * 60
}
