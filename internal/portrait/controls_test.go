package portrait_test

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func onChrome(build func(c *ui.Context)) func(c *ui.Context) {
	return func(c *ui.Context) {
		ui.Column(c).Fill().Background(portrait.Colors.Chrome).Padding(40).AlignItems(ui.Start).Children(func() { build(c) })
	}
}

func region(img *image.RGBA, r ui.Rect) []uint32 {
	var px []uint32
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			px = append(px, cr+cg+cb)
		}
	}
	return px
}

func differs(a, b []uint32) int {
	n := 0
	for i := range a {
		if a[i] != b[i] {
			n++
		}
	}
	return n
}

func TestRaisedButtonsAreLitFromAboveAndSinkWhilePressed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(c *ui.Context) ui.Element
	}{
		{"raised", func(c *ui.Context) ui.Element { return portrait.Button(c, "Checkpoints", "Checkpoints") }},
		{"accent", func(c *ui.Context) ui.Element { return portrait.AccentButton(c, "Open Folder", "Open Folder") }},
		{"icon", func(c *ui.Context) ui.Element { return portrait.IconButton(c, portrait.IconPlugs, "Plugins") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tt := ui.NewTester(onChrome(func(c *ui.Context) { tc.build(c) }), 240, 160)
			tt.Frame()
			info, ok := tt.AccessByRole(ui.RoleButton, map[string]string{"raised": "Checkpoints", "accent": "Open Folder", "icon": "Plugins"}[tc.name])
			if !ok {
				t.Fatal("button missing from the accessibility tree")
			}
			b := info.Bounds
			if b.H != 28 {
				t.Fatalf("height %v, want 28", b.H)
			}
			x := int(b.X + 3)
			if top, bottom := luma(tt, x, int(b.Y)+1), luma(tt, x, int(b.Y+b.H)-2); top <= bottom {
				t.Fatalf("top %d not lighter than bottom %d", top, bottom)
			}
			cx := int(b.X + b.W/2)
			if below, chrome := luma(tt, cx, int(b.Y+b.H)+1), luma(tt, cx, int(b.Y+b.H)+20); below >= chrome {
				t.Fatalf("no shadow under the button: %d vs chrome %d", below, chrome)
			}
			rest := region(tt.Image(), b)
			tt.Press(b.X+b.W/2, b.Y+b.H/2)
			tt.Advance(300 * time.Millisecond)
			pressed := region(tt.Image(), b)
			if n := differs(rest, pressed); n < len(rest)/4 {
				t.Fatalf("pressed look changes %d of %d pixels", n, len(rest))
			}
			tt.Release(b.X+b.W/2, b.Y+b.H/2)
			tt.Move(0, 0)
			tt.Advance(300 * time.Millisecond)
			if n := differs(rest, region(tt.Image(), b)); n != 0 {
				t.Fatalf("released button did not return to rest: %d pixels differ", n)
			}
		})
	}
}

func TestPressedLookEasesInOverThePressDuration(t *testing.T) {
	tt := ui.NewTester(onChrome(func(c *ui.Context) { portrait.Button(c, "Checkpoints", "Checkpoints") }), 240, 160)
	tt.Frame()
	info, _ := tt.AccessByRole(ui.RoleButton, "Checkpoints")
	b := info.Bounds
	probe := func() uint32 { return luma(tt, int(b.X+3), int(b.Y+b.H/2)) }
	rest := probe()
	tt.Press(b.X+b.W/2, b.Y+b.H/2)
	tt.Advance(30 * time.Millisecond)
	mid := probe()
	tt.Advance(300 * time.Millisecond)
	end := probe()
	if !(end < mid && mid < rest) {
		t.Fatalf("press fill rest %d, mid %d, end %d: want a gradual darkening", rest, mid, end)
	}
}

func TestHoverDeepensTheRaisedShadow(t *testing.T) {
	tt := ui.NewTester(onChrome(func(c *ui.Context) { portrait.Button(c, "Checkpoints", "Checkpoints") }), 240, 160)
	tt.Frame()
	info, _ := tt.AccessByRole(ui.RoleButton, "Checkpoints")
	b := info.Bounds
	under := func() uint32 { return luma(tt, int(b.X+b.W/2), int(b.Y+b.H)+3) }
	rest := under()
	tt.Move(b.X+b.W/2, b.Y+b.H/2)
	tt.Advance(300 * time.Millisecond)
	if hover := under(); hover >= rest {
		t.Fatalf("shadow under hovered button %d not darker than at rest %d", hover, rest)
	}
}

