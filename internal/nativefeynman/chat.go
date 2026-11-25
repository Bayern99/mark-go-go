package nativefeynman

import (
	"encoding/base64"
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

func (p *panel) chat(c *ui.Context) {
	s := p.state
	if len(s.statuses) > 0 {
		ui.Row(c).Wrap().Gap(4).Children(func() {
			for key, value := range s.statuses {
				portrait.Badge(c, key+": "+value, portrait.Tones.Neutral)
			}
		})
	}
	if s.progress != "" {
		ui.Row(c.Key("research-progress")).Role(ui.RoleStatus).Label(s.progress).Gap(6).Padding(3, 8).AlignItems(ui.Center).Children(func() {
			if s.status == statusWorking {
				portrait.Spinner(c, "Research in progress")
			}
			text(c, portrait.Type.Meta, s.progress).TextColor(portrait.Colors.Ink3).Grow(1).MinWidth(0)
		})
	}
	if len(s.widgets) > 0 {
		card(c).Gap(3).Padding(7, 9).Children(func() {
			for key, widget := range s.widgets {
				for _, line := range widget.WidgetLines {
					text(c, portrait.Type.Meta, line).TextColor(portrait.Colors.Ink2).Label(key)
				}
			}
		})
	}
	wasReady, wasFollowing := p.chatReady, p.chatFollow
	follow := wasFollowing || !wasReady
	if wasReady && p.chatScroll.Y < p.chatLastY-portrait.Editor.Hairline {
		follow = false
	}
	if p.chatScroll.MaxY == 0 || p.chatScroll.Y >= p.chatScroll.MaxY-portrait.Editor.Hairline {
		follow = true
	}
	shouldScroll := follow && (!wasReady || !wasFollowing || p.chatScroll.MaxY > p.chatLastMaxY+portrait.Editor.Hairline)
	p.chatFollow, p.chatReady = follow, true
	p.chatLastY, p.chatLastMaxY = p.chatScroll.Y, p.chatScroll.MaxY
	if shouldScroll {
		p.chatScroll.Y = math.MaxFloat32
	}
	ui.Scroll(c.Key("feynman-chat-scroll")).Grow(1).FillWidth().MinHeight(0).Label("Feynman conversation").TrackScroll(&p.chatScroll).Children(func() {
		if len(s.messages) == 0 && s.status == statusReady {
			text(c, portrait.Type.UI, "Ask Feynman to research this workspace.").TextColor(portrait.Colors.Ink3).Padding(12, 8)
		}
		for i, item := range s.messages {
			p.message(c, i, item, i == len(s.messages)-1 && s.status == statusWorking)
		}
	})
	if s.permission != nil {
		p.question(c, *s.permission)
	}
	if s.status == statusOffline || s.status == statusConnecting {
		ui.Row(c).Padding(4, 6).Gap(5).Children(func() {
			if s.status == statusConnecting {
				mutedText(c, "Connecting to Feynman…")
			} else if controlButton(c, "Reconnect", "Reconnect to Feynman").Clicked() {
				p.reconnect()
			}
		})
	}
	p.composer(c)
}

func (p *panel) message(c *ui.Context, index int, item message, streaming bool) {
	key := fmt.Sprintf("message-%d", index)
	switch item.kind {
	case "user":
		ui.Column(c.Key(key)).FillWidth().AlignItems(ui.End).Padding(3, 6).Children(func() {
			ui.Column(c).MaxWidth(portrait.Form.BubbleRatio*portrait.Form.BubbleMax).Padding(8, 12).Radius(portrait.Radii.L).Background(portrait.Colors.AccentSoft).Children(func() {
				text(c, portrait.Type.UI, item.text).Wrap().TextColor(portrait.Colors.Ink)
			})
			p.image(c, item.image)
		})
	case "assistant":
		reply := ui.Column(c.Key(key)).Gap(3).Padding(4, 8)
		reveal := reply.Hovered() || reply.FocusWithin()
		reply.Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				portrait.Glyph(c, portrait.IconAtom, portrait.IconSizes.M, portrait.Colors.Accent)
				text(c, portrait.Type.Meta, "Feynman").FontWeight(600).TextColor(portrait.Colors.Ink3)
			})
			options := markdownOptions(c, &markdown.Options{
				WrapCode: true,
				LoadImage: func(address string) (ui.ImageSource, error) {
					if strings.HasPrefix(strings.ToLower(address), "data:image/") {
						return p.loadDataImage(address)
					}
					return nil, fmt.Errorf("remote image previews are unavailable")
				},
				OnLink: p.followLink,
			})
			if streaming {
				markdown.Stream(c.Key(key+"-markdown"), item.text, options).FillWidth()
			} else {
				markdown.View(c.Key(key+"-markdown"), item.text, options).FillWidth()
			}
			p.image(c, item.image)
			if item.text != "" && portrait.RevealFor(c, toolButton(c, "Copy", "Copy Feynman response", true, false).AlignSelf(ui.Start), reveal).Clicked() {
				if err := mygo.Clipboard.Write(transfer.TextData(item.text)); err == nil {
					portrait.Notify(c, portrait.ToastSpec{ID: "copy", Tone: portrait.Tones.Success, Title: "Copied"})
				}
			}
		})
	case "thought":
		thoughtKey := item.messageID
		if thoughtKey == "" {
			thoughtKey = key
		}
		open := p.collapsibleState(thoughtKey + "-thought")
		title := "Thought"
		if streaming {
			title = "Thinking…"
		}
		portrait.Disclosure(c.Key(thoughtKey+"-thought"), title, open, portrait.DisclosureSpec{}, func() {
			markdown.View(c.Key(thoughtKey+"-body"), item.text, markdownOptions(c, &markdown.Options{WrapCode: true})).FillWidth()
		})
	case "notice":
		tone := portrait.Tones.Neutral
		if item.notify == "error" {
			tone = portrait.Tones.Danger
		} else if item.notify == "warning" {
			tone = portrait.Tones.Warning
		}
		portrait.Alert(c.Key(key), tone, "", item.text)
	case "plan":
		card(c.Key(key)).Gap(4).Padding(7, 9).Children(func() {
			text(c, portrait.Type.Meta, "Research plan").FontWeight(600).TextColor(portrait.Colors.Ink2)
			for _, step := range item.steps {
				icon, ink := portrait.IconCircle, portrait.Colors.Ink3
				tone := portrait.Tones.Neutral
				if step.Status == "completed" {
					icon, ink, tone = portrait.IconCheckCircle, portrait.Colors.Success, portrait.Tones.Success
				}
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					portrait.Glyph(c, icon, portrait.IconSizes.S, ink).Margin(1, 0, 0, 0)
					text(c, portrait.Type.Meta, step.Content).TextColor(portrait.Colors.Ink2).Grow(1).MinWidth(0).Wrap()
					portrait.Badge(c, strings.ReplaceAll(step.Status, "_", " "), tone)
				})
			}
		})
	case "tool":
		open := p.collapsibleState("tool-" + item.tool.ToolCallID)
		title := item.tool.Title
		if title == "" {
			title = item.tool.Kind
		}
		if title == "" {
			title = "Research tool"
		}
		if len(item.tool.Locations) > 0 {
			title += " · " + filepath.Base(item.tool.Locations[0].Path)
		}
		tone := portrait.Tones.Neutral
		switch item.tool.Status {
		case "completed":
			tone = portrait.Tones.Success
		case "failed", "error":
			tone = portrait.Tones.Danger
		case "pending", "in_progress":
			tone = portrait.Tones.Warning
		}
		portrait.Disclosure(c.Key(key+"-tool"), title, open, portrait.DisclosureSpec{Status: strings.ReplaceAll(item.tool.Status, "_", " "), Label: title + " · " + strings.ReplaceAll(item.tool.Status, "_", " "), Tone: tone}, func() {
			p.toolBody(c, item.tool)
		})
	}
}

