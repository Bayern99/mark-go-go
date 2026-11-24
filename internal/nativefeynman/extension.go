package nativefeynman

import (
	"slices"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

type Extension struct {
	Research *feynman.ResearchHost
	Review   *review.Service
}

func NewExtension(research *feynman.ResearchHost, reviewService *review.Service) func(*nativeui.Window) nativeui.Panels {
	return Extension{Research: research, Review: reviewService}.Attach
}

func (e Extension) Attach(window *nativeui.Window) nativeui.Panels {
	p := &panel{window: window, host: e.Research, review: e.Review, state: newPaneState(""), serverTransport: "http", serverExposure: "direct", alphaMode: "semantic", collapsed: make(map[string]*bool)}
	feynman.Updated.Listen(window.Native(), p.researchUpdated)
	feynman.FocusRequested.Listen(window.Native(), p.focusRequested)
	review.ResearchFilePublished.Listen(window.Native(), p.filePublished)
	return nativeui.Panels{
		Research: p.draw,
		Settings: p.settingsPages(),
		Changed:  p.syncConnection,
		Closed:   p.close,
	}
}

type panel struct {
	window               *nativeui.Window
	host                 *feynman.ResearchHost
	review               *review.Service
	state                *paneState
	connected            string
	settingsRoot         string
	closed               bool
	chatScroll           ui.ScrollState
	chatFollow           bool
	chatReady            bool
	chatLastY            float32
	chatLastMaxY         float32
	filesScroll          ui.ScrollState
	alphaScroll          ui.ScrollState
	commandQuery         string
	actionsDismissed     bool
	questionText         string
	questionPick         string
	questionFind         string
	permissionID         string
	questionPending      bool
	collapsed            map[string]*bool
	pickerStates         map[string]*researchPickerState
	pickerFocus          string
	localPrefs           map[string]string
	cliPassword          string
	searchKey            string
	mcpAddOpen           bool
	sessionMenu          bool
	serverLocal          bool
	serverArgs           string
	serverEnv            string
	serverHeaders        string
	serverTransport      string
	serverExposure       string
	serverName           string
	serverURL            string
	serverCommand        string
	serverCwd            string
	serverDescription    string
	serverBearerEnv      string
	sessionView          string
	forkMessages         []feynman.ResearchForkMessage
	researchTree         *feynman.ResearchTree
	loginProvider        string
	tierValue            string
	tierLoaded           bool
	searchProvider       string
	packageTarget        string
	modelTarget          string
	alphaQuery           string
	alphaMode            string
	alphaPaperID         string
	alphaQuestion        string
	alphaRepoURL         string
	alphaFilePath        string
	alphaNoteInput       string
	alphaFullText        bool
	alphaAnnotation      *feynman.CLIAlphaAnnotateResult
	alphaAskResult       *feynman.CLICommandResult
	alphaCodeResult      *feynman.CLICommandResult
	decodedImages        map[string]decodedDataImage
	confirmationOpen     bool
	confirmationSpec     portrait.ConfirmSpec
	confirmationReturns  map[string]*ui.Handle
	activeDocument       func() workspace.DocumentInfo
	loadResearchDocument func(string) (workspace.Snapshot, error)
	submitPrompt         func(root, prompt, documentID, base string, sources []string) (string, error)
}

func (p *panel) collapsibleState(key string) *bool {
	if p.collapsed == nil {
		p.collapsed = make(map[string]*bool)
	}
	if p.collapsed[key] == nil {
		p.collapsed[key] = new(bool)
	}
	return p.collapsed[key]
}

func (p *panel) syncConnection() {
	p.connectRoot(false)
}

// reconnect tears down the current connection and starts a new one. It is only
// called from explicit user actions, so failures never start a retry loop.
func (p *panel) reconnect() {
	p.connectRoot(true)
}

func (p *panel) connectRoot(force bool) {
	if p.closed || p.host == nil || p.window.Native() == nil {
		return
	}
	root := p.window.Workspace().Root
	if !p.window.PluginEnabled("feynman") || root == "" {
		p.disconnect(root == "")
		return
	}
	root = p.stateRoot(root)
	if root == p.connected && !force {
		return
	}
	if force && p.state.status == statusConnecting {
		return
	}
	previous := p.connected
	p.connected = root
	if previous != root {
		p.switchRoot(root)
	}
	p.state.status = statusConnecting
	p.state.error = ""
	p.window.Native().Invalidate()
	ctx := p.window.Context()
	go func() {
		if previous != "" {
			_ = p.host.Disconnect(ctx, previous)
		}
		err := p.host.Connect(ctx, root)
		if err != nil {
			p.update(root, func() {
				p.state.status = statusOffline
				p.state.error = err.Error()
			})
			return
		}
		_ = p.host.WatchFocus(ctx, root)
		state, stateErr := p.host.State(ctx, root)
		sessions, sessionsErr := p.host.Sessions(ctx, root)
		stats, statsErr := p.host.SessionStats(ctx, root)
		files, filesErr := p.host.Artifacts(ctx, root)
		commands, commandsErr := p.host.Commands(ctx, root)
		p.update(root, func() {
			if p.state.status == statusConnecting {
				p.state.status = statusReady
			}
			if stateErr == nil {
				p.state.session = state
				p.state.title = state.SessionName
			}
			if sessionsErr == nil {
				p.state.sessionRows = sessions
			}
			if statsErr == nil {
				p.state.stats, p.state.hasStats = stats, true
			}
			if filesErr == nil {
				p.state.artifacts = files
			} else {
				p.state.filesError = filesErr.Error()
			}
			if commandsErr == nil {
				p.state.commands = commands
			}
		})
	}()
}

func (p *panel) stateRoot(root string) string {
	return fsutil.CanonicalPath(root)
}

func (p *panel) disconnect(clearRoot bool) {
	if p.connected != "" {
		previous := p.connected
		p.connected = ""
		if p.state != nil {
			p.state.status = statusOffline
		}
		if p.host != nil && p.window != nil {
			go func() { _ = p.host.Disconnect(p.window.Context(), previous) }()
		}
	}
	if clearRoot && p.state != nil && p.state.root != "" {
		p.switchRoot("")
	}
}

func (p *panel) resetQuestionState() {
	p.questionText = ""
	p.questionPick = ""
	p.questionFind = ""
	p.permissionID = ""
	p.questionPending = false
}

func (p *panel) resetSessionViews() {
	p.decodedImages = nil
	p.sessionView = ""
	p.forkMessages = nil
	p.researchTree = nil
	p.resetQuestionState()
}

// switchRoot drops every view that describes the previous workspace so
// stale conversations, forks, and trees cannot leak into the new root.
func (p *panel) switchRoot(root string) {
	p.settingsRoot = ""
	p.confirmationOpen = false
	p.confirmationSpec = portrait.ConfirmSpec{}
	p.state = newPaneState(root)
	p.resetSessionViews()
}

func (p *panel) update(root string, change func()) {
	win := p.window.Native()
	if win == nil {
		return
	}
	win.Update(func() {
		if win.IsDestroyed() || p.closed || p.connected != root || p.state.root != root {
			return
		}
		change()
		win.Invalidate()
	})
}

func (p *panel) researchUpdated(update feynman.ResearchUpdate) {
	if p.state == nil || update.Root == "" || fsutil.CanonicalPath(update.Root) != p.state.root {
		return
	}
	if update.Kind == "reset" {
		p.resetSessionViews()
		p.chatFollow = true
	}
	p.state.receive(update)
	if update.Kind == "agentSettled" {
		p.refreshFiles(p.state.root)
	}
	if p.window != nil {
		if win := p.window.Native(); win != nil {
			win.Invalidate()
		}
	}
}

func (p *panel) filePublished(event review.ResearchFilePublishedEvent) {
	root := fsutil.CanonicalPath(event.WorkspaceRoot)
	if root != p.state.root {
		return
	}
	if p.state.status == statusWorking {
		files := append(p.state.artifacts, event.File)
		p.state.artifacts = dedupeArtifacts(files)
		p.window.Native().Invalidate()
		return
	}
	p.refreshFiles(root)
}

func (p *panel) focusRequested(event feynman.ResearchFocusEvent) {
	root := fsutil.CanonicalPath(event.Root)
	if root != p.state.root {
		return
	}
	if p.state.status == statusReady || p.state.status == statusWorking {
		go func() { _, _ = p.host.RefreshModels(p.window.Context(), root) }()
	}
}

func (p *panel) refreshFiles(root string) {
	p.state.filesRequest++
	request := p.state.filesRequest
	p.state.filesLoading = true
	p.state.filesError = ""
	go func() {
		files, err := p.host.Artifacts(p.window.Context(), root)
		p.update(root, func() {
			if p.state.filesRequest != request {
				return
			}
			p.state.filesLoading = false
			if err != nil {
				p.state.filesError = err.Error()
				return
			}
			p.state.artifacts = files
		})
	}()
}

func dedupeArtifacts(files []review.ResearchArtifact) []review.ResearchArtifact {
	byPath := make(map[string]review.ResearchArtifact, len(files))
	for _, file := range files {
		byPath[file.Path] = file
	}
	result := make([]review.ResearchArtifact, 0, len(byPath))
	for _, file := range byPath {
		result = append(result, file)
	}
	slices.SortFunc(result, func(a, b review.ResearchArtifact) int { return strings.Compare(a.Path, b.Path) })
	return result
}

func (p *panel) close() {
	p.closed = true
	p.decodedImages = nil
	if root := p.connected; root != "" && p.host != nil {
		p.connected = ""
		ctx := p.window.Context()
		go func() { _ = p.host.Disconnect(ctx, root) }()
	}
}

func (p *panel) draw(c *ui.Context) {
	p.syncConnection()
	s := p.state
	ui.Column(c.Key("feynman-pane")).Fill().MinWidth(0).Children(func() {
		p.header(c)
		if s.root == "" {
			portrait.Empty(c, portrait.EmptySpec{Icon: portrait.IconFolderOpen, Title: "Open a workspace folder", Description: "Open a workspace folder to start research."})
			return
		}
		p.tabs(c)
		if s.error != "" {
			portrait.Alert(c, portrait.Tones.Danger, "Research error", s.error, portrait.AlertAction{Label: "Dismiss", OnAction: func() { s.error = "" }})
		}
		switch s.selectedTab {
		case "files":
			p.files(c)
		case "alpha":
			p.alphaXiv(c)
		default:
			p.chat(c)
		}
	})
	p.drawConfirmation(c)
}

func (p *panel) openSettings(tab string) {
	if p.state != nil && p.state.root != "" && p.settingsRoot != p.state.root {
		p.loadSettings(p.state.root)
	}
	if p.window != nil {
		p.window.OpenSettings(tab)
	}
}

func (p *panel) heading(c *ui.Context) {
	s := p.state
	portrait.Glyph(c, portrait.IconAtom, portrait.IconSizes.M, portrait.Colors.Accent).Shrink(0)
	name := s.root
	if i := strings.LastIndex(name, "/"); i >= 0 && i+1 < len(name) {
		name = name[i+1:]
	}
	if s.title != "" {
		name += " · " + s.title
	}
	status := string(s.status)
	if status == "" {
		status = string(statusOffline)
	}
	tip := status
	if name != "" {
		tip = name + " · " + status
	}
	portrait.Tooltip(c, text(c, portrait.Type.Small, "Research").FontWeight(600).TextColor(portrait.Colors.Ink).MaxLines(1), tip)
	if s.status == statusWorking {
		portrait.Spinner(c, "Feynman is working")
		text(c, portrait.Type.Meta, status).TextColor(portrait.Colors.Ink3).Label("Feynman connection status").MaxLines(1)
	} else {
		dot := ui.Box(c.Key("feynman-status-dot")).Size(6, 6).Radius(portrait.Radii.Pill).
			Background(statusTone(s.status).Solid).Role(ui.RoleStatus).Label("Feynman connection status").Value(status).Shrink(0)
		portrait.Tooltip(c, dot, tip)
	}
	ui.Row(c).Grow(1)
}

func (p *panel) header(c *ui.Context) {
	s := p.state
	_, height := c.Size()
	var heading ui.Element
	connect := func() {
		if p.connected == "" && s.root != "" && s.status == statusOffline && toolButton(c, "Connect", "Connect Feynman to this workspace", true, false).Clicked() {
			p.syncConnection()
		}
	}
	history := func() ui.Element {
		button := portrait.ButtonWith(c.Key("research-tool-History"), "", "Workspace conversation history", portrait.ButtonSpec{
			Look:     portrait.Quiet,
			Size:     portrait.Sizes.Regular,
			Leading:  portrait.IconClockCounterClockwise,
			Selected: s.sessionsOpen,
		}).Expanded(s.sessionsOpen)
		if button.Clicked() {
			s.sessionsOpen = !s.sessionsOpen
			if s.sessionsOpen {
				p.loadSessions(s.root)
			}
		}
		// Leave room for Settings at the popover's end and begin below the tabs.
		popoverWidth := portrait.Form.MenuMin
		if panelWidth := heading.Bounds().W; panelWidth > 0 {
			popoverWidth = min(portrait.Form.MenuMax, panelWidth-portrait.Sizes.Regular-portrait.Form.Gap-2*portrait.Shell.PageInset)
		}
		maxHeight := max(portrait.Form.AreaCompact, min(portrait.Form.ListMax, height-2*portrait.Shell.PageBar-2*portrait.Form.PopoverPad))
		popover := portrait.PopoverWithSpec(c, button, &s.sessionsOpen, portrait.PopoverSpec{Width: popoverWidth, MaxHeight: maxHeight}, func() {
			p.sessions(c)
		})
		popover.Label("Workspace conversation history").Margin(portrait.Shell.PageBar, 0, 0, 0).AttachTo(button, ui.AnchorBottomEnd, ui.AnchorTopEnd)
		return button
	}
	settingsBtn := func() {
		if portrait.QuietIconButton(c, portrait.IconGear, "Feynman settings").Clicked() {
			p.openSettings("models")
		}
	}
	heading = ui.Row(c.Key("feynman-heading")).Height(portrait.Shell.PageBar).Gap(6).AlignItems(ui.Center).Padding(0, 4)
	heading.Children(func() {
		p.heading(c)
		connect()
		history()
		settingsBtn()
	})
}

func (p *panel) tabs(c *ui.Context) {
	s := p.state
	portrait.Track(c, "feynman tabs", func(t *portrait.Segments) {
		for _, tab := range []struct{ id, label string }{{"chat", "Chat"}, {"files", "Files"}, {"alpha", "alphaXiv"}} {
			selected := s.selectedTab == tab.id || (tab.id == "chat" && s.selectedTab != "files" && s.selectedTab != "alpha")
			button := t.Segment(c, tab.label, tab.label, selected).Grow(1).Role(ui.RoleTab).AccessSelected(selected)
			if button.Clicked() {
				s.selectedTab = tab.id
				if tab.id == "files" {
					p.refreshFiles(s.root)
				}
				if tab.id == "alpha" && s.cliStatus == nil {
					p.loadSettings(s.root)
				}
			}
		}
	}).Label("Research sections").Margin(2, 0, 6, 0)
}
