package nativeeditor_test

import (
	"image/color"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const navigationFoldSource = "# Heading\n\nBody text.\n"

func navigationFoldPreview(preferences ui.Preferences, autoFocus bool) (*nativeeditor.Document, *ui.Tester) {
	return navigationFoldPreviewMode(preferences, autoFocus, false)
}

func navigationFoldPreviewMode(preferences ui.Preferences, autoFocus, readOnly bool) (*nativeeditor.Document, *ui.Tester) {
	d := nativeeditor.New(navigationFoldSource)
	body := strings.Index(navigationFoldSource, "Body")
	if err := d.SelectUTF16(body, body); err != nil {
		panic(err)
	}
	tt := ui.NewTesterWith(func(c *ui.Context) {
		view := d.View(c, readOnly).Fill()
		if autoFocus {
			view.AutoFocus()
		}
	}, 740, 420, ui.TesterOptions{Preferences: preferences})
	tt.Frame()
	return d, tt
}

func navigationFoldMarkVisible(t *testing.T, tt *ui.Tester, label string) bool {
	t.Helper()
	button, ok := tt.Access(label)
	if !ok {
		t.Fatalf("missing fold control %q", label)
	}
	ink := color.RGBA{R: portrait.Colors.Ink.R, G: portrait.Colors.Ink.G, B: portrait.Colors.Ink.B, A: 255}
	ink3 := color.RGBA{R: portrait.Colors.Ink3.R, G: portrait.Colors.Ink3.G, B: portrait.Colors.Ink3.B, A: 255}
	cx, cy := button.Bounds.X+button.Bounds.W/2, button.Bounds.Y+button.Bounds.H/2
	for y := int(cy - 6); y <= int(cy+6); y++ {
		for x := int(cx - 6); x <= int(cx+6); x++ {
			pixel := tt.Image().RGBAAt(x, y)
			if pixel == ink || pixel == ink3 {
				return true
			}
		}
	}
	return false
}

func navigationFoldButtonCenter(t *testing.T, tt *ui.Tester, label string) (float32, float32) {
	t.Helper()
	button, ok := tt.Access(label)
	if !ok {
		t.Fatalf("missing fold control %q", label)
	}
	return button.Bounds.X + button.Bounds.W/2, button.Bounds.Y + button.Bounds.H/2
}

func TestFoldMarkRestingVisibilityPreservesGeometryAndHitTarget(t *testing.T) {
	d, tt := navigationFoldPreview(ui.Preferences{}, false)
	headingBefore, ok := d.RectUTF16(2)
	if !ok {
		t.Fatal("missing heading rectangle")
	}
	buttonBefore, ok := tt.Access("Fold Heading")
	if !ok {
		t.Fatal("missing expanded fold control")
	}
	if navigationFoldMarkVisible(t, tt, "Fold Heading") {
		t.Fatal("expanded fold mark is visible while the caret is outside its heading")
	}

	x, y := navigationFoldButtonCenter(t, tt, "Fold Heading")
	tt.Move(x, y)
	tt.Frame()
	if !navigationFoldMarkVisible(t, tt, "Fold Heading") {
		t.Fatal("hovering the fold hit target did not reveal its mark")
	}
	headingHovered, _ := d.RectUTF16(2)
	buttonHovered, _ := tt.Access("Fold Heading")
	if headingHovered != headingBefore || buttonHovered.Bounds != buttonBefore.Bounds {
		t.Fatalf("revealing the mark moved the heading or hit target: heading %v -> %v, button %v -> %v", headingBefore, headingHovered, buttonBefore.Bounds, buttonHovered.Bounds)
	}

	tt.Press(x, y)
	if !navigationFoldMarkVisible(t, tt, "Fold Heading") {
		t.Fatal("pressing the fold hit target did not reveal its mark")
	}
	tt.Release(x, y)
	tt.Move(0, 0)
	tt.Frame()
	if !navigationFoldMarkVisible(t, tt, "Expand Heading") {
		t.Fatal("collapsed fold mark is not permanently visible")
	}
	headingFolded, _ := d.RectUTF16(2)
	buttonFolded, ok := tt.Access("Expand Heading")
	if !ok {
		t.Fatal("clicking the hidden-at-rest control did not fold the section")
	}
	if headingFolded != headingBefore || buttonFolded.Bounds != buttonBefore.Bounds {
		t.Fatalf("folding moved the heading or hit target: heading %v -> %v, button %v -> %v", headingBefore, headingFolded, buttonBefore.Bounds, buttonFolded.Bounds)
	}
	if err := tt.Click("Expand Heading"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.Find("Fold Heading"); !ok {
		t.Fatal("clicking the collapsed control did not expand the section")
	}
}

func TestFoldMarkAppearsForKeyboardFocusAndActivates(t *testing.T) {
	_, tt := navigationFoldPreviewMode(ui.Preferences{}, false, true)
	if navigationFoldMarkVisible(t, tt, "Fold Heading") {
		t.Fatal("expanded fold mark is visible before keyboard focus")
	}
	focused := false
	for range 8 {
		button, _ := tt.Access("Fold Heading")
		focused = button.Focused
		if focused {
			break
		}
		tt.Key(0, ui.KeyTab)
	}
	if !focused {
		t.Fatal("Tab never focused the fold control")
	}
	if !navigationFoldMarkVisible(t, tt, "Fold Heading") {
		button, _ := tt.Access("Fold Heading")
		t.Fatalf("keyboard-focused fold control did not reveal its mark: %+v", button)
	}
	tt.Key(0, ui.KeySpace)
	if _, ok := tt.Find("Expand Heading"); !ok {
		t.Fatal("Space did not fold the keyboard-focused section")
	}
	if !navigationFoldMarkVisible(t, tt, "Expand Heading") {
		t.Fatal("keyboard-folded section has no visible mark")
	}
	tt.Key(0, ui.KeySpace)
	if _, ok := tt.Find("Fold Heading"); !ok {
		t.Fatal("Space did not expand the keyboard-focused section")
	}
}

func TestFoldMarkAppearsWithoutHoverCapability(t *testing.T) {
	_, tt := navigationFoldPreview(ui.Preferences{NoHover: true}, false)
	if !navigationFoldMarkVisible(t, tt, "Fold Heading") {
		t.Fatal("expanded fold mark is hidden on a device without hover")
	}
}

func TestFoldMarkFollowsKeyboardCaretAndSelectionOnHeading(t *testing.T) {
	t.Run("keyboard caret", func(t *testing.T) {
		_, tt := navigationFoldPreview(ui.Preferences{}, true)
		if navigationFoldMarkVisible(t, tt, "Fold Heading") {
			t.Fatal("expanded fold mark is visible while the caret is in the body")
		}
		tt.Key(0, ui.KeyUp)
		tt.Frame()
		if navigationFoldMarkVisible(t, tt, "Fold Heading") {
			t.Fatal("expanded fold mark is visible while the caret is on the intervening blank line")
		}
		tt.Key(0, ui.KeyUp)
		tt.Frame()
		if !navigationFoldMarkVisible(t, tt, "Fold Heading") {
			t.Fatal("moving the keyboard caret onto the heading did not reveal its mark")
		}
		tt.Key(ui.Cmd|ui.Alt, ui.KeyLeft)
		tt.Frame()
		if _, ok := tt.Find("Expand Heading"); !ok {
			t.Fatal("fold shortcut did not collapse the section")
		}
		tt.Key(ui.Cmd|ui.Alt, ui.KeyRight)
		tt.Frame()
		if _, ok := tt.Find("Fold Heading"); !ok {
			t.Fatal("expand shortcut did not reopen the section")
		}
	})

	t.Run("selection", func(t *testing.T) {
		d, tt := navigationFoldPreview(ui.Preferences{}, false)
		if err := d.SelectUTF16(2, 5); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if !navigationFoldMarkVisible(t, tt, "Fold Heading") {
			t.Fatal("selecting heading text did not reveal its expanded fold mark")
		}
	})
}