func (p *panel) toolBody(c *ui.Context, tool feynman.ResearchTool) {
	ui.Column(c).Gap(3).Padding(5, 8).FillWidth().Children(func() {
		if tool.Input != "" {
			resultText(c, "Input", tool.Input, "")
		}
		if tool.Output != "" {
			resultText(c, "Result", tool.Output, "")
		}
		if tool.TerminalDelta != "" {
			resultText(c, "Command output", tool.TerminalDelta, "")
		}
		if tool.ExitCode != nil {
			text(c, portrait.Type.Meta, fmt.Sprintf("Exit %d · %d ms", *tool.ExitCode, tool.DurationMs)).FontFeatures("tnum").TextColor(portrait.Colors.Ink3)
		}
		for _, location := range tool.Locations {
			path := location.Path
			if strings.Contains(path, "/research/") {
				if i := strings.Index(path, "/work/"); i >= 0 {
					path = path[i+len("/work/"):]
				}
			}
			matched := findArtifact(p.state.artifacts, path, location.Path)
			if matched != nil {
				if smallButton(c, path, "Open research file "+path).AlignSelf(ui.Start).Clicked() {
					p.openArtifact(*matched)
				}
			} else {
				text(c, portrait.Type.CodeSmall, path+lineSuffix(location.Line)).TextColor(portrait.Colors.Ink3).FillWidth()
			}
		}
		for _, diff := range tool.Diffs {
			text(c, portrait.Type.CodeSmall, diff.Path).FontWeight(600).TextColor(portrait.Colors.Ink2)
			ui.Column(c).FillWidth().Padding(portrait.Form.HelpGap, portrait.Form.PadX).Background(portrait.Colors.DangerSoft).Children(func() {
				text(c, portrait.Type.CodeSmall, "− "+diff.OldText).FillWidth().Wrap().TextColor(portrait.Colors.Danger)
			})
			ui.Column(c).FillWidth().Padding(portrait.Form.HelpGap, portrait.Form.PadX).Background(portrait.Diff.Insert).Children(func() {
				text(c, portrait.Type.CodeSmall, "+ "+diff.NewText).FillWidth().Wrap().TextColor(portrait.Colors.Success)
			})
		}
		for _, image := range tool.Images {
			p.image(c, image)
		}
	})
}

func lineSuffix(line int) string {
	if line < 1 {
		return ""
	}
	return fmt.Sprintf(" · %d", line)
}

func (p *panel) image(c *ui.Context, source string) {
	if source == "" {
		return
	}
	image, err := p.loadDataImage(source)
	if err != nil {
		mutedText(c, "Image preview unavailable")
		return
	}
	ui.Image(c, image).FillWidth().MaxHeight(portrait.Form.ListMax).Fit(ui.Contain).Radius(portrait.Radii.M)
}

type decodedDataImage struct {
	source ui.ImageSource
	err    error
}

func (p *panel) loadDataImage(source string) (ui.ImageSource, error) {
	if p.decodedImages == nil {
		p.decodedImages = map[string]decodedDataImage{}
	}
	if cached, ok := p.decodedImages[source]; ok {
		return cached.source, cached.err
	}
	image, err := decodeDataImage(source)
	p.decodedImages[source] = decodedDataImage{source: image, err: err}
	return image, err
}

