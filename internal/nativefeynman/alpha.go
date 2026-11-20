package nativefeynman

import (
	"fmt"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

var alphaModes = []string{"semantic", "keyword", "both", "agentic", "all"}

func (p *panel) alphaXiv(c *ui.Context) {
	s := p.state
	ui.Row(c.Key("alpha-header")).Gap(4).AlignItems(ui.Center).Children(func() {
		sectionTitle(c, "alphaXiv research")
		ui.Row(c).Grow(1)
		if smallButton(c, "Back to Chat", "Return to Feynman conversation").Clicked() {
			s.selectedTab = "chat"
		}
	})
	ui.Scroll(c.Key("alpha-scroll")).Grow(1).MinHeight(0).FillWidth().TrackScroll(&p.alphaScroll).Label("alphaXiv search and paper tools").Children(func() {
		ui.Column(c.Key("alpha-controls")).Gap(5).Padding(7).Radius(8).Background(trackColor).Children(func() {
			ui.Row(c).Wrap().Gap(4).Children(func() {
				for _, mode := range alphaModes {
					button := smallButton(c, mode, "Search alphaXiv using "+mode+" mode")
					if p.alphaMode == mode {
						button.Background(ui.Hex("#e8f0ed")).TextColor(accentColor)
					}
					if button.Clicked() {
						p.alphaMode = mode
					}
				}
			})
			ui.Row(c).Gap(4).Children(func() {
				input(c, "alpha-query", "Search papers", &p.alphaQuery, "Search alphaXiv papers…")
				if controlButton(c, "Search", "Search alphaXiv").Disabled(s.cliBusy || strings.TrimSpace(p.alphaQuery) == "").Clicked() {
					p.alphaSearch()
				}
			})
		})
		if s.searchResult != nil {
			p.alphaResults(c, *s.searchResult)
		}
		ui.Column(c.Key("alpha-paper-tools")).Gap(5).Padding(7).Radius(8).Background(trackColor).Children(func() {
			sectionTitle(c, "Paper tools")
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				input(c, "alpha-paper-id", "Paper ID", &p.alphaPaperID, "arXiv ID or alphaXiv ID")
				ui.Checkbox(c.Key("alpha-full-text"), &p.alphaFullText, "Full text")
				if controlButton(c, "Inspect", "Get alphaXiv paper details").Disabled(s.cliBusy || strings.TrimSpace(p.alphaPaperID) == "").Clicked() {
					p.alphaInspect()
				}
			})
			if s.paperResult != nil {
				p.alphaPaper(c, *s.paperResult)
			}
			ui.Row(c).Gap(4).Children(func() {
				input(c, "alpha-ask", "Ask about paper", &p.alphaQuestion, "Question about this paper")
				if smallButton(c, "Ask", "Ask alphaXiv about this paper").Disabled(s.cliBusy || p.alphaPaperID == "" || p.alphaQuestion == "").Clicked() {
					p.alphaAskAction()
				}
			})
			if p.alphaAskResult != nil {
				resultText(c, p.alphaAskResult.Command, p.alphaAskResult.RawOutput, p.alphaAskResult.Error)
			}
			ui.Row(c).Gap(4).Children(func() {
				input(c, "alpha-repo", "Repository URL", &p.alphaRepoURL, "https://github.com/…")
				input(c, "alpha-file", "Repository file", &p.alphaFilePath, "Optional file path")
				if smallButton(c, "Inspect code", "Inspect repository code for this paper").Disabled(s.cliBusy || p.alphaRepoURL == "").Clicked() {
					p.alphaCodeAction()
				}
			})
			if p.alphaCodeResult != nil {
				resultText(c, p.alphaCodeResult.Command, p.alphaCodeResult.RawOutput, p.alphaCodeResult.Error)
			}
		})
		ui.Column(c.Key("alpha-annotations")).Gap(5).Padding(7).Radius(8).Background(trackColor).Children(func() {
			sectionTitle(c, "Annotations")
			input(c, "alpha-note", "Paper note", &p.alphaNoteInput, "Private paper annotation")
			ui.Row(c).Wrap().Gap(4).Children(func() {
				for _, action := range []string{"list", "get", "set", "clear"} {
					if smallButton(c, strings.Title(action), "alphaXiv annotation "+action).Disabled(s.cliBusy || (action != "list" && p.alphaPaperID == "")).Clicked() {
						p.alphaAnnotate(action)
					}
				}
			})
			if p.alphaAnnotation != nil {
				if p.alphaAnnotation.Parsed {
					for _, annotation := range p.alphaAnnotation.Annotations {
						ui.Text(c, annotation.PaperID+" · "+annotation.Note).FillWidth().FontSize(11)
					}
					if p.alphaAnnotation.Note != "" {
						ui.Text(c, p.alphaAnnotation.Note).FillWidth().FontSize(11)
					}
				} else {
					resultText(c, p.alphaAnnotation.Command, p.alphaAnnotation.RawOutput, p.alphaAnnotation.Error)
				}
			}
		})
		if s.alphaOutput != "" {
			resultText(c, "alphaXiv", s.alphaOutput, s.cliError)
		}
	})
}

func (p *panel) alphaSearch() {
	root, query, mode := p.state.root, strings.TrimSpace(p.alphaQuery), p.alphaMode
	if mode == "" {
		mode = "semantic"
	}
	p.runUpdate(func() (func(), error) {
		result, err := p.host.CLIAlphaSearch(p.window.Context(), root, query, mode)
		return func() {
			p.state.searchResult = &result
			if first := firstAlphaPaper(result); first != nil && p.alphaPaperID == "" {
				p.alphaPaperID = first.ID
			}
		}, err
	})
}

