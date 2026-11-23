package portraitgallery

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestMain(m *testing.M) {
	if err := portrait.RegisterFonts(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestGallery(t *testing.T) {
	for _, required := range []Component{Button, AccentButton, QuietButton, DangerButton, DangerOutlineButton, LinkButton, IconButton, Group, Track, InputFrame, Tooltip, Spinner, Card, Frame, Skeleton} {
		if _, ok := Lookup(required); !ok {
			t.Fatalf("required component %s is not registered", required)
		}
	}

	dir := os.Getenv("PORTRAIT_GALLERY_DIR")
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	sessions := make(map[Preference]*cellSession, len(Preferences))
	for _, pref := range Preferences {
		sessions[pref] = newCellSession(pref)
	}
	for _, component := range Components() {
		t.Run(string(component.Name), func(t *testing.T) {
			surfaces := Surfaces
			if len(component.AllowedSurfaces) > 0 {
				surfaces = component.AllowedSurfaces
			}
			var overview *image.RGBA
			row := 0
			needImage := dir != ""
			if needImage {
				overview = image.NewRGBA(image.Rect(0, 0, cellWidth*len(States), (headerHeight+cellHeight)*len(Preferences)*len(surfaces)))
			}
			for _, preference := range Preferences {
				for _, surface := range surfaces {
					if !needImage && (preference != Default || surface != surfaces[0]) {
						rendered, err := sessions[preference].render(Cell{Component: component.Name, State: Rest, Preference: preference, Surface: surface}, false)
						if err != nil {
							t.Fatal(err)
						}
						assertCellStructure(t, rendered)
						continue
					}
					sheet, err := renderSheetWith(sessions[preference], component.Name, preference, surface, needImage, func(rendered cellRender) {
						assertCellStructure(t, rendered)
					})
					if err != nil {
						t.Fatal(err)
					}
					if overview != nil {
						at := image.Rect(0, row*(headerHeight+cellHeight), sheet.Bounds().Dx(), (row+1)*(headerHeight+cellHeight))
						draw.Draw(overview, at, sheet, image.Point{}, draw.Src)
					}
					row++
				}
			}
			if overview != nil {
				path := filepath.Join(dir, strings.ToLower(string(component.Name))+".png")
				file, err := os.Create(path)
				if err != nil {
					t.Fatal(err)
				}
				encodeErr := png.Encode(file, overview)
				closeErr := file.Close()
				if encodeErr != nil {
					t.Fatal(encodeErr)
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
				t.Logf("exported %s", path)
			}
		})
	}
}

func assertCellStructure(t *testing.T, rendered cellRender) {
	t.Helper()
	tt, cell := rendered.tester, rendered.cell
	root, ok := tt.Access(cellName(cell))
	if !ok {
		t.Fatalf("%s: cell is missing from the accessibility tree", cellName(cell))
	}
	if root.Bounds.W <= 0 || root.Bounds.H <= 0 {
		t.Fatalf("%s: zero cell bounds %+v", cellName(cell), root.Bounds)
	}
	if root.Bounds.W != cellWidth || root.Bounds.H != cellHeight {
		t.Fatalf("%s: cell bounds %+v, want %dx%d", cellName(cell), root.Bounds, cellWidth, cellHeight)
	}

	target, ok := tt.AccessByRole(rendered.def.Role, labelFor(cell))
	if !ok || target.Name == "" {
		t.Fatalf("%s: target lacks its role and label: %+v, found %v", cellName(cell), target, ok)
	}
	if target.Offscreen || target.Bounds.W <= 0 || target.Bounds.H <= 0 {
		t.Fatalf("%s: target is clipped or has zero size: %+v", cellName(cell), target)
	}
	if !inside(target.Bounds, root.Bounds) {
		t.Fatalf("%s: target bounds %+v escape cell bounds %+v", cellName(cell), target.Bounds, root.Bounds)
	}
	if text, found := tt.Find(labelFor(cell)); !found || text.W <= 0 || text.H <= 0 || !inside(text, root.Bounds) {
		t.Fatalf("%s: text is clipped: bounds %+v, found %v", cellName(cell), text, found)
	}
	for _, info := range tt.AccessAll() {
		switch info.Role {
		case ui.RoleButton, ui.RoleTextField, ui.RoleProgress, ui.RoleTabList:
			if info.Name == "" || info.Offscreen || info.Bounds.W <= 0 || info.Bounds.H <= 0 {
				t.Fatalf("%s: inaccessible interactive element: %+v", cellName(cell), info)
			}
		}
	}
	if cell.State == Disabled && !target.Disabled {
		t.Fatalf("%s: disabled state is not exposed", cellName(cell))
	}
	if cell.State == Loading && isButton(cell.Component) && (!target.Disabled || !target.Busy) {
		t.Fatalf("%s: loading state is not exposed as busy and disabled: %+v", cellName(cell), target)
	}
	if cell.State == Selected && isButton(cell.Component) && !target.Checked {
		t.Fatalf("%s: selected state is not exposed", cellName(cell))
	}
	if cell.State == Invalid && cell.Component == InputFrame && !target.Invalid {
		t.Fatalf("%s: invalid state is not exposed", cellName(cell))
	}
	if cell.State == FocusVisible && cell.Component != Spinner && !target.Focused {
		t.Fatalf("%s: keyboard focus did not reach the target", cellName(cell))
	}
}

func inside(inner, outer ui.Rect) bool {
	const tolerance = 0.5
	return inner.X >= outer.X-tolerance && inner.Y >= outer.Y-tolerance &&
		inner.X+inner.W <= outer.X+outer.W+tolerance && inner.Y+inner.H <= outer.Y+outer.H+tolerance
}

func TestCardIsOpaqueOnGlass(t *testing.T) {
	session := newCellSession(Default)
	for _, surface := range []Surface{Paper, Chrome, Glass} {
		rendered, err := session.render(Cell{Component: Card, State: Rest, Surface: surface}, true)
		if err != nil {
			t.Fatal(err)
		}
		bounds := rendered.target.Bounds
		x, y := int(bounds.X+bounds.W/2), int(bounds.Y+bounds.H-4)
		r, g, b, a := rendered.image.At(x, y).RGBA()
		if r != 0xffff || g != 0xffff || b != 0xffff || a != 0xffff {
			t.Errorf("Card interior on %s = rgba(%d, %d, %d, %d), want opaque white", surface, r, g, b, a)
		}
	}
}

func TestGalleryRejectsFrameAwayFromPaper(t *testing.T) {
	frame, ok := Lookup(Frame)
	if !ok {
		t.Fatal("Frame is not registered")
	}
	if err := Validate(Cell{Component: Frame, Surface: Paper}); err != nil {
		t.Fatalf("Paper must be accepted: %v", err)
	}
	for _, surface := range []Surface{Chrome, Glass} {
		if err := frame.ValidateSurface(surface); err == nil {
			t.Errorf("Frame on %s passed its surface constraint", surface)
		}
		if err := Validate(Cell{Component: Frame, Surface: surface}); err == nil {
			t.Errorf("Frame cell on %s passed validation", surface)
		}
		if _, err := RenderSheet(Frame, Default, surface); err == nil {
			t.Errorf("Frame sheet on %s rendered without an error", surface)
		}
	}
}

func TestGalleryButtonsChangeWhilePressedAndRecover(t *testing.T) {
	components := []Component{
		Button, AccentButton, QuietButton, DangerButton, DangerOutlineButton, LinkButton, IconButton, Group,
	}
	combos := []struct {
		preference Preference
		surface    Surface
	}{
		{Default, Paper},
		{Default, Glass},
		{HighContrast, Chrome},
		{ReduceTransparency, Glass},
	}
	sessions := make(map[Preference]*cellSession, len(Preferences))
	for _, pref := range Preferences {
		sessions[pref] = newCellSession(pref)
	}
	for _, component := range components {
		for _, combo := range combos {
			t.Run(fmt.Sprintf("%s/%s/%s", component, combo.preference, combo.surface), func(t *testing.T) {
				cell := Cell{Component: component, State: Rest, Preference: combo.preference, Surface: combo.surface}
				rendered, err := sessions[combo.preference].render(cell, true)
				if err != nil {
					t.Fatal(err)
				}
				bounds := rendered.target.Bounds
				rest := region(rendered.image, bounds)
				x, y := bounds.X+bounds.W/2, bounds.Y+bounds.H/2
				rendered.tester.Press(x, y)
				rendered.tester.Advance(300 * time.Millisecond)
				if changed := differs(rest, region(rendered.tester.Image(), bounds)); changed < len(rest)/4 {
					t.Fatalf("pressed look changes %d of %d pixels, want at least 25%%", changed, len(rest))
				}
				rendered.tester.Release(x, y)
				rendered.tester.Move(0, 0)
				rendered.tester.Advance(300 * time.Millisecond)
				if changed := differs(rest, region(rendered.tester.Image(), bounds)); changed != 0 {
					t.Fatalf("released button did not return to rest: %d pixels differ", changed)
				}
			})
		}
	}
}

func isButton(component Component) bool {
	if component == IconButton || component == Group {
		return true
	}
	for _, axis := range buttonAxes {
		if axis.name == component {
			return true
		}
	}
	return false
}

func region(img *image.RGBA, bounds ui.Rect) []uint32 {
	var pixels []uint32
	for y := int(bounds.Y); y < int(bounds.Y+bounds.H); y++ {
		for x := int(bounds.X); x < int(bounds.X+bounds.W); x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			pixels = append(pixels, r+g+b)
		}
	}
	return pixels
}

func differs(a, b []uint32) int {
	changed := 0
	for i := range a {
		if a[i] != b[i] {
			changed++
		}
	}
	return changed
}