func decodeDataImage(source string) (ui.ImageSource, error) {
	comma := strings.IndexByte(source, ',')
	if comma < 0 || !strings.HasPrefix(strings.ToLower(source[:comma]), "data:image/") || !strings.Contains(source[:comma], ";base64") {
		return nil, fmt.Errorf("unsupported image data")
	}
	data, err := base64.StdEncoding.DecodeString(source[comma+1:])
	if err != nil || len(data) > 1<<20 {
		return nil, fmt.Errorf("invalid image data")
	}
	return ui.DecodeImage(data)
}

func (p *panel) composer(c *ui.Context) {
	s := p.state
	compact := compactResearchLayout(c)
	if s.queue.Steering != nil || s.queue.FollowUp != nil {
		if len(s.queue.Steering)+len(s.queue.FollowUp) > 0 {
			card(c.Key("research-queue")).Gap(2).Padding(4, 8).Children(func() {
				text(c, portrait.Type.Meta, fmt.Sprintf("Queue · %d steering · %d follow-up", len(s.queue.Steering), len(s.queue.FollowUp))).FontFeatures("tnum").TextColor(portrait.Colors.Ink3)
				for i, item := range s.queue.Steering {
					p.queueItem(c, "steer", i, item)
				}
				for i, item := range s.queue.FollowUp {
					p.queueItem(c, "follow-up", i, item)
				}
				if smallButton(c, "Clear queue", "Clear queued research messages").AlignSelf(ui.Start).Clicked() {
					p.runUpdate(func() (func(), error) {
						_, err := p.host.ClearQueue(p.window.Context(), s.root)
						return func() { s.queue = feynman.ResearchQueueState{} }, err
					})
				}
			})
		}
	}
	if s.failedDraft != "" || len(s.failedSources) > 0 {
		card(c.Key("research-failed-draft")).Gap(4).Padding(4, 8).Children(func() {
			text(c, portrait.Type.Meta, "Held draft:").TextColor(portrait.Colors.Ink2)
			summary := s.failedDraft
			if summary == "" {
				summary = fmt.Sprintf("%d attached source(s)", len(s.failedSources))
			}
			text(c, portrait.Type.Meta, summary).MaxLines(2).Ellipsis("…").TextColor(portrait.Colors.Ink)
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				composerHasInput := s.draft != "" || len(s.sources) > 0
				actionTitle := "Restore failed draft"
				if composerHasInput {
					actionTitle = "Swap with failed draft"
				}
				if smallButton(c, actionTitle, "Restore unsent prompt and sources").Clicked() {
					if composerHasInput {
						s.draft, s.failedDraft = s.failedDraft, s.draft
						s.sources, s.failedSources = s.failedSources, s.sources
					} else {
						s.draft = s.failedDraft
						s.sources = s.failedSources
						s.failedDraft = ""
						s.failedSources = nil
					}
				}
				if portrait.QuietIconButton(c, portrait.IconX, "Dismiss failed draft").Clicked() {
					s.failedDraft = ""
					s.failedSources = nil
				}
			})
		})
	}
	if s.suggested != "" {
		ui.Row(c.Key("research-suggestion")).Gap(4).AlignItems(ui.Center).Children(func() {
			text(c, portrait.Type.Meta, s.suggested).MaxLines(2).Ellipsis("…").Grow(1).MinWidth(0).TextColor(portrait.Colors.Ink2)
			if smallButton(c, "Insert", "Insert suggested prompt").Clicked() {
				if s.draft == "" {
					s.draft = s.suggested
				} else {
					s.draft += "\n" + s.suggested
				}
				s.suggested = ""
			}
			if portrait.QuietIconButton(c, portrait.IconX, "Dismiss suggested prompt").Clicked() {
				s.suggested = ""
			}
		})
	}
	paddingY, gap := float32(6), float32(5)
	if compact {
		paddingY, gap = 3, 3
	}
	ui.Column(c.Key("feynman-compose")).Children(func() {
		composer := ui.Column(c.Key("feynman-composer")).Padding(paddingY, 7).Gap(gap).Radius(portrait.Radii.L).Background(portrait.Colors.Paper).Border(1, portrait.Colors.LineStrong)
		if composer.FocusWithin() {
			portrait.Rings.Editing.Apply(composer)
		}
		composer.Children(func() {
			doc := p.activeResearchDocument()
			documentEligible := researchDocumentEligible(doc, s.root)
			if len(s.sources) > 0 || (s.includeDoc && documentEligible) {
				ui.Scroll(c.Key("research-attachments")).FillWidth().MaxHeight(portrait.Sizes.Regular + portrait.Sizes.Regular).Label("Attached research sources").Children(func() {
					ui.Row(c).Wrap().Gap(4).Children(func() {
						if s.includeDoc && documentEligible {
							label := doc.Name
							if label == "" {
								label = "Current document"
							}
							if chip(c, "research-document", portrait.Icon{}, "Document · "+label, "Remove active document from the research prompt", compact).Clicked() {
								s.includeDoc = false
							}
						}
						labels := sourceChipLabels(s.sources)
						for _, source := range s.sources {
							if chip(c, "research-source-"+source, portrait.IconPaperclip, labels[source], "Remove attached source "+source, compact).Clicked() {
								s.sources = without(s.sources, source)
							}
						}
					})
				})
			}
			promptHeight := researchPromptHeight(s.draft, compact)
			var draftEditor ui.Element
			bareArea(c, "research-draft", "Research prompt", &s.draft, promptPlaceholder(s.status), promptHeight, func(editor ui.Element) {
				draftEditor = editor
				editor.OnTextKey(func(key ui.TextKey) bool {
					if key.Mods != 0 {
						return false
					}
					if s.actionsOpen && p.actionsFromSlash {
						commands := p.filteredCommands(p.commandQuery)
						switch key.Key {
						case ui.KeyDown, ui.KeyUp:
							p.commandSelection.Move(key.Key, len(commands))
							return true
						case ui.KeyEnter:
							if len(commands) > 0 {
								p.commandSelection.Clamp(len(commands))
								p.chooseResearchCommand(commands[p.commandSelection.Index])
								key.SetText(s.draft)
							}
							return true
						}
					}
					if key.Key != ui.KeyEnter {
						return false
					}
					accepted := false
					if s.status == statusWorking {
						accepted = p.followUp()
					} else {
						accepted = p.send()
					}
					if accepted {
						key.SetText("")
					}
					return true
				})
			})
			if p.commandCaret > 0 {
				draftEditor.SetTextSelection(p.commandCaret, p.commandCaret)
				p.commandCaret = 0
			}
			if draftEditor.Changed() {
				if !strings.HasPrefix(s.draft, "/") {
					if p.actionsFromSlash {
						p.commandQuery = ""
					}
					p.actionsDismissed = false
					p.actionsFromSlash = false
					s.actionsOpen = false
				} else if p.actionsFromSlash && !strings.Contains(strings.TrimPrefix(s.draft, "/"), "/") {
					p.commandQuery = strings.TrimSpace(strings.TrimPrefix(s.draft, "/"))
					p.commandSelection.Reset()
					s.actionsOpen = true
				} else if !p.actionsDismissed && p.slashCommandMatches(s.draft) {
					p.commandQuery = strings.TrimSpace(strings.TrimPrefix(s.draft, "/"))
					p.commandSelection.Reset()
					p.actionsFromSlash = true
					s.actionsOpen = true
				} else if p.actionsFromSlash || strings.Contains(strings.TrimPrefix(s.draft, "/"), "/") {
					if p.actionsFromSlash {
						p.commandQuery = ""
					}
					p.actionsFromSlash = false
					s.actionsOpen = false
				}
			}
			ui.Row(c).Wrap().Gap(4).AlignItems(ui.Center).Children(func() {
				if portrait.ButtonWith(c, "", "Attach source files", portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Regular, Leading: portrait.IconPaperclip, Disabled: s.status == statusOffline || s.status == statusConnecting}).Clicked() {
					p.attachSources()
				}
				actionsSpec := researchActionsButtonSpec(s.actionsOpen)
				actionsSpec.Disabled = len(s.commands) == 0
				actionsBtn := portrait.ButtonWith(c, "Actions", "Browse workflows, skills and extensions", actionsSpec).Expanded(s.actionsOpen)
				if actionsBtn.Clicked() {
					s.actionsOpen = !s.actionsOpen
					p.commandSelection.Reset()
					p.actionsDismissed = false
					p.actionsFromSlash = false
					p.actionsFocusFilter = s.actionsOpen
				}
				actionsPopover := portrait.PopoverWithSpec(c, actionsBtn, &s.actionsOpen, portrait.PopoverSpec{Width: portrait.Form.SelectMax, Title: "Research actions", Placement: ui.AnchorTopStart}, func() {
					p.commandList(c)
				})
				actionsPopover.AttachTo(composer, ui.AnchorTopStart, ui.AnchorBottomStart).Label("Research actions")
				if actionsPopover.Dismissed() {
					p.actionsFocusFilter = false
					if p.actionsFromSlash {
						p.actionsDismissed = true
						p.actionsFromSlash = false
					} else {
						actionsBtn.Focus()
					}
				}
				if documentEligible {
					includeDocumentButton(c, &s.includeDoc)
				}
				ui.Row(c).Grow(1).Justify(ui.End).Gap(4).Children(func() {
					if s.status == statusWorking {
						emptyDraft := strings.TrimSpace(s.draft) == ""
						if portrait.ButtonWith(c, "Steer", "Send a steering message", portrait.ButtonSpec{Look: portrait.Raised, Size: portrait.Sizes.Regular, Disabled: emptyDraft}).Clicked() {
							p.steer()
						}
						if portrait.ButtonWith(c, "Follow up", "Queue a follow-up message", portrait.ButtonSpec{Look: portrait.Raised, Size: portrait.Sizes.Regular, Disabled: emptyDraft}).Clicked() {
							p.followUp()
						}
						if portrait.ButtonWith(c, "Stop", "Stop the current research action", portrait.ButtonSpec{Look: portrait.DangerOutline, Size: portrait.Sizes.Regular, Leading: portrait.IconStop}).Clicked() {
							p.run(func() error { return p.host.Abort(p.window.Context(), s.root) })
						}
					} else {
						if portrait.ButtonWith(c, "Send", "Send research prompt", portrait.ButtonSpec{Look: portrait.Accent, Size: portrait.Sizes.Regular, Leading: portrait.IconPaperPlaneRight, Disabled: s.status != statusReady || strings.TrimSpace(s.draft) == ""}).Clicked() {
							p.send()
						}
					}
				})
			})
			p.configControls(c)
		})
	})
}

