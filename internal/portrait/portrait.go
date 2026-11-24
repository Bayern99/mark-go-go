package portrait

import "github.com/egoist/mygo/ui"

const (
	FontSans    = "Inter, IBM Plex Sans SC"
	FontDisplay = "Inter Display, Inter, IBM Plex Sans SC"
	FontMono    = "IBM Plex Mono, IBM Plex Sans SC"
	FontIcons   = "Phosphor"
)

const (
	displayTracking = -0.02
	titleTracking   = -0.012
)

type TypeStyle struct {
	Family                  string
	Size, Leading, Tracking float32
}

func (s TypeStyle) In(family string) TypeStyle { s.Family = family; return s }

func (s TypeStyle) RenderedSize() float32 { return s.Size }

func (s TypeStyle) LineHeight() float32 { return s.Leading / s.RenderedSize() }

func (s TypeStyle) Font(weight int) ui.Font {
	return ui.Font{Family: s.Family, Size: s.RenderedSize(), Weight: weight}
}

func (s TypeStyle) Apply(e ui.Element) ui.Element {
	e.Font(s.Family).FontSize(s.RenderedSize()).FixedLineHeight(s.Leading)
	if s.Tracking != 0 {
		e.LetterSpacing(s.Tracking)
	}
	return e
}

type TypeScale struct {
	Display, Heading, Title, Reading, Small, UI, Control, Meta, Code TypeStyle
	H3, H4, H5, H6, CodeSmall                                        TypeStyle
}

var Type = func() TypeScale {
	scale := TypeScale{
		Display:   TypeStyle{FontSans, 28, 36, displayTracking * 28},
		Title:     TypeStyle{FontSans, 20, 28, titleTracking * 20},
		Reading:   TypeStyle{FontSans, 16, 28, 0},
		Small:     TypeStyle{FontSans, 14, 22, 0},
		UI:        TypeStyle{FontSans, 13, 18, 0},
		Control:   TypeStyle{FontSans, 13, 20, 0},
		Meta:      TypeStyle{FontSans, 12, 16, 0},
		Code:      TypeStyle{FontMono, 13, 22, 0},
		H3:        TypeStyle{FontDisplay, 18, 28, 0},
		H4:        TypeStyle{FontSans, 15, 28, 0},
		H5:        TypeStyle{FontSans, 14, 28, 0},
		H6:        TypeStyle{FontSans, 13, 28, 0},
		CodeSmall: TypeStyle{FontMono, 12, 18, 0},
	}
	scale.Heading = scale.Reading.In(FontDisplay)
	return scale
}()

type Palette struct {
	Chrome, Paper, Code                  ui.Color
	Ink, Ink2, Ink3, Ink4                ui.Color
	Line, LineStrong                     ui.Color
	Fill, FillHover, FillStrong          ui.Color
	Accent, OnAccent, AccentSoft         ui.Color
	Success                              ui.Color
	Warn, WarnSoft, Danger, DangerSoft   ui.Color
	Note, NoteSoft, Warning, WarningSoft ui.Color
	Caution, CautionSoft                 ui.Color
	Focus, Scrim, ModalScrim             ui.Color
}

var Colors Palette

var (
	lightInk = [4]ui.Color{ui.Hex("#1D1B1C"), ui.Hex("#48484D"), ui.Hex("#6E6E73"), ui.Hex("#A1A1A6")}
	darkInk  = [4]ui.Color{ui.Hex("#F2F3F4"), ui.Hex("#CDD1D3"), ui.Hex("#959DA2"), ui.Hex("#6E6E73")}
	onLight  = ui.Hex("#FFFFFF")
	onDark   = ui.Hex("#0E100F")
)

func paletteFor(s Scheme, dark bool) Palette {
	ink, on, veil, soft, focus := lightInk, onLight, black, float32(0.1), float32(0.28)
	scrim, modal := ui.RGBA(29, 27, 28, 0.08), ui.RGBA(29, 27, 28, 0.12)
	lines := [5]float32{0.08, 0.16, 0.045, 0.07, 0.11}
	if dark {
		ink, on, veil, soft, focus = darkInk, onDark, white, 0.16, 0.4
		scrim, modal = black(0.32), black(0.48)
		lines = [5]float32{0.10, 0.20, 0.055, 0.09, 0.14}
	}
	return Palette{
		Chrome: s.Paper.Mix(ink[0], 0.05), Paper: s.Paper, Code: s.Code,
		Ink: ink[0], Ink2: ink[1], Ink3: ink[2], Ink4: ink[3],
		Line: veil(lines[0]), LineStrong: veil(lines[1]),
		Fill: veil(lines[2]), FillHover: veil(lines[3]), FillStrong: veil(lines[4]),
		Accent: s.Accent, OnAccent: on, AccentSoft: s.Accent.Alpha(soft),
		Success: s.Success,
		Warn:    s.Warn, WarnSoft: s.Warn.Alpha(soft - 0.01),
		Danger: s.Danger, DangerSoft: s.Danger.Alpha(soft - 0.02),
		Note: s.Accent, NoteSoft: s.Accent.Alpha(soft),
		Warning: s.Warn, WarningSoft: s.Warn.Alpha(soft - 0.01),
		Caution: s.Danger, CautionSoft: s.Danger.Alpha(soft - 0.02),
		Focus: s.Accent.Alpha(focus), Scrim: scrim, ModalScrim: modal,
	}
}

func black(a float32) ui.Color { return ui.RGBA(0, 0, 0, a) }
func white(a float32) ui.Color { return ui.RGBA(255, 255, 255, a) }
