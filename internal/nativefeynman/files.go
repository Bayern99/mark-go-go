package nativefeynman

import (
	"path/filepath"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/egoist/mygo/ui"
)

func (p *panel) files(c *ui.Context) {
	s := p.state
	ui.Row(c.Key("research-files-heading")).Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
		sectionTitle(c, "Research files")
		ui.Row(c).Grow(1)
		if controlButton(c, "Refresh", "Refresh research artifacts").Disabled(s.filesLoading || s.status == statusWorking).Clicked() {
			p.refreshFiles(s.root)
		}
		if controlButton(c, "Open folder", "Reveal the research output folder").Clicked() {
			p.run(func() error { return p.host.RevealOutputs(p.window.Context(), s.root) })
		}
	})
	if s.filesError != "" {
		portrait.Alert(c, portrait.Tones.Danger, "Research files", s.filesError)
	}
	ui.Scroll(c.Key("research-artifacts")).Grow(1).MinHeight(0).FillWidth().TrackScroll(&p.filesScroll).Label("Research files and recovery").Children(func() {
		if s.filesLoading && len(s.artifacts) == 0 {
			ui.Column(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Padding(portrait.Form.DialogPad).Children(func() {
				portrait.Empty(c, portrait.EmptySpec{Icon: portrait.IconFolderOpen, Title: "Loading research files…"})
			})
		}
		if len(s.artifacts) == 0 && !s.filesLoading {
			ui.Column(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Padding(portrait.Form.DialogPad).Children(func() {
				portrait.Empty(c, portrait.EmptySpec{Icon: portrait.IconFileText, Title: "No research files yet", Description: "Completed research files appear here."})
			})
		}
		if len(s.artifacts) == 0 {
			return
		}
		list := portrait.Card(c).FillWidth().Padding(0).Radius(portrait.Radii.L).Clip()
		list.Children(func() {
			for i, file := range s.artifacts {
				if i > 0 {
					ui.Box(c).FillWidth().Height(portrait.Editor.Hairline).Background(portrait.Colors.Line)
				}
				p.fileRow(c, file)
			}
		})
	})
}

func (p *panel) fileRow(c *ui.Context, file review.ResearchArtifact) {
	action, hint := "Open", "Open the published research file."
	icon := portrait.IconFileText
	if file.Missing {
		action, hint, icon = "Restore", "The published workspace file was removed. Restore copies the saved artifact back into its original path.", portrait.IconArrowCounterClockwise
	} else if file.Proposal {
		action, hint, icon = "Review", "A Feynman edit is available. Review submits it to the shared document review panel.", portrait.IconSparkle
	}
	row := ui.Column(c.Key("research-file-"+file.Path)).FillWidth().Gap(portrait.Form.HelpGap).Padding(portrait.Form.AreaPadY, portrait.Form.PadX)
	row.Children(func() {
		ui.Row(c).Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
			portrait.Glyph(c, icon, portrait.IconSizes.M, portrait.Colors.Accent).Shrink(0)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				portrait.Tooltip(c, text(c, portrait.Type.UI, filepath.Base(file.Path)).FontWeight(500).TextColor(portrait.Colors.Ink).MaxLines(1).Ellipsis("…"), file.Path+" · "+hint)
				if strings.Contains(file.Path, "/") {
					portrait.Caption(c, file.Path[:strings.LastIndex(file.Path, "/")]).MaxLines(1).Ellipsis("…")
				}
			})
			if file.Error == "" && (file.Missing || file.Target != "") {
				description := "Open the workspace target and prepare its review proposal"
				if file.Missing {
					description = "Restore the deleted research artifact"
				}
				button := portrait.ButtonWith(c, action, description, portrait.ButtonSpec{Look: portrait.Raised, Size: portrait.Sizes.Regular})
				button.Disabled(p.state.status == statusWorking && (file.Proposal || file.Missing))
				if button.Clicked() {
					p.openArtifact(file)
				}
			}
		})
		if file.Error != "" {
			portrait.Alert(c, portrait.Tones.Danger, file.Path, file.Error)
		}
	})
}
