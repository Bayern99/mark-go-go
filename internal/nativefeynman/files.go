package nativefeynman

import (
	"fmt"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/egoist/mygo/ui"
)

func (p *panel) files(c *ui.Context) {
	s := p.state
	ui.Row(c.Key("research-files-heading")).Gap(4).AlignItems(ui.Center).Children(func() {
		sectionTitle(c, "Research files")
		ui.Row(c).Grow(1)
		if smallButton(c, "Refresh", "Refresh research artifacts").Disabled(s.filesLoading || s.status == statusWorking).Clicked() {
			p.refreshFiles(s.root)
		}
		if smallButton(c, "Open folder", "Reveal the research output folder").Clicked() {
			p.run(func() error { return p.host.RevealOutputs(p.window.Context(), s.root) })
		}
	})
	if s.filesError != "" {
		text(c, portrait.Type.Meta, s.filesError).Role(ui.RoleAlert).TextColor(portrait.Colors.Danger).FillWidth().Wrap()
	}
	ui.Scroll(c.Key("research-artifacts")).Grow(1).MinHeight(0).FillWidth().TrackScroll(&p.filesScroll).Label("Research files and recovery").Children(func() {
		if s.filesLoading && len(s.artifacts) == 0 {
			mutedText(c, "Loading research files…")
		}
		if len(s.artifacts) == 0 && !s.filesLoading {
			mutedText(c, "Completed research files appear here.")
		}
		for _, file := range s.artifacts {
			p.fileRow(c, file)
		}
	})
}

func (p *panel) fileRow(c *ui.Context, file review.ResearchArtifact) {
	label := file.Path
	if file.Missing {
		label = "Restore · " + label
	} else if file.Proposal {
		label = "Review · " + label
	}
	ui.Column(c.Key("research-file-"+file.Path)).FillWidth().Gap(3).Padding(6, 7).Radius(9).Background(portrait.Colors.Fill).Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			portrait.Tooltip(c, text(c, portrait.Type.UI, label).Grow(1).MinWidth(0).FontWeight(600).TextColor(portrait.Colors.Ink).MaxLines(2).Ellipsis("…"), file.Path)
			if file.Missing && file.Error == "" {
				if controlButton(c, "Restore", "Restore the deleted research artifact").Disabled(p.state.status == statusWorking).Clicked() {
					p.openArtifact(file)
				}
			} else if file.Target != "" && file.Error == "" {
				buttonLabel := "Open"
				if file.Proposal {
					buttonLabel = "Review in document panel"
				}
				if controlButton(c, buttonLabel, "Open the workspace target and prepare its review proposal").Disabled(p.state.status == statusWorking && file.Proposal).Clicked() {
					p.openArtifact(file)
				}
			}
		})
		if strings.Contains(file.Path, "/") {
			mutedText(c, file.Path[:strings.LastIndex(file.Path, "/")])
		}
		if file.Error != "" {
			text(c, portrait.Type.Meta, fmt.Sprintf("%s · %s", file.Path, file.Error)).Role(ui.RoleAlert).TextColor(portrait.Colors.Danger).FillWidth().Wrap()
		}
		if file.Missing {
			mutedText(c, "The published workspace file was removed. Restore copies the saved artifact back into its original path.")
		} else if file.Proposal {
			mutedText(c, "A Feynman edit is available. Review submits it to the shared document review panel.")
		}
	})
}