func (p *panel) queueItem(c *ui.Context, kind string, index int, item string) {
	label := kind + " · " + item
	if len(item) <= 80 && !strings.Contains(item, "\n") {
		text(c, portrait.Type.Meta, label).TextColor(portrait.Colors.Ink2).FillWidth().Wrap()
		return
	}
	key := fmt.Sprintf("research-queue-%s-%d", kind, index)
	portrait.Disclosure(c.Key(key), label, p.collapsibleState(key), portrait.DisclosureSpec{Label: "Inspect queued " + kind + " message"}, func() {
		text(c, portrait.Type.Meta, item).TextColor(portrait.Colors.Ink2).FillWidth().Wrap()
	})
}

func researchPromptHeight(draft string, compact bool) float32 {
	lines := min(float32(strings.Count(draft, "\n")+1), 6)
	if compact && lines == 1 {
		return portrait.Form.AreaCompact
	}
	contentHeight := lines*portrait.Type.UI.Leading + 2*portrait.Form.AreaPadY
	if compact {
		return max(portrait.Form.AreaCompact, contentHeight)
	}
	return max(portrait.Form.AreaMin, contentHeight)
}

func sourceChipLabels(sources []string) map[string]string {
	basenames := make(map[string]int, len(sources))
	for _, source := range sources {
		basenames[filepath.Base(source)]++
	}
	labels := make(map[string]string, len(sources))
	for _, source := range sources {
		base := filepath.Base(source)
		labels[source] = base
		if basenames[base] > 1 {
			labels[source] = filepath.Join(filepath.Base(filepath.Dir(source)), base)
		}
	}
	return labels
}

