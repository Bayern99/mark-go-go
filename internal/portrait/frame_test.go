package portrait_test

import (
	"bytes"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestCardPaintsExactlyAsSheetWithMediumRadius(t *testing.T) {
	card := ui.NewTester(onChrome(func(c *ui.Context) {
		portrait.Card(c).Size(96, 48)
	}), 180, 120)
	reference := ui.NewTester(onChrome(func(c *ui.Context) {
		e := ui.Column(c).Size(96, 48).Radius(portrait.Radii.M)
		portrait.Materials.Sheet.Apply(e)
	}), 180, 120)
	card.Frame()
	reference.Frame()
	if !bytes.Equal(card.Image().Pix, reference.Image().Pix) {
		t.Fatal("Card does not paint as Materials.Sheet with Radii.M")
	}
}

func TestFramePaintsATranslucentTrayAroundAWhitePanel(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(portrait.Colors.Paper).Padding(20).Children(func() {
			portrait.Frame(c).Size(96, 56).Role(ui.RoleGroup).Label("Preview")
		})
	}, 180, 120)
	tt.Frame()
	frame, ok := tt.AccessByRole(ui.RoleGroup, "Preview")
	if !ok {
		t.Fatal("Frame has no accessible group")
	}
	x := int(frame.Bounds.X + frame.Bounds.W/2)
	trayY := int(frame.Bounds.Y + 1)
	want := portrait.Colors.Fill.Alpha(0.72).Over(portrait.Colors.Paper)
	assertNearColor(t, tt.Image().At(x, trayY), want, "tray")
	assertNearColor(t, tt.Image().At(x, int(frame.Bounds.Y+frame.Bounds.H/2)), portrait.Colors.Paper, "inner panel")
	if got, want := portrait.Radii.L, portrait.Radii.M+4; got != want {
		t.Fatalf("Frame outer radius %v, want inner radius + 4 = %v", got, want)
	}
}

func assertNearColor(t *testing.T, got interface {
	RGBA() (uint32, uint32, uint32, uint32)
}, want ui.Color, name string) {
	t.Helper()
	r, g, b, a := got.RGBA()
	channels := []struct {
		name     string
		got      uint32
		want8Bit uint8
	}{{"red", r, want.R}, {"green", g, want.G}, {"blue", b, want.B}, {"alpha", a, want.A}}
	for _, channel := range channels {
		got8Bit := int(channel.got >> 8)
		difference := got8Bit - int(channel.want8Bit)
		if difference < -1 || difference > 1 {
			t.Errorf("%s %s channel = %d, want %d (±1)", name, channel.name, got8Bit, channel.want8Bit)
		}
	}
}
