// Package portrait holds the Portrait design tokens shared by every native
// surface: font stacks, the type scale, colors, materials and motion. Values
// follow the retired WebUI at c6504af (ADR 0010), light theme only (ADR 0006).
package portrait

import "github.com/egoist/mygo/ui"

// Font stacks. MyGo resolves a comma-separated family list glyph by glyph, so
// Han in a Latin run falls back to IBM Plex Sans SC without switching the
// whole string.
const (
	FontSans    = "IBM Plex Sans, IBM Plex Sans SC"
	FontDisplay = "Barlow Semi Condensed, IBM Plex Sans, IBM Plex Sans SC"
	FontMono    = "IBM Plex Mono, IBM Plex Sans SC"
	FontIcons   = "Phosphor"
)

// DisplaySizeAdjust mirrors the WebUI's "MGG Condensed" face, Barlow Semi
// Condensed with size-adjust: 105%.
const DisplaySizeAdjust = 1.05

// TypeStyle is one step of the type scale. Size and Leading are nominal CSS
// pixels; Tracking is letter spacing in DIPs.
type TypeStyle struct {
	Family                  string
	Size, Leading, Tracking float32
}

// In returns the style set in another font stack.
func (s TypeStyle) In(family string) TypeStyle { s.Family = family; return s }

// RenderedSize is the font size to draw with, including the display size adjust.
func (s TypeStyle) RenderedSize() float32 {
	if s.Family == FontDisplay {
		return s.Size * DisplaySizeAdjust
	}
	return s.Size
}

// LineHeight is Leading as a multiple of RenderedSize, for APIs such as the
// editor's that take a multiplier.
func (s TypeStyle) LineHeight() float32 { return s.Leading / s.RenderedSize() }

// Font is the style as a ui.Font of the given weight.
func (s TypeStyle) Font(weight int) ui.Font {
	return ui.Font{Family: s.Family, Size: s.RenderedSize(), Weight: weight}
}

// Apply sets family, size, fixed leading and tracking on e.
func (s TypeStyle) Apply(e ui.Element) ui.Element {
	e.Font(s.Family).FontSize(s.RenderedSize()).FixedLineHeight(s.Leading)
	if s.Tracking != 0 {
		e.LetterSpacing(s.Tracking)
	}
	return e
}

// TypeScale is the closed set of text sizes; no text is smaller than Meta.
type TypeScale struct {
	Display, Title, Reading, Small, UI, Control, Meta, Code TypeStyle
}

// Type is the Portrait type scale.
var Type = TypeScale{
	Display: TypeStyle{FontSans, 28, 36, 0},
	Title:   TypeStyle{FontSans, 20, 28, 0},
	Reading: TypeStyle{FontSans, 16, 28, 0.01 * 16},
	Small:   TypeStyle{FontSans, 14, 22, 0},
	UI:      TypeStyle{FontSans, 13, 18, 0},
	Control: TypeStyle{FontSans, 13, 20, 0},
	Meta:    TypeStyle{FontSans, 12, 16, 0},
	Code:    TypeStyle{FontMono, 13, 22, 0},
}

// Palette lists the Portrait colors.
type Palette struct {
	Chrome, Paper, Code                  ui.Color
	Ink, Ink2, Ink3, Ink4                ui.Color
	Line, LineStrong                     ui.Color
	Fill, FillHover, FillStrong          ui.Color
	Accent, OnAccent, AccentSoft         ui.Color
	Warn, WarnSoft, Danger, DangerSoft   ui.Color
	Note, NoteSoft, Warning, WarningSoft ui.Color
	Caution, CautionSoft                 ui.Color
	Focus, Scrim                         ui.Color
}

// Colors is the light Portrait palette.
var Colors = Palette{
	Chrome: ui.Hex("#f2f2f4"), Paper: ui.Hex("#ffffff"), Code: ui.Hex("#f6f6f8"),
	Ink: ui.Hex("#1d1d1f"), Ink2: ui.Hex("#48484d"), Ink3: ui.Hex("#6e6e73"), Ink4: ui.Hex("#a1a1a6"),
	Line: black(0.08), LineStrong: black(0.16),
	Fill: black(0.045), FillHover: black(0.07), FillStrong: black(0.11),
	Accent: ui.Hex("#2f6f62"), OnAccent: ui.Hex("#ffffff"), AccentSoft: ui.RGBA(47, 111, 98, 0.1),
	Warn: ui.Hex("#9a4d12"), WarnSoft: ui.RGBA(154, 77, 18, 0.09),
	Danger: ui.Hex("#a83a3a"), DangerSoft: ui.RGBA(168, 58, 58, 0.08),
	Note: ui.Hex("#2f6f62"), NoteSoft: ui.RGBA(47, 111, 98, 0.1),
	Warning: ui.Hex("#9a4d12"), WarningSoft: ui.RGBA(154, 77, 18, 0.09),
	Caution: ui.Hex("#a83a3a"), CautionSoft: ui.RGBA(168, 58, 58, 0.08),
	Focus: ui.RGBA(47, 111, 98, 0.34), Scrim: ui.RGBA(29, 29, 31, 0.08),
}

func black(a float32) ui.Color { return ui.RGBA(0, 0, 0, a) }
func white(a float32) ui.Color { return ui.RGBA(255, 255, 255, a) }