func (p *panel) slashCommandMatches(draft string) bool {
	if !strings.HasPrefix(draft, "/") || strings.Contains(strings.TrimPrefix(draft, "/"), "/") {
		return false
	}
	query := strings.TrimSpace(strings.TrimPrefix(draft, "/"))
	return p.matchingCommand(query) != nil
}

func researchActionsButtonSpec(open bool) portrait.ButtonSpec {
	return portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Regular, Trailing: portrait.IconCaretUp, Selected: open}
}

func includeDocumentButton(c *ui.Context, include *bool) ui.Element {
	button := portrait.ButtonWith(c.Key("research-include-document"), "Include doc", "Include doc", portrait.ButtonSpec{
		Look: portrait.Quiet, Size: portrait.Sizes.Regular, Leading: portrait.IconFileText, Selected: *include,
	}).Checked(*include)
	if button.Clicked() {
		*include = !*include
	}
	return button
}

func promptPlaceholder(status connectionStatus) string {
	if status == statusWorking {
		return "Steer or queue a follow-up…"
	}
	return "Ask Feynman…"
}

func without(items []string, target string) []string {
	result := items[:0]
	for _, item := range items {
		if item != target {
			result = append(result, item)
		}
	}
	return result
}

func (p *panel) commandList(c *ui.Context) {
	query := p.commandQuery
	var filterInputEditor ui.Element
	filterInput(c, "research-actions-filter", "Filter research actions", &p.commandQuery, "Search workflows and skills…", func(editor ui.Element) {
		filterInputEditor = editor
	})
	if p.actionsFocusFilter {
		filterInputEditor.Focus()
		p.actionsFocusFilter = false
	}
	if filterInputEditor.Changed() {
		query = p.commandQuery
		p.commandSelection.Reset()
		p.actionsFromSlash = false
	}
	commands := p.filteredCommands(query)
	if p.commandSelection.Shortcuts(filterInputEditor, len(commands)) {
		p.chooseResearchCommand(commands[p.commandSelection.Index])
	}
	ui.Scroll(c.Key("research-actions-list")).MaxHeight(portrait.Form.ListMax).FillWidth().Role(ui.RoleListBox).Label("Research actions").Children(func() {
		for i, command := range commands {
			selected := i == p.commandSelection.Index
			// The name stays whole and the description takes its own line, so
			// a short panel never cuts an action to an ellipsis.
			spec := portrait.ListRowSpec{
				Title: "/" + command.Name, Detail: command.Description, Label: "Choose research action " + command.Name, Selected: selected,
			}
			if command.Source != "" {
				source := command.Source
				spec.Trailing = func() { portrait.Caption(c, source).Shrink(0) }
			}
			row := portrait.ListRow(c.Key("research-action-"+command.Name), spec).Role(ui.RoleOption).TabStop(false)
			if selected {
				row.Background(portrait.Colors.AccentSoft)
				if p.commandSelection.Moved {
					row.ScrollIntoView()
				}
			}
			if row.Clicked() {
				p.chooseResearchCommand(command)
			}
		}
		p.commandSelection.Moved = false
		if len(commands) == 0 {
			mutedText(c, "No matching workflows, skills or extensions.")
		}
	})
}

func (p *panel) filteredCommands(query string) []feynman.ResearchCommand {
	needle := strings.ToLower(strings.TrimSpace(query))
	var commands []feynman.ResearchCommand
	for _, command := range p.state.commands {
		search := strings.ToLower(command.Name + " " + command.Description + " " + command.Source)
		if needle == "" || strings.Contains(search, needle) {
			commands = append(commands, command)
		}
	}
	return commands
}

func (p *panel) chooseResearchCommand(command feynman.ResearchCommand) {
	p.state.draft = "/" + command.Name + " "
	p.commandCaret = len([]rune(p.state.draft))
	p.state.actionsOpen = false
	p.commandQuery = ""
	p.commandSelection.Reset()
	p.actionsDismissed = false
	p.actionsFromSlash = false
}

func (p *panel) matchingCommand(query string) *feynman.ResearchCommand {
	for i := range p.state.commands {
		command := &p.state.commands[i]
		if strings.EqualFold(command.Name, query) || strings.HasPrefix(strings.ToLower(command.Name), strings.ToLower(query)) {
			return command
		}
	}
	return nil
}

