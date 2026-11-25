package portrait_test

import (
	"math"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func switchView(on *bool) func(*ui.Context) {
	return onChrome(func(c *ui.Context) {
		portrait.Switch(c, on).Label("Live sync")
	})
}

func thumbCenter(t *testing.T, tt *ui.Tester, bounds ui.Rect) float32 {
	t.Helper()
	y := int(bounds.Y + bounds.H/2)
	lo, hi := float32(math.Inf(1)), float32(math.Inf(-1))
	img := tt.Image()
	for x := int(bounds.X + switchInsetForTest); x < int(bounds.X+bounds.W-switchInsetForTest); x++ {
		px := img.RGBAAt(x, y)
		if px.R > 245 && px.G > 245 && px.B > 245 {
			at := float32(x) + 0.5
			lo, hi = min(lo, at), max(hi, at)
		}
	}
	if math.IsInf(float64(lo), 1) {
		t.Fatal("switch thumb was not found")
	}
	return (lo + hi) / 2
}

const switchInsetForTest = 2

func TestOffSwitchTrackMeetsNonTextContrast(t *testing.T) {
	eachLook(t, func(t *testing.T, _ portrait.Space, _ bool) {
		for _, highContrast := range []bool{false, true} {
			on := false
			tt := ui.NewTesterWith(switchView(&on), 240, 120, ui.TesterOptions{Preferences: ui.Preferences{HighContrast: highContrast}})
			tt.Frame()
			info, _ := tt.AccessByRole(ui.RoleSwitch, "Live sync")
			track := portrait.Colors.Ink3
			if highContrast {
				track = portrait.Colors.Ink2
			}
			px := tt.Image().RGBAAt(int(info.Bounds.X+info.Bounds.W-5), int(info.Bounds.Y+info.Bounds.H/2))
			if px.R != track.R || px.G != track.G || px.B != track.B {
				t.Errorf("high contrast %v: track #%02x%02x%02x, want #%02x%02x%02x", highContrast, px.R, px.G, px.B, track.R, track.G, track.B)
			}
			if ratio := wcagContrast(wcagColor(track), wcagColor(portrait.Colors.Paper)); ratio < 3 {
				t.Errorf("high contrast %v: track contrast %.3f, want >= 3", highContrast, ratio)
			}
		}
	})
}

func TestSwitchTogglesAndAnimatesItsThumb(t *testing.T) {
	on := false
	tt := ui.NewTester(switchView(&on), 240, 120)
	tt.Frame()
	info, ok := tt.AccessByRole(ui.RoleSwitch, "Live sync")
	if !ok || info.Checked {
		t.Fatalf("off switch accessibility = %+v, %v", info, ok)
	}
	off := thumbCenter(t, tt, info.Bounds)
	if err := tt.Click("Live sync"); err != nil {
		t.Fatal(err)
	}
	if !on {
		t.Fatal("click did not turn the switch on")
	}
	if !tt.FramePending() {
		t.Fatal("switch did not schedule its thumb animation")
	}
	tt.Advance(portrait.Motion.Switch / 4)
	mid := thumbCenter(t, tt, info.Bounds)
	tt.Advance(portrait.Motion.Switch + 50*time.Millisecond)
	end := thumbCenter(t, tt, info.Bounds)
	wantEnd := info.Bounds.X + info.Bounds.W - info.Bounds.H/2
	if !(off+3 < mid && mid < wantEnd-1) {
		t.Fatalf("thumb positions off %v, mid %v, end %v; want a smooth transition", off, mid, end)
	}
	if math.Abs(float64(end-wantEnd)) > 1 {
		t.Fatalf("thumb ended at %v, want %v", end, wantEnd)
	}
	info, _ = tt.AccessByRole(ui.RoleSwitch, "Live sync")
	if !info.Checked {
		t.Fatalf("on state was not exposed: %+v", info)
	}
}

func TestSwitchReduceMotionLandsImmediately(t *testing.T) {
	on := false
	tt := ui.NewTesterWith(switchView(&on), 240, 120, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	tt.Frame()
	info, _ := tt.AccessByRole(ui.RoleSwitch, "Live sync")
	if err := tt.Click("Live sync"); err != nil {
		t.Fatal(err)
	}
	if tt.FramePending() {
		t.Fatal("switch scheduled animation under ReduceMotion")
	}
	got := thumbCenter(t, tt, info.Bounds)
	want := info.Bounds.X + info.Bounds.W - info.Bounds.H/2
	if math.Abs(float64(got-want)) > 1 {
		t.Fatalf("thumb at %v on the first on frame, want %v", got, want)
	}
}

func TestSwitchKeyboardDisabledAndInvalidStates(t *testing.T) {
	t.Run("keyboard", func(t *testing.T) {
		on := false
		tt := ui.NewTester(switchView(&on), 240, 120)
		tt.Frame()
		tt.Key(0, ui.KeyTab)
		info, _ := tt.AccessByRole(ui.RoleSwitch, "Live sync")
		if !info.Focused {
			t.Fatal("Tab did not focus the switch")
		}
		tt.Key(0, ui.KeySpace)
		if !on {
			t.Fatal("Space did not turn the switch on")
		}
		if tt.FramePending() {
			t.Fatal("keyboard toggle scheduled a thumb animation")
		}
		got := thumbCenter(t, tt, info.Bounds)
		want := info.Bounds.X + info.Bounds.W - info.Bounds.H/2
		if math.Abs(float64(got-want)) > 1 {
			t.Fatalf("keyboard thumb at %v on the first frame, want %v", got, want)
		}
	})

	t.Run("disabled and invalid", func(t *testing.T) {
		on := false
		tt := ui.NewTester(onChrome(func(c *ui.Context) {
			portrait.Switch(c, &on).Label("Live sync").Disabled(true).Invalid(true)
		}), 240, 120)
		tt.Frame()
		info, _ := tt.AccessByRole(ui.RoleSwitch, "Live sync")
		if !info.Disabled || !info.Invalid {
			t.Fatalf("disabled invalid switch = %+v", info)
		}
		tt.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
		if on {
			t.Fatal("disabled switch changed state")
		}
	})
}