func firstAlphaPaper(result feynman.CLIAlphaSearchResult) *feynman.CLIAlphaPaper {
	for _, group := range result.Groups {
		if len(group.Papers) > 0 {
			return &group.Papers[0]
		}
	}
	if len(result.Papers) > 0 {
		return &result.Papers[0]
	}
	return nil
}

func (p *panel) alphaResults(c *ui.Context, result feynman.CLIAlphaSearchResult) {
	if !result.Parsed {
		resultText(c, result.Command, result.RawOutput, result.Error)
		return
	}
	groups := result.Groups
	if len(groups) == 0 {
		groups = []feynman.CLIAlphaSearchGroup{{Mode: result.Mode, Papers: result.Papers}}
	}
	for i, group := range groups {
		ui.Column(c.Key(fmt.Sprintf("alpha-group-%d", i))).Gap(4).Padding(6, 7).Children(func() {
			if group.Mode != "" {
				ui.Text(c, group.Mode).FontSize(11).Bold().TextColor(accentColor)
			}
			if len(group.Papers) == 0 {
				mutedText(c, "No matching alphaXiv papers.")
			}
			for _, paper := range group.Papers {
				ui.Column(c.Key("alpha-paper-" + paper.ID)).Gap(3).Padding(6).Radius(7).Background(paperColor).Children(func() {
					ui.Text(c, paper.Title).FontSize(12).Bold().FillWidth()
					meta := strings.Join([]string{paper.Authors, paper.Published, paper.ID}, " · ")
					mutedText(c, meta)
					if paper.Summary != "" {
						markdown.View(c.Key("alpha-summary-"+paper.ID), paper.Summary, &markdown.Options{}).FillWidth()
					}
					ui.Row(c).Wrap().Gap(4).Children(func() {
						if smallButton(c, "Inspect", "Inspect paper "+paper.ID).Disabled(paper.ID == "" || p.state.cliBusy).Clicked() {
							p.alphaPaperID = paper.ID
							p.alphaInspect()
						}
						if smallButton(c, "Cite in Chat", "Insert paper citation into Feynman chat").Clicked() {
							p.state.draft = strings.TrimSpace(p.state.draft + "\n\n" + paperCitation(paper))
							p.state.selectedTab = "chat"
						}
					})
				})
			}
		})
	}
}

func (p *panel) alphaInspect() {
	root, id, full := p.state.root, strings.TrimSpace(p.alphaPaperID), p.alphaFullText
	p.runUpdate(func() (func(), error) {
		result, err := p.host.CLIAlphaGet(p.window.Context(), root, id, full)
		return func() {
			p.state.paperResult = &result
			if result.Annotation != "" {
				p.alphaNoteInput = result.Annotation
			}
		}, err
	})
}

func (p *panel) alphaPaper(c *ui.Context, result feynman.CLIAlphaPaperResult) {
	if !result.Parsed {
		resultText(c, result.Command, result.RawOutput, result.Error)
		return
	}
	ui.Column(c.Key("alpha-paper-result")).Gap(4).Padding(6, 7).Radius(7).Background(paperColor).Children(func() {
		ui.Text(c, result.Title).Bold().FontSize(13).FillWidth()
		mutedText(c, result.PaperID+map[bool]string{true: " · full text", false: " · abstract"}[result.FullText])
		if result.Content != "" {
			markdown.View(c.Key("alpha-paper-content"), result.Content, &markdown.Options{WrapCode: true}).FillWidth()
		}
		if smallButton(c, "Cite in Chat", "Insert paper details into the Feynman prompt").Clicked() {
			p.state.draft = strings.TrimSpace(p.state.draft + "\n\nalphaXiv paper " + result.PaperID + ": " + result.Title + "\n" + result.Content)
			p.state.selectedTab = "chat"
		}
	})
}

func paperCitation(paper feynman.CLIAlphaPaper) string {
	parts := []string{"alphaXiv paper " + paper.ID + ": " + paper.Title}
	if paper.Authors != "" {
		parts = append(parts, "Authors: "+paper.Authors)
	}
	if paper.URL != "" {
		parts = append(parts, "URL: "+paper.URL)
	}
	if paper.Summary != "" {
		parts = append(parts, paper.Summary)
	}
	return strings.Join(parts, "\n")
}

func (p *panel) alphaAskAction() {
	root, id, question := p.state.root, strings.TrimSpace(p.alphaPaperID), strings.TrimSpace(p.alphaQuestion)
	p.runUpdate(func() (func(), error) {
		result, err := p.host.CLIAlphaAsk(p.window.Context(), root, id, question)
		return func() { p.alphaAskResult = &result }, err
	})
}

func (p *panel) alphaCodeAction() {
	root, repo, path := p.state.root, strings.TrimSpace(p.alphaRepoURL), strings.TrimSpace(p.alphaFilePath)
	p.runUpdate(func() (func(), error) {
		result, err := p.host.CLIAlphaCode(p.window.Context(), root, repo, path)
		return func() { p.alphaCodeResult = &result }, err
	})
}

func (p *panel) alphaAnnotate(action string) {
	root, id := p.state.root, strings.TrimSpace(p.alphaPaperID)
	note := ""
	if action == "set" {
		note = strings.TrimSpace(p.alphaNoteInput)
	}
	p.runUpdate(func() (func(), error) {
		result, err := p.host.CLIAlphaAnnotate(p.window.Context(), root, action, id, note)
		return func() {
			p.alphaAnnotation = &result
			if action == "get" && result.Note != "" {
				p.alphaNoteInput = result.Note
			}
			if action == "clear" {
				p.alphaNoteInput = ""
			}
		}, err
	})
}