func (p *panel) messageLink(address string) {
	if strings.HasPrefix(address, "#") {
		return
	}
	u, err := url.Parse(address)
	if err == nil && (u.Scheme == "https" || u.Scheme == "http") {
		p.run(func() error { return p.host.OpenCitation(p.window.Context(), p.state.root, address) })
		return
	}
	path := strings.SplitN(address, "#", 2)[0]
	path = strings.TrimPrefix(path, "./")
	if file := findArtifact(p.state.artifacts, filepath.Clean(path), path); file != nil {
		p.openArtifact(*file)
		return
	}
	p.state.error = "This link does not point to a published research file or web source."
}

func (p *panel) followLink(address string) { p.messageLink(address) }

func findArtifact(files []review.ResearchArtifact, candidates ...string) *review.ResearchArtifact {
	for i := range files {
		for _, candidate := range candidates {
			if candidate != "" && (files[i].Path == candidate || filepath.Clean(files[i].Path) == filepath.Clean(candidate) || files[i].Target == candidate) {
				return &files[i]
			}
		}
	}
	return nil
}

func (p *panel) question(c *ui.Context, question feynman.ResearchUpdate) {
	if p.permissionID != question.RequestID {
		p.permissionID = question.RequestID
		p.questionText, p.questionPick, p.questionFind = question.Prefill, "", ""
	}
	title := question.Text
	if title == "" {
		title = question.Title
	}
	if title == "" {
		title = "Choose how Feynman should proceed."
	}
	textEntry := question.Kind == "input" || question.Kind == "editor"
	confirm := question.Kind == "confirm"
	hostConfirm := confirm && len(question.Options) == 2 && question.Options[0].ID == "yes" && question.Options[0].Kind == "confirm" && question.Options[1].ID == "no" && question.Options[1].Kind == "cancel"
	toolList := title == "Tools" && len(question.Options) > 0
	for _, option := range question.Options {
		toolList = toolList && strings.Contains(option.Name, " — ")
	}
	tone, icon := portrait.Tones.Accent, portrait.IconInfo
	if confirm {
		tone, icon = portrait.Tones.Warning, portrait.IconWarning
	}
	portrait.Card(c.Key("research-question")).Role(ui.RoleGroup).Label("Feynman confirmation").Padding(portrait.Form.AreaPadY, portrait.Form.PadX).Margin(0, 0, portrait.Form.Gap, 0).Gap(portrait.Form.Gap).Radius(portrait.Radii.L).
		Border(portrait.Editor.Hairline, portrait.Tones.Neutral.Border).Children(func() {
		ui.Row(c).Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
			portrait.Glyph(c, icon, portrait.IconSizes.M, tone.Ink).Shrink(0)
			text(c, portrait.Type.UI, title).FontWeight(600).TextColor(portrait.Colors.Ink).FillWidth().Wrap()
		})
		if question.Message != "" && question.Message != title && !strings.Contains(title, question.Message) {
			text(c, portrait.Type.Meta, question.Message).TextColor(portrait.Colors.Ink2).FillWidth().Wrap()
		}
		if textEntry {
			if question.Kind == "editor" {
				area(c, "research-question-answer", title, &p.questionText, question.Placeholder, 78)
			} else {
				input(c, "research-question-answer", title, &p.questionText, question.Placeholder)
			}
			ui.Row(c).Justify(ui.End).Gap(4).Children(func() {
				if portrait.ButtonWith(c, "Cancel", "Cancel this Feynman request", portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Regular}).Disabled(p.questionPending).Clicked() {
					p.answerQuestion("", true)
				}
				if portrait.ButtonWith(c, "Submit", "Submit answer to Feynman", portrait.ButtonSpec{Look: portrait.Accent, Size: portrait.Sizes.Regular}).Disabled(p.questionPending).Clicked() {
					p.answerQuestion(p.questionText, false)
				}
			})
			return
		}
		options := question.Options
		filterable := len(options) > 6
		if filterable {
			filterInput(c, "research-question-filter", "Filter choices", &p.questionFind, "Find a choice…")
		}
		if confirm {
			ui.Row(c).Justify(ui.End).Gap(4).Children(func() {
				if hostConfirm {
					decline, accept := options[1], options[0]
					if portrait.ButtonWith(c, decline.Name, "Answer "+decline.Name, portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Regular}).KeyShortcuts(0, ui.KeyEscape).Disabled(p.questionPending).Clicked() {
						p.answerQuestion(decline.ID, false)
					}
					if portrait.ButtonWith(c, accept.Name, "Answer "+accept.Name, portrait.ButtonSpec{Look: portrait.Accent, Size: portrait.Sizes.Regular}).Disabled(p.questionPending).Clicked() {
						p.answerQuestion(accept.ID, false)
					}
					return
				}
				if portrait.ButtonWith(c, "Cancel", "Cancel confirmation", portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Regular}).Disabled(p.questionPending).Clicked() {
					p.answerQuestion("", true)
				}
				for i, option := range options {
					look := portrait.Raised
					if i == len(options)-1 {
						look = portrait.Accent
					}
					if portrait.ButtonWith(c, option.Name, "Answer "+option.Name, portrait.ButtonSpec{Look: look, Size: portrait.Sizes.Regular}).Disabled(p.questionPending).Clicked() {
						p.answerQuestion(option.ID, false)
					}
				}
			})
			return
		}
		visibleOptions := make([]feynman.ResearchOption, 0, len(options))
		selectedVisible := false
		for _, option := range options {
			if filterable && !strings.Contains(strings.ToLower(option.Name), strings.ToLower(strings.TrimSpace(p.questionFind))) {
				continue
			}
			visibleOptions = append(visibleOptions, option)
			selectedVisible = selectedVisible || option.ID == p.questionPick
		}
		ui.Scroll(c.Key("research-question-options")).MaxHeight(portrait.Form.ListMax).Label("Feynman choices").Children(func() {
			ui.Column(c).Gap(portrait.Form.PopoverGap).Children(func() {
				for _, option := range visibleOptions {
					portrait.CompactRadio(c.Key("research-question-"+option.ID), &p.questionPick, option.ID, option.Name).Disabled(p.questionPending).FillWidth()
				}
			})
		})
		if len(visibleOptions) == 0 {
			mutedText(c, "No matching options")
		}
		ui.Row(c).Justify(ui.End).Gap(4).Children(func() {
			if portrait.ButtonWith(c, "Cancel", "Cancel this Feynman request", portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Regular}).Disabled(p.questionPending).Clicked() {
				p.answerQuestion("", true)
			}
			if toolList && portrait.ButtonWith(c, "Use in question", "Insert the selected tool into the Feynman prompt", portrait.ButtonSpec{Look: portrait.Raised, Size: portrait.Sizes.Regular}).Disabled(!selectedVisible || p.questionPending).Clicked() {
				p.answerQuestion(p.questionPick, false, true)
			}
			if portrait.ButtonWith(c, "Confirm", "Confirm selected choice", portrait.ButtonSpec{Look: portrait.Accent, Size: portrait.Sizes.Regular}).Disabled(!selectedVisible || p.questionPending).Clicked() {
				p.answerQuestion(p.questionPick, false)
			}
		})
	})
}