func TestIconButtonsKeepTheirNameAndTooltip(t *testing.T) {
	tt := ui.NewTester(onChrome(func(c *ui.Context) {
		ui.Row(c).Gap(8).Children(func() {
			portrait.IconButton(c, portrait.IconSidebar.Mirrored(), "Toggle research")
			portrait.QuietIconButton(c, portrait.IconX, "Close Paper.md")
		})
	}), 240, 160)
	tt.Frame()
	for _, name := range []string{"Toggle research", "Close Paper.md"} {
		info, ok := tt.AccessByRole(ui.RoleButton, name)
		if !ok || info.Description != name {
			t.Fatalf("%s: %+v (found %v), want its name as tooltip", name, info, ok)
		}
	}
	for _, a := range tt.AccessAll() {
		for _, s := range []string{a.Name, a.Value, a.Description} {
			if strings.Contains(s, portrait.IconX.Glyph) || strings.Contains(s, portrait.IconSidebar.Glyph) {
				t.Fatalf("icon glyph exposed to assistive technology: %+v", a)
			}
		}
	}
}

func TestQuietIconButtonIsFlatUntilHovered(t *testing.T) {
	tt := ui.NewTester(onChrome(func(c *ui.Context) { portrait.QuietIconButton(c, portrait.IconX, "Close") }), 240, 160)
	tt.Frame()
	info, _ := tt.AccessByRole(ui.RoleButton, "Close")
	b := info.Bounds
	chrome := luma(tt, 10, 10)
	if corner := luma(tt, int(b.X+2), int(b.Y+2)); corner != chrome {
		t.Fatalf("quiet button paints a fill at rest: %d vs chrome %d", corner, chrome)
	}
	tt.Move(b.X+b.W/2, b.Y+b.H/2)
	tt.Advance(300 * time.Millisecond)
	if corner := luma(tt, int(b.X+3), int(b.Y+3)); corner == chrome {
		t.Fatal("hovered quiet button stays flat")
	}
}

func rowHoverTester(prefs ui.Preferences) (*ui.Tester, ui.Rect) {
	tt := ui.NewTesterWith(func(c *ui.Context) {
		ui.Column(c).Fill().Background(portrait.Colors.Paper).Padding(40).Children(func() {
			row := ui.ButtonBase(c.Key("row")).Label("Notes.md").Width(160).Height(portrait.Sizes.Regular).Radius(portrait.Radii.M)
			portrait.RowHover(row, row.Hovered())
		})
	}, 240, 120, ui.TesterOptions{Preferences: prefs})
	tt.Frame()
	info, _ := tt.AccessByRole(ui.RoleButton, "Notes.md")
	return tt, info.Bounds
}

func TestRowHoverEasesOntoTheRowMaterial(t *testing.T) {
	tt, b := rowHoverTester(ui.Preferences{})
	paper := luma(tt, 10, 10)
	if mid := luma(tt, int(b.X+b.W/2), int(b.Y+1)); mid != paper {
		t.Fatalf("row paints a fill at rest: %d vs paper %d", mid, paper)
	}
	tt.Move(b.X+b.W/2, b.Y+b.H/2)
	first := region(tt.Image(), b)
	tt.Advance(hoverStep)
	mid := region(tt.Image(), b)
	tt.Advance(time.Second)
	settled := region(tt.Image(), b)
	if differs(first, mid) == 0 || differs(mid, settled) == 0 {
		t.Fatal("row hover did not pass through an intermediate frame")
	}
	if edge := luma(tt, int(b.X+b.W/2), int(b.Y+b.H)); edge == paper {
		t.Fatal("hovered row has no hairline lift below it")
	}
}

func TestRowHoverLandsAtOnceUnderReducedMotion(t *testing.T) {
	tt, b := rowHoverTester(ui.Preferences{ReduceMotion: true})
	tt.Move(b.X+b.W/2, b.Y+b.H/2)
	first := region(tt.Image(), b)
	tt.Advance(time.Second)
	if n := differs(first, region(tt.Image(), b)); n != 0 {
		t.Fatalf("row hover still animating under ReduceMotion: %d pixels changed", n)
	}
}

const hoverStep = 40 * time.Millisecond

type segments struct{ selected int }

func (s *segments) view(c *ui.Context) {
	ui.Column(c).Fill().Background(ui.Hex("#000000")).Padding(40).AlignItems(ui.Start).Children(func() {
		portrait.Track(c, "views", func(t *portrait.Segments) {
			for i, title := range []string{"Preview", "Markdown", "GenUI"} {
				b := t.Segment(c, title, title, s.selected == i).Width(90)
				if b.Clicked() {
					s.selected = i
				}
				if b.Focused() {
					switch {
					case b.Shortcut(0, ui.KeyRight):
						s.selected = (i + 1) % 3
					case b.Shortcut(0, ui.KeyLeft):
						s.selected = (i + 2) % 3
					}
				}
			}
		}).Label("Document views")
	})
}

func capsuleLeft(t *testing.T, tt *ui.Tester) float32 {
	t.Helper()
	track, ok := tt.Access("Document views")
	if !ok {
		t.Fatal("track missing")
	}
	img := tt.Image()
	y := int(track.Bounds.Y) + 4
	for x := int(track.Bounds.X); x < int(track.Bounds.X+track.Bounds.W); x++ {
		r, g, b, _ := img.At(x, y).RGBA()
		if r+g+b > 3*0xf800 {
			return float32(x)
		}
	}
	t.Fatal("no capsule in the track")
	return 0
}

