package nativefeynman

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

func (p *panel) chat(c *ui.Context) {
	s := p.state
	p.configControls(c)
	if len(s.statuses) > 0 {
		ui.Row(c).Wrap().Gap(4).Children(func() {
			for key, value := range s.statuses {
				text(c, portrait.Type.Meta, key+": "+value).TextColor(colors.Ink3).Padding(3, 7).Radius(999).Background(colors.Fill)
			}
		})
	}
	if s.progress != "" {
		ui.Row(c.Key("research-progress")).Role(ui.RoleStatus).Label(s.progress).Gap(6).Padding(3, 8).AlignItems(ui.Center).Children(func() {
			if s.status == statusWorking {
				portrait.Spinner(c, "Research in progress")
			}
			text(c, portrait.Type.Meta, s.progress).TextColor(colors.Ink3).Grow(1).MinWidth(0)
		})
	}
	if len(s.widgets) > 0 {
		card(ui.Column(c)).Gap(3).Padding(7, 9).Children(func() {
			for key, widget := range s.widgets {
				for _, line := range widget.WidgetLines {
					text(c, portrait.Type.Meta, line).TextColor(colors.Ink2).Label(key)
				}
			}
		})
	}
	ui.Scroll(c.Key("feynman-chat-scroll")).Grow(1).FillWidth().MinHeight(0).Label("Feynman conversation").TrackScroll(&p.chatScroll).Children(func() {
		if len(s.messages) == 0 && s.status == statusReady {
			text(c, portrait.Type.UI, "Ask Feynman to research this workspace.").TextColor(colors.Ink3).Padding(12, 8)
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

func (p *panel) configControls(c *ui.Context) {
	s := p.state
	var model *feynman.ResearchModel
	if s.session.Model != nil {
		model = s.session.Model
	}
	if len(s.models) == 0 && len(s.thinking) == 0 && len(s.configs) == 0 && !s.hasStats {
		return
	}
	if compactResearchLayout(c) {
		p.compactConfigControls(c, model)
		return
	}
	card(ui.Column(c.Key("research-configs"))).Gap(4).Padding(6, 7).Children(func() {
		if model != nil {
			label := model.Name
			if label == "" {
				label = model.ID
			}
			portrait.Tooltip(c, text(c, portrait.Type.Meta, "Model · "+label).FontWeight(600).TextColor(colors.Ink2), model.Provider+"/"+model.ID)
		}
		p.configChoices(c)
	})
}

func (p *panel) compactConfigControls(c *ui.Context, model *feynman.ResearchModel) {
	modelName := "Choose model"
	if model != nil {
		modelName = model.Name
		if modelName == "" {
			modelName = model.ID
		}
		modelName = "Model · " + modelName
	}
	thinking := p.state.thinkingLevel
	if thinking == "" {
		thinking = "default"
	}
	open := p.collapsibleState("compact-research-config")
	width, height := c.Size()
	card(ui.Row(c.Key("research-compact-config-row"))).Gap(5).Padding(3, 5).AlignItems(ui.Center).Children(func() {
		button := pill(c, "research-compact-config", modelName, "Research model and thinking settings", portrait.Quiet, false).Description("Choose a research model and thinking level").Grow(1).Justify(ui.Start)
		if button.Clicked() {
			*open = !*open
		}
		mutedText(c, "Thinking · "+thinking)
		portrait.Popover(c, button, open, func() {
			popoverWidth := min(float32(352), max(float32(280), width-112))
			maxHeight := max(float32(128), height-180)
			ui.Column(c).Width(popoverWidth).Children(func() {
				ui.Scroll(c.Key("research-config-popover-scroll")).MaxHeight(maxHeight).FillWidth().Label("Research model and thinking settings").Children(func() {
					ui.Column(c).Gap(4).Padding(7).Children(func() { p.configChoices(c) })
				})
			})
		})
	})
}

func (p *panel) configChoices(c *ui.Context) {
	s := p.state
	if len(s.models) > 0 {
		ui.Row(c).Wrap().Gap(3).Children(func() {
			for _, option := range s.models {
				value := option.Name
				if value == "" {
					value = option.ID
				}
				current := s.session.Model != nil && s.session.Model.Provider == option.Provider && s.session.Model.ID == option.ID
				if choiceButton(c, "research-model-"+option.Provider+"/"+option.ID, value, "Use "+option.Provider+" model "+option.ID, current, false).Clicked() && s.status == statusReady {
					p.runUpdate(func() (func(), error) {
						got, err := p.host.SetModel(p.window.Context(), s.root, option.Provider, option.ID)
						return func() { s.session.Model = &got }, err
					})
				}
			}
		})
	}
	if len(s.thinking) > 0 {
		ui.Row(c).Wrap().Gap(3).AlignItems(ui.Center).Children(func() {
			mutedText(c, "Thinking")
			for _, level := range s.thinking {
				if choiceButton(c, "research-thinking-"+level, level, "Set thinking level to "+level, level == s.thinkingLevel, false).Clicked() && s.status == statusReady {
					p.runUpdate(func() (func(), error) {
						err := p.host.SetThinkingLevel(p.window.Context(), s.root, level)
						return func() { s.thinkingLevel = level }, err
					})
				}
			}
		})
	}
	if s.hasStats {
		label := fmt.Sprintf("%d tokens · $%.4f", s.stats.Tokens.Total, s.stats.Cost)
		if s.stats.ContextUsage != nil && s.stats.ContextUsage.ContextWindow > 0 {
			label = fmt.Sprintf("%d / %d tokens · $%.4f", s.stats.ContextUsage.Tokens, s.stats.ContextUsage.ContextWindow, s.stats.Cost)
		}
		mutedText(c, label)
	}
}

func (p *panel) message(c *ui.Context, index int, item message, streaming bool) {
	key := fmt.Sprintf("message-%d", index)
	switch item.kind {
	case "user":
		ui.Column(c.Key(key)).AlignItems(ui.End).Padding(3, 8).Children(func() {
			text(c, portrait.Type.Meta, "You").FontWeight(600).TextColor(colors.Ink3)
			text(c, portrait.Type.UI, item.text).FillWidth().TextColor(colors.Ink).Padding(9, 11).Radius(12).Background(colors.AccentSoft)
			p.image(c, item.image)
		})
	case "assistant":
		ui.Column(c.Key(key)).Gap(3).Padding(4, 8).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				portrait.Glyph(c, portrait.IconAtom, 15, colors.Accent)
				text(c, portrait.Type.Meta, "Feynman").FontWeight(600).TextColor(colors.Ink3)
			})
			options := markdownOptions(&markdown.Options{
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
			if smallButton(c, "Copy", "Copy Feynman response").AlignSelf(ui.Start).Clicked() {
				mygo.Clipboard.WriteText(item.text)
			}
		})
	case "thought":
		open := p.collapsibleState(key + "-thought")
		ui.Collapsible(c.Key(key+"-thought"), "Thinking", open, func() {
			markdown.View(c.Key(key+"-body"), item.text, markdownOptions(&markdown.Options{WrapCode: true})).FillWidth()
		}).FillWidth()
	case "notice":
		ink, fill := colors.Ink2, colors.Fill
		if item.notify == "error" {
			ink, fill = colors.Danger, colors.DangerSoft
		} else if item.notify == "warning" {
			ink, fill = colors.Warn, colors.WarnSoft
		}
		portrait.Type.Meta.Apply(ui.Text(c.Key(key), item.text)).Role(ui.RoleStatus).FillWidth().Wrap().TextColor(ink).Padding(6, 9).Radius(8).Background(fill)
	case "plan":
		card(ui.Column(c.Key(key))).Gap(4).Padding(7, 9).Children(func() {
			text(c, portrait.Type.Meta, "Research plan").FontWeight(600).TextColor(colors.Ink2)
			for _, step := range item.steps {
				icon, ink := portrait.IconCircle, colors.Ink3
				if step.Status == "completed" {
					icon, ink = portrait.IconCheckCircle, colors.Accent
				}
				ui.Row(c).Gap(6).AlignItems(ui.Start).Children(func() {
					portrait.Glyph(c, icon, 13, ink).Margin(1, 0, 0, 0)
					text(c, portrait.Type.Meta, step.Content+" · "+step.Status).TextColor(colors.Ink3).Grow(1).MinWidth(0).Wrap()
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
		ui.Collapsible(c.Key(key+"-tool"), title+" · "+strings.ReplaceAll(item.tool.Status, "_", " "), open, func() {
			p.toolBody(c, item.tool)
		}).FillWidth()
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
			mutedText(c, fmt.Sprintf("Exit %d · %d ms", *tool.ExitCode, tool.DurationMs))
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
				if smallButton(c, path, "Open research file "+path).Clicked() {
					p.openArtifact(*matched)
				}
			} else {
				text(c, portrait.Type.Meta.In(portrait.FontMono), path+lineSuffix(location.Line)).TextColor(colors.Ink3).FillWidth()
			}
		}
		for _, diff := range tool.Diffs {
			sectionTitle(c, diff.Path)
			text(c, portrait.Type.Meta, "Before").FontWeight(600).TextColor(colors.Ink2)
			text(c, portrait.Type.Meta.In(portrait.FontMono), diff.OldText).FillWidth().Wrap()
			text(c, portrait.Type.Meta, "After").FontWeight(600).TextColor(colors.Ink2)
			text(c, portrait.Type.Meta.In(portrait.FontMono), diff.NewText).FillWidth().Wrap()
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
	ui.Image(c, image).FillWidth().MaxHeight(220).Fit(ui.Contain).Radius(6)
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
			card(ui.Column(c.Key("research-queue"))).Gap(2).Padding(4, 8).Children(func() {
				mutedText(c, fmt.Sprintf("Queue · %d steering · %d follow-up", len(s.queue.Steering), len(s.queue.FollowUp)))
				for _, item := range s.queue.Steering {
					text(c, portrait.Type.Meta, "Steer · "+item).MaxLines(2).Ellipsis("…").TextColor(colors.Ink2).FillWidth()
				}
				for _, item := range s.queue.FollowUp {
					text(c, portrait.Type.Meta, "Follow-up · "+item).MaxLines(2).Ellipsis("…").TextColor(colors.Ink2).FillWidth()
				}
				if smallButton(c, "Clear queue", "Clear queued research messages").Clicked() {
					p.runUpdate(func() (func(), error) {
						queue, err := p.host.ClearQueue(p.window.Context(), s.root)
						return func() { s.queue = queue }, err
					})
				}
			})
		}
	}
	if s.suggested != "" {
		ui.Row(c.Key("research-suggestion")).Gap(4).AlignItems(ui.Center).Children(func() {
			text(c, portrait.Type.Meta, s.suggested).MaxLines(2).Ellipsis("…").Grow(1).MinWidth(0).TextColor(colors.Ink2)
			if smallButton(c, "Insert", "Insert suggested prompt").Clicked() {
				s.draft = strings.TrimSpace(strings.TrimSpace(s.draft) + "\n" + s.suggested)
				s.suggested = ""
			}
			if iconButton(c, portrait.IconX, "Dismiss suggested prompt").Clicked() {
				s.suggested = ""
			}
		})
	}
	paddingY, gap := float32(6), float32(5)
	if compact {
		paddingY, gap = 3, 3
	}
	composer := ui.Column(c.Key("feynman-composer")).Padding(paddingY, 7).Gap(gap).Radius(12).Background(colors.Paper).Border(1, colors.LineStrong)
	if composer.FocusWithin() {
		portrait.Rings.Editing.Apply(composer)
	}
	composer.Children(func() {
		if len(s.sources) > 0 || s.includeDoc {
			ui.Row(c).Wrap().Gap(4).Children(func() {
				if s.includeDoc {
					doc := p.window.Active()
					label := doc.Name
					if label == "" {
						label = "Current document"
					}
					if chip(c, "research-document", portrait.Icon{}, "Document · "+label, "Remove active document from the research prompt", compact).Clicked() {
						s.includeDoc = false
					}
				}
				for _, source := range s.sources {
					path := source
					if i := strings.LastIndex(path, "/"); i >= 0 {
						path = path[i+1:]
					}
					if chip(c, "research-source-"+source, portrait.IconPaperclip, path, "Remove attached source "+source, compact).Clicked() {
						s.sources = without(s.sources, source)
					}
				}
			})
		}
		if s.actionsOpen {
			p.commandList(c)
		}
		promptHeight := float32(76)
		if compact {
			promptHeight = 44
		}
		entry := area(c, "research-draft", "Research prompt", &s.draft, promptPlaceholder(s.status), promptHeight).
			OnTextKey(func(key ui.TextKey) bool {
				if key.Key != ui.KeyEnter || key.Mods != 0 {
					return false
				}
				if s.actionsOpen && strings.HasPrefix(s.draft, "/") {
					if command := p.matchingCommand(strings.TrimSpace(strings.TrimPrefix(s.draft, "/"))); command != nil {
						s.draft = "/" + command.Name + " "
						s.actionsOpen = false
						key.SetText(s.draft)
						return true
					}
				}
				if s.status == statusWorking {
					p.followUp()
				} else {
					p.send()
				}
				key.SetText("")
				return true
			})
		entry.Border(0, ui.Transparent).NoShadow().FocusRing(false)
		if compact {
			entry.MinHeight(promptHeight)
		}
		if entry.Changed() && strings.HasPrefix(s.draft, "/") {
			s.actionsOpen = true
		}
		ui.Row(c).Wrap().Gap(4).AlignItems(ui.Center).Children(func() {
			if smallButton(c, "Attach", "Attach source files").Disabled(s.status == statusOffline || s.status == statusConnecting).Clicked() {
				p.attachSources()
			}
			if smallButton(c, "Actions", "Browse workflows, skills and extensions").Disabled(len(s.commands) == 0).Clicked() {
				s.actionsOpen = !s.actionsOpen
				if s.actionsOpen {
					s.actionsOpen = true
				}
			}
			if doc := p.window.Active(); doc.ID != "" && doc.Path != "" && !doc.ReadOnly {
				ui.Checkbox(c.Key("research-include-document"), &s.includeDoc, "Include document")
			}
			ui.Row(c).Grow(1).Justify(ui.End).Gap(4).Children(func() {
				if s.status == statusWorking {
					if smallButton(c, "Steer", "Send a steering message").Disabled(strings.TrimSpace(s.draft) == "").Clicked() {
						p.steer()
					}
					if smallButton(c, "Follow up", "Queue a follow-up message").Disabled(strings.TrimSpace(s.draft) == "").Clicked() {
						p.followUp()
					}
					if controlButton(c, "Stop", "Stop the current research action").Clicked() {
						p.run(func() error { return p.host.Abort(p.window.Context(), s.root) })
					}
				} else {
					if pill(c, "research-control-Send", "Send", "Send research prompt", portrait.Accent, false).Disabled(s.status != statusReady || strings.TrimSpace(s.draft) == "").Clicked() {
						p.send()
					}
				}
			})
		})
	})
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
	s := p.state
	query := ""
	if strings.HasPrefix(s.draft, "/") {
		query = strings.TrimSpace(strings.TrimPrefix(s.draft, "/"))
	}
	filter := field(ui.TextInput(c.Key("research-actions-filter"), &p.commandQuery).Label("Filter research actions").Placeholder("Find a workflow, skill or extension…").Height(29))
	if filter.Changed() {
		query = p.commandQuery
	}
	ui.Scroll(c.Key("research-actions-list")).MaxHeight(170).FillWidth().Label("Research actions").Children(func() {
		count := 0
		for _, command := range s.commands {
			search := strings.ToLower(command.Name + " " + command.Description + " " + command.Source)
			if query != "" && !strings.Contains(search, strings.ToLower(query)) {
				continue
			}
			count++
			label := "/" + command.Name
			if command.Description != "" {
				label += " · " + command.Description
			}
			if discreteButton(c, label, "Choose research action "+command.Name).Clicked() {
				s.draft = "/" + command.Name + " "
				s.actionsOpen = false
				p.commandQuery = ""
			}
		}
		if count == 0 {
			mutedText(c, "No matching workflows, skills or extensions.")
		}
	})
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
	toolList := title == "Tools" && len(question.Options) > 0
	for _, option := range question.Options {
		toolList = toolList && strings.Contains(option.Name, " — ")
	}
	ui.Column(c.Key("research-question")).Role(ui.RoleGroup).Label("Feynman confirmation").Padding(8, 10).Gap(5).Radius(9).Background(colors.WarnSoft).Border(0.5, colors.Line).Children(func() {
		text(c, portrait.Type.UI, title).FontWeight(600).TextColor(colors.Ink).FillWidth().Wrap()
		if question.Message != "" && question.Message != title {
			text(c, portrait.Type.Meta, question.Message).TextColor(colors.Ink2).FillWidth().Wrap()
		}
		if textEntry {
			if question.Kind == "editor" {
				area(c, "research-question-answer", title, &p.questionText, question.Placeholder, 78)
			} else {
				input(c, "research-question-answer", title, &p.questionText, question.Placeholder)
			}
			ui.Row(c).Gap(4).Children(func() {
				if controlButton(c, "Submit", "Submit answer to Feynman").Disabled(p.questionPending).Clicked() {
					p.answerQuestion(p.questionText, false)
				}
				if smallButton(c, "Cancel", "Cancel this Feynman request").Disabled(p.questionPending).Clicked() {
					p.answerQuestion("", true)
				}
			})
			return
		}
		options := question.Options
		filterable := len(options) > 6
		if filterable {
			input(c, "research-question-filter", "Filter choices", &p.questionFind, "Find a choice…")
		}
		if confirm {
			ui.Row(c).Gap(4).Children(func() {
				for _, option := range options {
					if smallButton(c, option.Name, "Answer "+option.Name).Disabled(p.questionPending).Clicked() {
						p.answerQuestion(option.ID, false)
					}
				}
				if smallButton(c, "Cancel", "Cancel confirmation").Clicked() {
					p.answerQuestion("", true)
				}
			})
			return
		}
		ui.Scroll(c.Key("research-question-options")).MaxHeight(130).Label("Feynman choices").Children(func() {
			for _, option := range options {
				if filterable && !strings.Contains(strings.ToLower(option.Name), strings.ToLower(strings.TrimSpace(p.questionFind))) {
					continue
				}
				var icons []portrait.Icon
				if p.questionPick == option.ID {
					icons = append(icons, portrait.IconCheck)
				}
				button := discreteButton(c, option.Name, "Choose "+option.Name, icons...).Checked(p.questionPick == option.ID)
				if p.questionPick == option.ID {
					button.Background(colors.AccentSoft).NoShadow()
				}
				if button.Clicked() {
					p.questionPick = option.ID
				}
			}
		})
		ui.Row(c).Gap(4).Children(func() {
			if controlButton(c, "Confirm", "Confirm selected choice").Disabled(p.questionPick == "" || p.questionPending).Clicked() {
				p.answerQuestion(p.questionPick, false)
			}
			if toolList && smallButton(c, "Use in question", "Insert the selected tool into the Feynman prompt").Disabled(p.questionPick == "" || p.questionPending).Clicked() {
				p.answerQuestion(p.questionPick, false, true)
			}
			if smallButton(c, "Cancel", "Cancel this Feynman request").Disabled(p.questionPending).Clicked() {
				p.answerQuestion("", true)
			}
		})
	})
}

func (p *panel) sessions(c *ui.Context) {
	s := p.state
	card(ui.Column(c.Key("feynman-session-history"))).FillWidth().Padding(6, 8).Gap(4).Children(func() {
		title := "Conversations"
		if p.sessionView == "fork" {
			title = "Fork from a message"
		} else if p.sessionView == "tree" {
			title = "Conversation tree"
		}
		back := func() {
			if p.sessionView != "" && smallButton(c, "Back", "Return to conversation history").Clicked() {
				p.sessionView = ""
			}
		}
		close := func() {
			if smallButton(c, "Close", "Close conversation history").Clicked() {
				s.sessionsOpen = false
			}
		}
		actions := func() {
			if smallButton(c, "New", "Start a new conversation").Disabled(s.status != statusReady).Clicked() {
				p.newSession()
			}
			if smallButton(c, "Clone", "Clone this conversation").Disabled(s.status != statusReady || s.draft != "").Clicked() {
				p.cloneSession()
			}
			if smallButton(c, "Compact", "Compact conversation context").Disabled(s.status != statusReady).Clicked() {
				p.compactSession("")
			}
			if smallButton(c, "Fork", "Fork from a message in this conversation").Disabled(s.status != statusReady || s.draft != "").Clicked() {
				p.loadForkMessages()
			}
			if smallButton(c, "Tree", "Show conversation tree").Clicked() {
				p.loadConversationTree()
			}
		}
		if compactResearchLayout(c) {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				back()
				sectionTitle(c, title).Grow(1).MinWidth(0).MaxLines(1).Ellipsis("…")
				close()
			})
			ui.Row(c).Wrap().Gap(4).Children(actions)
		} else {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				back()
				sectionTitle(c, title)
				ui.Row(c).Grow(1)
				actions()
				close()
			})
		}
		if p.sessionView == "" {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				input(c, "feynman-session-name", "Conversation name", &s.renameDraft, "Rename conversation")
				if smallButton(c, "Save name", "Rename this conversation").Disabled(strings.TrimSpace(s.renameDraft) == "").Clicked() {
					p.renameSession()
				}
			})
			input(c, "feynman-session-filter", "Filter conversations", &s.sessionFilter, "Find a conversation…")
			ui.Scroll(c.Key("feynman-sessions-list")).MaxHeight(170).FillWidth().Label("Workspace conversations").Children(func() {
				for _, row := range s.sessionRows {
					title := row.Title
					if title == "" {
						title = "Untitled conversation"
					}
					if !strings.Contains(strings.ToLower(title), strings.ToLower(s.sessionFilter)) {
						continue
					}
					var icons []portrait.Icon
					if row.Current {
						icons = append(icons, portrait.IconCheck)
					}
					if discreteButton(c, title, row.Preview, icons...).Disabled(row.Current || s.status != statusReady || s.draft != "" || len(s.sources) > 0).Clicked() {
						p.selectSession(row.ID)
					}
					if !row.Current && smallButton(c, "Delete", "Delete conversation "+title).Clicked() {
						p.deleteSession(row.ID)
					}
				}
				if len(s.sessionRows) == 0 {
					mutedText(c, "No saved conversations for this workspace.")
				}
			})
		} else if p.sessionView == "fork" {
			ui.Scroll(c.Key("feynman-fork-messages")).MaxHeight(170).FillWidth().Label("Messages available for a conversation fork").Children(func() {
				for _, item := range p.forkMessages {
					label := item.Text
					if len(label) > 180 {
						label = label[:180] + "…"
					}
					if discreteButton(c, label, "Fork conversation from this message").Disabled(s.status != statusReady || s.draft != "").Clicked() {
						p.forkFrom(item.EntryID)
					}
				}
				if len(p.forkMessages) == 0 {
					mutedText(c, "No messages are available for forking.")
				}
			})
		} else {
			ui.Scroll(c.Key("feynman-session-tree")).MaxHeight(170).FillWidth().Label("Conversation tree").Children(func() {
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
	row := ui.Row(c).Gap(6).Padding(4, 6).Margin(0, 0, 0, float32(depth)*12).Radius(7).AlignItems(ui.Center).FillWidth()
	if current {
		row.Background(colors.AccentSoft)
	}
	row.Children(func() {
		if current {
			portrait.Glyph(c, portrait.IconCheck, 12, colors.Accent)
		}
		text(c, portrait.Type.Meta, label).TextColor(colors.Ink).Grow(1).MinWidth(0).Wrap()
	})
	for _, child := range node.Children {
		p.treeNode(c, child, depth+1)
	}
}

var _ = workspace.Snapshot{}