func (p *panel) sessions(c *ui.Context) {
	s := p.state
	ui.Column(c.Key("feynman-session-history")).FillWidth().Gap(portrait.Form.Gap).Children(func() {
		title := "Conversations"
		if p.sessionView == "fork" {
			title = "Fork from a message"
		} else if p.sessionView == "tree" {
			title = "Conversation tree"
		} else if p.sessionView == "rename" {
			title = "Rename conversation"
		}
		ui.Row(c).Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
			if p.sessionView != "" && portrait.ButtonWith(c, "Back", "Return to conversation history", portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Compact, Leading: portrait.IconCaretLeft}).Clicked() {
				p.sessionView = ""
			}
			sectionTitle(c, title).Grow(1).MinWidth(0).MaxLines(1).Ellipsis("…")
			trigger := portrait.QuietIconButton(c.Key("conversation-actions"), portrait.IconDotsThree, "Conversation actions").Expanded(p.sessionMenu)
			trigger.PopoverMenuButton(func(menu *ui.Menu) {
				menu.MaxWidth(portrait.Form.MenuMax)
				if conversationActionItem(menu, portrait.MenuItemSpec{Label: "New", Icon: portrait.IconPlus, Disabled: s.status != statusReady}).Chosen() {
					p.newSession()
				}
				if conversationActionItem(menu, portrait.MenuItemSpec{Label: "Rename", Icon: portrait.IconPencil}).Chosen() {
					p.sessionView = "rename"
				}
				if conversationActionItem(menu, portrait.MenuItemSpec{Label: "Clone", Disabled: s.status != statusReady || s.draft != ""}).Chosen() {
					p.cloneSession()
				}
				if conversationActionItem(menu, portrait.MenuItemSpec{Label: "Compact", Disabled: s.status != statusReady}).Chosen() {
					p.compactSession("")
				}
				if conversationActionItem(menu, portrait.MenuItemSpec{Label: "Fork", Disabled: s.status != statusReady || s.draft != ""}).Chosen() {
					p.loadForkMessages()
				}
				if conversationActionItem(menu, portrait.MenuItemSpec{Label: "Tree"}).Chosen() {
					p.loadConversationTree()
				}
			}, ui.PopoverMenuButtonOptions{
				At: ui.AnchorBottomEnd, Self: ui.AnchorTopEnd, Open: &p.sessionMenu,
				Panel: func(panel ui.Element) {
					panel.MinWidth(portrait.Form.MenuMin).MaxWidth(portrait.Form.MenuMax).Padding(portrait.Form.MenuPad).Gap(portrait.Form.PopoverGap).Radius(portrait.Radii.L)
					portrait.Materials.GlassOverlay.Apply(panel)
				},
				Item: func(item ui.Element) {
					item.Height(portrait.Form.MenuRow).Padding(0, portrait.Form.PadX-portrait.Form.PopoverGap).Gap(portrait.Form.Gap).Radius(portrait.Form.MenuRadius).AlignItems(ui.Center)
				},
			})
		})
		if p.sessionView == "" || p.sessionView == "rename" {
			if p.sessionView == "rename" {
				ui.Row(c).Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
					input(c, "feynman-session-name", "Conversation name", &s.renameDraft, "Rename conversation").Grow(1)
					if portrait.Button(c, "Save name", "Rename this conversation").Disabled(strings.TrimSpace(s.renameDraft) == "").Clicked() {
						p.renameSession()
						p.sessionView = ""
					}
				})
			}
			filterInput(c, "feynman-session-filter", "Filter conversations", &s.sessionFilter, "Find a conversation…")
			ui.Column(c.Key("feynman-sessions-list")).FillWidth().Label("Workspace conversations").Children(func() {
				for _, row := range s.sessionRows {
					row := row
					title := row.Title
					if title == "" {
						title = "Untitled conversation"
					}
					if !strings.Contains(strings.ToLower(title), strings.ToLower(s.sessionFilter)) {
						continue
					}
					var mark portrait.Icon
					if row.Current {
						mark = portrait.IconCheck
					}
					container := ui.Row(c).FillWidth().Gap(portrait.Form.PopoverGap).AlignItems(ui.Center)
					container.Children(func() {
						choice := discreteButton(c, title, row.Preview, mark).Grow(1).Disabled(s.status != statusReady || s.draft != "" || len(s.sources) > 0)
						if row.Current {
							choice.Background(portrait.Colors.AccentSoft).Checked(true)
						}
						if choice.Clicked() {
							p.selectSession(row.ID)
						}
						if !row.Current {
							trigger := portrait.ButtonWith(c, "", "Delete conversation "+title, portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Compact, Leading: portrait.IconTrash})
							portrait.RevealFor(c, trigger, container.Hovered() || trigger.FocusVisible())
							p.confirmOnClick("delete-conversation:"+row.ID, trigger, portrait.ConfirmSpec{
								Title: "Delete conversation?", Body: "Delete “" + title + "”? This action cannot be undone.", ConfirmLabel: "Delete",
								OnConfirm: func() { p.deleteSession(row.ID) },
							})
						}
					})
				}
				if len(s.sessionRows) == 0 {
					portrait.Empty(c, portrait.EmptySpec{Title: "No saved conversations", Description: "No saved conversations for this workspace."})
				}
			})
		} else if p.sessionView == "fork" {
			ui.Column(c.Key("feynman-fork-messages")).FillWidth().Label("Messages available for a conversation fork").Children(func() {
				for _, item := range p.forkMessages {
					label := item.Text
					if len(label) > 180 {
						label = label[:180] + "…"
					}
					if discreteButton(c, label, "Fork conversation from this message", portrait.Icon{}).Disabled(s.status != statusReady || s.draft != "").Clicked() {
						p.forkFrom(item.EntryID)
					}
				}
				if len(p.forkMessages) == 0 {
					mutedText(c, "No messages are available for forking.")
				}
			})
		} else {
			ui.Column(c.Key("feynman-session-tree")).FillWidth().Label("Conversation tree").Children(func() {
				if p.researchTree == nil {
					mutedText(c, "Loading conversation tree…")
				} else {
					for _, node := range p.researchTree.Tree {
						p.treeNode(c, node, 0)
					}
				}
			})
		}
	})
}

