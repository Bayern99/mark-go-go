package nativefeynman

import (
	"slices"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/review"
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
		Changed:  p.syncConnection,
		Closed:   p.close,
	}
}

type panel struct {
	window            *nativeui.Window
	host              *feynman.ResearchHost
	review            *review.Service
	state             *paneState
	connected         string
	settingsRoot      string
	closed            bool
	chatScroll        ui.ScrollState
	filesScroll       ui.ScrollState
	settingsScr       ui.ScrollState
	alphaScroll       ui.ScrollState
	commandQuery      string
	questionText      string
	questionPick      string
	questionFind      string
	permissionID      string
	questionPending   bool
	collapsed         map[string]*bool
	cliPassword       string
	searchKey         string
	serverLocal       bool
	serverArgs        string
	serverEnv         string
	serverHeaders     string
	serverTransport   string
	serverExposure    string
	serverName        string
	serverURL         string
	serverCommand     string
	serverCwd         string
	serverDescription string
	serverBearerEnv   string
	sessionView       string
	forkMessages      []feynman.ResearchForkMessage
	researchTree      *feynman.ResearchTree
	loginProvider     string
	tierValue         string
	searchProvider    string
	packageTarget     string
	modelTarget       string
	alphaQuery        string
	alphaMode         string
	alphaPaperID      string
	alphaQuestion     string
	alphaRepoURL      string
	alphaFilePath     string
	alphaNoteInput    string
	alphaFullText     bool
	alphaAnnotation   *feynman.CLIAlphaAnnotateResult
	alphaAskResult    *feynman.CLICommandResult
	alphaCodeResult   *feynman.CLICommandResult
	decodedImages     map[string]decodedDataImage
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
	if p.state.selectedTab == "settings" {
		p.loadSettings(root)
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
			text(c, portrait.Type.UI, "Open a workspace folder to start research.").TextColor(colors.Ink3).Padding(8, 10)
			return
		}
		p.tabs(c)
		if s.error != "" {
			ui.Row(c).Role(ui.RoleAlert).Margin(0, 0, 6, 0).Padding(6, 9).Gap(6).Radius(8).Background(colors.DangerSoft).AlignItems(ui.Center).Children(func() {
				portrait.Glyph(c, portrait.IconWarningCircle, 15, colors.Danger)
				text(c, portrait.Type.Meta, s.error).Grow(1).MinWidth(0).Wrap().TextColor(colors.Danger)
				if iconButton(c, portrait.IconX, "Dismiss research error").Clicked() {
					s.error = ""
				}
			})
		}
		switch s.selectedTab {
		case "files":
			p.files(c)
		case "settings":
			p.settings(c)
		case "alpha":
			p.alphaXiv(c)
		default:
			p.chat(c)
		}
	})
}

func (p *panel) heading(c *ui.Context) {
	s := p.state
	portrait.Glyph(c, portrait.IconAtom, 20, colors.Accent)
	ui.Column(c).Grow(1).MinWidth(0).Children(func() {
		text(c, portrait.Type.Small, "Feynman").FontWeight(600).TextColor(colors.Ink)
		name := s.root
		if i := strings.LastIndex(name, "/"); i >= 0 && i+1 < len(name) {
			name = name[i+1:]
		}
		if s.title != "" {
			name += " · " + s.title
		}
		portrait.Tooltip(c, text(c, portrait.Type.Meta, name).MaxLines(1).Ellipsis("…").TextColor(colors.Ink3), s.root)
	})
	if s.status == statusWorking {
		portrait.Spinner(c, "Feynman is working")
	}
	status := string(s.status)
	if status == "" {
		status = string(statusOffline)
	}
	text(c, portrait.Type.Meta, status).TextColor(statusColor(s.status)).Label("Feynman connection status")
}

func (p *panel) header(c *ui.Context) {
	s := p.state
	compact := compactResearchLayout(c)
	width, height := c.Size()
	connect := func() {
		if p.connected == "" && s.root != "" && controlButton(c, "Connect", "Connect Feynman to this workspace").Clicked() {
			p.syncConnection()
		}
	}
	history := func() ui.Element {
		button := controlButton(c, "History", "Workspace conversation history").Expanded(s.sessionsOpen)
		if button.Clicked() {
			s.sessionsOpen = !s.sessionsOpen
			if s.sessionsOpen {
				p.loadSessions(s.root)
			}
		}
		if compact {
			portrait.Popover(c, button, &s.sessionsOpen, func() {
				popoverWidth := min(float32(352), max(float32(280), width-128))
				maxHeight := max(float32(120), height-170)
				ui.Column(c).Width(popoverWidth).Children(func() {
					ui.Scroll(c.Key("feynman-history-popover-scroll")).MaxHeight(maxHeight).FillWidth().Label("Workspace conversation history").Children(func() {
						p.sessions(c)
					})
				})
			})
		}
		return button
	}
	if compact {
		ui.Column(c.Key("feynman-heading")).Padding(2, 6).Children(func() {
			ui.Row(c.Key("feynman-heading-title")).MinHeight(30).Gap(6).AlignItems(ui.Center).Children(func() {
				history()
				p.heading(c)
				connect()
			})
		})
		return
	}
	ui.Row(c.Key("feynman-heading")).MinHeight(38).Gap(8).AlignItems(ui.Center).Padding(4, 6).Children(func() {
		p.heading(c)
		connect()
		history()
	})
	if s.sessionsOpen {
		p.sessions(c)
	}
}

func (p *panel) tabs(c *ui.Context) {
	s := p.state
	portrait.Track(c, "feynman tabs", func(t *portrait.Segments) {
		for _, tab := range []struct{ id, label string }{{"chat", "Chat"}, {"files", "Files"}, {"settings", "Settings"}, {"alpha", "alphaXiv"}} {
			selected := s.selectedTab == tab.id
			button := t.Segment(c, tab.label, tab.label, selected).Grow(1).Role(ui.RoleTab).AccessSelected(selected)
			if button.Clicked() {
				s.selectedTab = tab.id
				if tab.id == "files" {
					p.refreshFiles(s.root)
				}
				if tab.id == "settings" && p.settingsRoot != s.root {
					p.loadSettings(s.root)
				}
				if tab.id == "alpha" && s.cliStatus == nil {
					p.loadSettings(s.root)
				}
			}
		}
	}).Margin(2, 0, 6, 0)
}
