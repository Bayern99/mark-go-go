package nativefeynman

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

var alphaModes = []string{"semantic", "keyword", "both", "agentic", "all"}

func (p *panel) alphaSectionState(key string) *bool {
	if p.collapsed == nil {
		p.collapsed = make(map[string]*bool)
	}
	if p.collapsed[key] == nil {
		value := true
		p.collapsed[key] = &value
	}
	return p.collapsed[key]
}

func (p *panel) alphaXiv(c *ui.Context) {
	s := p.state
	ui.Row(c.Key("alpha-header")).Gap(portrait.Form.Gap).Margin(0, 0, portrait.Form.Gap, 0).AlignItems(ui.Center).Children(func() {
		sectionTitle(c, "alphaXiv research")
		ui.Row(c).Grow(1)
		if portrait.ButtonWith(c, "Account…", "alphaXiv account settings", portrait.ButtonSpec{Look: portrait.Raised, Size: portrait.Sizes.Compact, Leading: portrait.IconGear}).Clicked() {
			p.openSettings("search")
		}
	})
	ui.Scroll(c.Key("alpha-scroll")).Grow(1).MinHeight(0).FillWidth().TrackScroll(&p.alphaScroll).Label("alphaXiv search and paper tools").Children(func() {
		portrait.Card(c.Key("alpha-controls")).Gap(portrait.Form.Gap).Padding(portrait.GenUI.CardPadding).Radius(portrait.Radii.L).Children(func() {
			activeMode := p.alphaMode
			if activeMode == "" {
				activeMode = "semantic"
			}
			selectedMode := max(0, slices.Index(alphaModes, activeMode))
			modeLabels := make([]string, len(alphaModes))
			for i, mode := range alphaModes {
				modeLabels[i] = titleWord(mode)
			}
			portrait.Segmented(c.Key("alpha-search-modes"), &selectedMode, modeLabels, "alphaXiv search modes")
			p.alphaMode = alphaModes[selectedMode]
			ui.Row(c).Wrap().Gap(portrait.Form.Gap).Children(func() {
				search := portrait.SearchField(c, &p.alphaQuery, portrait.SearchSpec{Label: "Search papers", Placeholder: "Search alphaXiv papers…"})
				search.Frame().Basis(160).Grow(1).MinWidth(0)
				if search.Input().Submitted() && strings.TrimSpace(p.alphaQuery) != "" && !s.cliBusy {
					p.alphaSearch()
				}
				if portrait.AccentButton(c, "Search", "Search alphaXiv").Disabled(s.cliBusy || strings.TrimSpace(p.alphaQuery) == "").Clicked() {
					p.alphaSearch()
				}
			})
		})
		if s.searchResult != nil {
			p.alphaResults(c, *s.searchResult)
		}
		portrait.Disclosure(c.Key("alpha-paper-tools"), "Paper tools", p.alphaSectionState("alpha-paper-tools-open"), portrait.DisclosureSpec{}, func() {
			ui.Row(c).Wrap().Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
				input(c, "alpha-paper-id", "Paper ID", &p.alphaPaperID, "arXiv ID or alphaXiv ID").Basis(200).Grow(1)
				ui.Row(c).Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
					portrait.CompactCheckbox(c.Key("alpha-full-text"), &p.alphaFullText, "Full text")
					if controlButton(c, "Inspect", "Get alphaXiv paper details").Disabled(s.cliBusy || strings.TrimSpace(p.alphaPaperID) == "").Clicked() {
						p.alphaInspect()
					}
				})
			})
			if s.paperResult != nil {
				p.alphaPaper(c, *s.paperResult)
			}
			ui.Row(c).Wrap().Gap(portrait.Form.Gap).Children(func() {
				input(c, "alpha-ask", "Ask about paper", &p.alphaQuestion, "Question about this paper").Basis(160).Grow(1)
				if controlButton(c, "Ask", "Ask alphaXiv about this paper").Disabled(s.cliBusy || p.alphaPaperID == "" || p.alphaQuestion == "").Clicked() {
					p.alphaAskAction()
				}
			})
			if p.alphaAskResult != nil {
				resultText(c, p.alphaAskResult.Command, p.alphaAskResult.RawOutput, p.alphaAskResult.Error)
			}
			ui.Row(c).Wrap().Gap(portrait.Form.Gap).Children(func() {
				input(c, "alpha-repo", "Repository URL", &p.alphaRepoURL, "https://github.com/…").Basis(150).Grow(1)
				input(c, "alpha-file", "Repository file", &p.alphaFilePath, "Optional file path").Basis(150).Grow(1)
				if controlButton(c, "Inspect code", "Inspect repository code for this paper").Disabled(s.cliBusy || p.alphaRepoURL == "").Clicked() {
					p.alphaCodeAction()
				}
			})
			if p.alphaCodeResult != nil {
				resultText(c, p.alphaCodeResult.Command, p.alphaCodeResult.RawOutput, p.alphaCodeResult.Error)
			}
		})
		portrait.Disclosure(c.Key("alpha-annotations"), "Annotations", p.alphaSectionState("alpha-annotations-open"), portrait.DisclosureSpec{}, func() {
			input(c, "alpha-note", "Paper note", &p.alphaNoteInput, "Private paper annotation")
			ui.Row(c).Wrap().Gap(portrait.Form.Gap).Children(func() {
				for _, action := range []string{"list", "get", "set", "clear"} {
					if controlButton(c, titleWord(action), "alphaXiv annotation "+action).Disabled(s.cliBusy || (action != "list" && p.alphaPaperID == "")).Clicked() {
						p.alphaAnnotate(action)
					}
				}
			})
			if p.alphaAnnotation != nil {
				if p.alphaAnnotation.Parsed {
					for _, annotation := range p.alphaAnnotation.Annotations {
						text(c, portrait.Type.Meta, annotation.PaperID+" · "+annotation.Note).TextColor(portrait.Colors.Ink).FillWidth().Wrap()
					}
					if p.alphaAnnotation.Note != "" {
						text(c, portrait.Type.Meta, p.alphaAnnotation.Note).TextColor(portrait.Colors.Ink).FillWidth().Wrap()
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

func titleWord(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
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
		ui.Column(c.Key(fmt.Sprintf("alpha-group-%d", i))).Gap(4).Padding(6, 0).Children(func() {
			if group.Mode != "" && len(groups) > 1 {
				text(c, portrait.Type.Meta, group.Mode).FontWeight(600).TextColor(portrait.Colors.Accent)
			}
			if len(group.Papers) == 0 {
				mutedText(c, "No matching alphaXiv papers.")
			}
			for _, paper := range group.Papers {
				portrait.Card(c.Key("alpha-paper-"+paper.ID)).Gap(3).Padding(portrait.GenUI.CardPadding).Radius(portrait.Radii.L).Background(portrait.Colors.Paper).Border(0.5, portrait.Colors.Line).Children(func() {
					text(c, portrait.Type.UI, paper.Title).FontWeight(600).TextColor(portrait.Colors.Ink).FillWidth().Wrap()
					meta := strings.Join(nonEmpty(paper.Authors, paper.Published, paper.ID), " · ")
					mutedText(c, meta)
					if paper.Summary != "" {
						markdown.View(c.Key("alpha-summary-"+paper.ID), paper.Summary, markdownOptions(c, &markdown.Options{})).FillWidth()
					}
					ui.Row(c).Wrap().Gap(4).Children(func() {
						if portrait.Button(c, "Inspect", "Inspect paper "+paper.ID).Disabled(paper.ID == "" || p.state.cliBusy).Clicked() {
							p.alphaPaperID = paper.ID
							p.alphaInspect()
						}
						if portrait.Button(c, "Cite in Chat", "Insert paper citation into Feynman chat").Clicked() {
							p.state.draft = strings.TrimSpace(p.state.draft + "\n\n" + paperCitation(paper))
							p.state.selectedTab = "chat"
						}
					})
				})
			}
		})
	}
}

func nonEmpty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
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
	portrait.Card(c.Key("alpha-paper-result")).Gap(portrait.Form.HelpGap).Padding(portrait.GenUI.CardPadding).Radius(portrait.Radii.L).Background(portrait.Colors.Paper).Border(0.5, portrait.Colors.Line).Children(func() {
		text(c, portrait.Type.UI, result.Title).FontWeight(600).TextColor(portrait.Colors.Ink).FillWidth().Wrap()
		mutedText(c, result.PaperID+map[bool]string{true: " · full text", false: " · abstract"}[result.FullText])
		if result.Content != "" {
			markdown.View(c.Key("alpha-paper-content"), result.Content, markdownOptions(c, &markdown.Options{WrapCode: true})).FillWidth()
		}
		cite := controlButton(c, "Cite in Chat", "Insert paper details into the Feynman prompt").AlignSelf(ui.Start)
		if cite.Clicked() {
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