func conversationActionItem(menu *ui.Menu, spec portrait.MenuItemSpec) *ui.MenuItem {
	item := menu.Item(spec.Label).Disabled(spec.Disabled)
	if spec.Danger {
		item.Danger()
	}
	item.Content(func(content ui.MenuItemContent) {
		ui.Row(content.Context).FillWidth().Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
			if spec.Icon.Glyph != "" {
				portrait.Glyph(content.Context, spec.Icon, portrait.IconSizes.M, content.Text).Shrink(0)
			}
			ui.Column(content.Context).Grow(1).MinWidth(0).Children(func() {
				text(content.Context, portrait.Type.UI, spec.Label).TextColor(content.Text).MaxLines(1).Ellipsis("…")
				if spec.Detail != "" {
					text(content.Context, portrait.Type.Meta, spec.Detail).TextColor(content.Muted).MaxLines(1).Ellipsis("…")
				}
			})
		})
	})
	return item
}

func (p *panel) loadForkMessages() {
	root := p.state.root
	p.runUpdate(func() (func(), error) {
		rows, err := p.host.ForkMessages(p.window.Context(), root)
		return func() { p.forkMessages = rows; p.sessionView = "fork" }, err
	})
}

func (p *panel) loadConversationTree() {
	root := p.state.root
	p.sessionView = "tree"
	p.researchTree = nil
	p.runUpdate(func() (func(), error) {
		tree, err := p.host.Tree(p.window.Context(), root)
		return func() { p.researchTree = &tree }, err
	})
}

func (p *panel) forkFrom(entryID string) {
	root := p.state.root
	p.runUpdate(func() (func(), error) {
		result, err := p.host.Fork(p.window.Context(), root, entryID)
		return func() {
			if !result.Cancelled && result.Text != "" && strings.TrimSpace(p.state.draft) == "" {
				p.state.draft = result.Text
			}
			p.sessionView = ""
			p.loadSessions(root)
		}, err
	})
}

func (p *panel) treeNode(c *ui.Context, node feynman.ResearchTreeNode, depth int) {
	label := node.Entry.Role
	if node.Label != "" {
		label = node.Label
	}
	if node.Entry.Text != "" {
		label = node.Entry.Text
	}
	if len(label) > 100 {
		label = label[:100] + "…"
	}
	current := node.Entry.ID == p.researchTree.LeafID
	row := ui.Row(c).Gap(portrait.Form.Gap).Padding(portrait.Form.HelpGap, portrait.Form.Gap).Margin(0, 0, 0, float32(depth)*portrait.IconSizes.XS).Radius(portrait.Radii.M).AlignItems(ui.Center).FillWidth()
	if current {
		row.Background(portrait.Colors.AccentSoft)
	}
	row.Children(func() {
		if current {
			portrait.Glyph(c, portrait.IconCheck, portrait.IconSizes.XS, portrait.Colors.Accent)
		}
		text(c, portrait.Type.Meta, label).TextColor(portrait.Colors.Ink).Grow(1).MinWidth(0).Wrap()
	})
	for _, child := range node.Children {
		p.treeNode(c, child, depth+1)
	}
}

var _ = workspace.Snapshot{}