func TestCapsuleSlidesToTheNewSegment(t *testing.T) {
	s := &segments{}
	tt := ui.NewTester(s.view, 480, 160)
	tt.Frame()
	from := capsuleLeft(t, tt)
	if err := tt.Click("GenUI"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(portrait.Motion.Switch / 3)
	mid := capsuleLeft(t, tt)
	tt.Advance(time.Second)
	to := capsuleLeft(t, tt)
	if !(from < mid && mid < to) {
		t.Fatalf("capsule at %v → %v → %v, want it between while moving", from, mid, to)
	}
}

func TestCapsuleLandsAtOnceForKeyboardOrReducedMotion(t *testing.T) {
	target := func() float32 {
		s := &segments{selected: 1}
		tt := ui.NewTester(s.view, 480, 160)
		tt.Frame()
		return capsuleLeft(t, tt)
	}()

	t.Run("reduced motion", func(t *testing.T) {
		s := &segments{}
		tt := ui.NewTesterWith(s.view, 480, 160, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
		tt.Frame()
		if err := tt.Click("Markdown"); err != nil {
			t.Fatal(err)
		}
		if got := capsuleLeft(t, tt); got != target {
			t.Fatalf("capsule at %v on frame 0, want %v", got, target)
		}
	})
	t.Run("keyboard", func(t *testing.T) {
		s := &segments{}
		tt := ui.NewTester(s.view, 480, 160)
		tt.Frame()
		tt.Key(0, ui.KeyTab)
		if !tt.Focused("Preview") {
			t.Fatal("Tab did not focus the first segment")
		}
		tt.Key(0, ui.KeyRight)
		if s.selected != 1 {
			t.Fatalf("selected %d after Right", s.selected)
		}
		if got := capsuleLeft(t, tt); got != target {
			t.Fatalf("capsule at %v on frame 0, want %v", got, target)
		}
	})
	t.Run("pointer then backward keyboard", func(t *testing.T) {
		s := &segments{}
		tt := ui.NewTester(s.view, 480, 160)
		tt.Frame()
		if err := tt.Click("GenUI"); err != nil {
			t.Fatal(err)
		}
		tt.Advance(time.Second)
		if !tt.Focused("GenUI") {
			t.Fatal("pointer click did not focus GenUI")
		}
		tt.Key(0, ui.KeyLeft)
		if s.selected != 1 {
			t.Fatalf("selected %d after Left", s.selected)
		}
		if !tt.Focused("GenUI") {
			t.Fatal("Left unexpectedly moved focus instead of only the selection")
		}
		if got := capsuleLeft(t, tt); got != target {
			t.Fatalf("capsule at %v on frame 0, want %v", got, target)
		}
	})
}

func TestTrackIsSunken(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(portrait.Colors.Paper).Padding(40).AlignItems(ui.Start).Children(func() {
			portrait.Track(c, "", func(t *portrait.Segments) {
				t.Segment(c, "One", "One", true).Width(60)
				t.Segment(c, "Two", "Two", false).Width(60)
			}).Label("Views")
		})
	}, 240, 160)
	tt.Frame()
	track, _ := tt.Access("Views")
	b := track.Bounds
	x := int(b.X + b.W - 20)
	if edge, inside := luma(tt, x, int(b.Y)), luma(tt, x, int(b.Y+b.H/2)); edge >= inside {
		t.Fatalf("track top edge %d not darker than its middle %d", edge, inside)
	}
}

func TestPopoverPanelFloatsOnARaisedShadow(t *testing.T) {
	open := true
	tt := ui.NewTester(onChrome(func(c *ui.Context) {
		anchor := portrait.Button(c, "More", "More documents")
		portrait.Popover(c, anchor, &open, func() { ui.Text(c, "Hidden.md").Padding(8) })
	}), 320, 240)
	tt.Frame()
	pop, ok := tt.AccessByRole(ui.RolePopup, "")
	if !ok {
		for _, a := range tt.AccessAll() {
			if a.Role == ui.RolePopup {
				pop, ok = a, true
			}
		}
	}
	if !ok {
		t.Fatal("popover panel missing")
	}
	b := pop.Bounds
	cx := int(b.X + b.W/2)
	if inside, chrome := luma(tt, cx, int(b.Y+b.H/2)-12), luma(tt, 10, 230); inside <= chrome {
		t.Fatalf("panel fill %d not lighter than the chrome %d behind its overlay glass", inside, chrome)
	}
	if below, chrome := luma(tt, cx, int(b.Y+b.H)+4), luma(tt, 10, 230); below >= chrome {
		t.Fatalf("no shadow under the panel: %d vs %d", below, chrome)
	}
}
