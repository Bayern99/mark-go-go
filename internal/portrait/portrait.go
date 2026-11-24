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
	H4, H5, H6                                                       TypeStyle
}

var Type = func() TypeScale {
	scale := TypeScale{
		Display: TypeStyle{FontSans, 28, 36, displayTracking * 28},
		Title:   TypeStyle{FontSans, 20, 28, titleTracking * 20},
		Reading: TypeStyle{FontSans, 16, 28, 0},
		Small:   TypeStyle{FontSans, 14, 22, 0},
		UI:      TypeStyle{FontSans, 13, 18, 0},
		Control: TypeStyle{FontSans, 13, 20, 0},
		Meta:    TypeStyle{FontSans, 12, 16, 0},
		Code:    TypeStyle{FontMono, 13, 22, 0},
		H4:      TypeStyle{FontSans, 15, 28, 0},
		H5:      TypeStyle{FontSans, 14, 28, 0},
		H6:      TypeStyle{FontSans, 13, 28, 0},
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

var accent = ui.Hex("#3139fb")

var Colors = Palette{
	Chrome: ui.Hex("#f2f2f4"), Paper: ui.Hex("#ffffff"), Code: ui.Hex("#f6f6f8"),
	Ink: ui.Hex("#1d1d1f"), Ink2: ui.Hex("#48484d"), Ink3: ui.Hex("#6e6e73"), Ink4: ui.Hex("#a1a1a6"),
	Line: black(0.08), LineStrong: black(0.16),
	Fill: black(0.045), FillHover: black(0.07), FillStrong: black(0.11),
	Accent: accent, OnAccent: ui.Hex("#ffffff"), AccentSoft: accent.Alpha(0.1),
	Success: ui.Hex("#197a49"),
	Warn:    ui.Hex("#9a4d12"), WarnSoft: ui.RGBA(154, 77, 18, 0.09),
	Danger: ui.Hex("#a83a3a"), DangerSoft: ui.RGBA(168, 58, 58, 0.08),
	Note: accent, NoteSoft: accent.Alpha(0.1),
	Warning: ui.Hex("#9a4d12"), WarningSoft: ui.RGBA(154, 77, 18, 0.09),
	Caution: ui.Hex("#a83a3a"), CautionSoft: ui.RGBA(168, 58, 58, 0.08),
	Focus: accent.Alpha(0.28), Scrim: ui.RGBA(29, 29, 31, 0.08),
	ModalScrim: ui.RGBA(29, 29, 31, 0.12),
}

func black(a float32) ui.Color { return ui.RGBA(0, 0, 0, a) }
func white(a float32) ui.Color { return ui.RGBA(255, 255, 255, a) }
