package nativefeynman

import (
	"errors"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

func (p *panel) run(run func() error) {
	root := p.state.root
	go func() {
		err := run()
		p.update(root, func() {
			if err != nil {
				p.state.error = err.Error()
			}
		})
	}()
}

func (p *panel) runCLI(run func() (string, error)) {
	root := p.state.root
	p.state.cliBusy = true
	p.state.cliError, p.state.cliResult = "", ""
	go func() {
		result, err := run()
		p.update(root, func() {
			p.state.cliBusy = false
			if err != nil {
				p.state.cliError = err.Error()
				return
			}
			p.state.cliResult = result
		})
	}()
}

func (p *panel) loadSessions(root string) {
	go func() {
		rows, err := p.host.Sessions(p.window.Context(), root)
		p.update(root, func() {
			if err != nil {
				p.state.error = err.Error()
				return
			}
			p.state.sessionRows = rows
		})
	}()
}

func (p *panel) send() {
	s := p.state
	if s.status != statusReady || strings.TrimSpace(s.draft) == "" {
		return
	}
	root := s.root
	prompt := strings.TrimSpace(s.draft)
	sources := append([]string(nil), s.sources...)
	document := p.window.Active()
	var base workspace.Snapshot
	var documentID string
	if s.includeDoc && document.ID != "" && document.Path != "" && !document.ReadOnly && document.WorkspaceRoot == root {
		snapshot, err := p.window.LoadResearchDocument(document.ID)
		if err != nil {
			s.error = err.Error()
			return
		}
		if snapshot.ID != document.ID || snapshot.State.Path != document.Path || snapshot.State.WorkspaceRoot != root {
			s.error = "The active document changed before research started. Try again."
			return
		}
		base, documentID = snapshot, document.ID
	}
	s.draft = ""
	s.sources = nil
	s.status, s.progress, s.error = statusWorking, "Working…", ""
	p.window.Native().Invalidate()
	go func() {
		var proposed string
		var err error
		if len(sources) > 0 {
			proposed, err = p.host.PromptWithSources(p.window.Context(), root, prompt, documentID, base.Text, sources)
		} else {
			proposed, err = p.host.Prompt(p.window.Context(), root, prompt, documentID, base.Text)
		}
		if err == nil && documentID != "" && proposed != base.Text {
			if p.review == nil {
				err = errors.New("Document review is unavailable.")
			} else {
				err = submitProposal(p.review, root, document, base, proposed)
			}
		}
		p.update(root, func() {
			if err != nil {
				p.state.error = err.Error()
				p.state.draft = prompt
				p.state.sources = mergeSources(sources, p.state.sources)
			}
			if p.state.status == statusWorking {
				p.state.status = statusReady
				p.state.progress = ""
			}
			p.refreshFiles(root)
		})
	}()
}

func (p *panel) steer() {
	p.queueMessage(true)
}

func (p *panel) followUp() {
	p.queueMessage(false)
}

func (p *panel) queueMessage(steer bool) {
	s := p.state
	if s.status != statusWorking || strings.TrimSpace(s.draft) == "" {
		return
	}
	root := s.root
	text := strings.TrimSpace(s.draft)
	sources := append([]string(nil), s.sources...)
	s.draft, s.sources, s.error = "", nil, ""
	p.window.Native().Invalidate()
	go func() {
		var err error
		if steer {
			err = p.host.Steer(p.window.Context(), root, text, sources)
		} else {
			err = p.host.FollowUp(p.window.Context(), root, text, sources)
		}
		p.update(root, func() {
			if err != nil {
				p.state.error = err.Error()
				p.state.draft = text
				p.state.sources = mergeSources(sources, p.state.sources)
			}
		})
	}()
}

func mergeSources(first, second []string) []string {
	seen := make(map[string]bool, len(first)+len(second))
	result := make([]string, 0, len(first)+len(second))
	for _, source := range append(append([]string(nil), first...), second...) {
		if !seen[source] {
			seen[source] = true
			result = append(result, source)
		}
	}
	return result
}

func (p *panel) attachSources() {
	s := p.state
	if s.root == "" || s.status == statusOffline || s.status == statusConnecting {
		return
	}
	root := s.root
	p.runUpdate(func() (func(), error) {
		paths, err := p.host.PickSources(p.window.Context(), root)
		if err != nil {
			return nil, err
		}
		return func() { p.state.sources = mergeSources(p.state.sources, paths) }, nil
	})
}

func (p *panel) runUpdate(run func() (func(), error)) {
	if p.state.cliBusy {
		return
	}
	root := p.state.root
	p.state.cliBusy = true
	p.window.Native().Invalidate()
	go func() {
		apply, err := run()
		p.update(root, func() {
			p.state.cliBusy = false
			if err != nil {
				p.state.error = err.Error()
				return
			}
			if apply != nil {
				apply()
			}
		})
	}()
}

func (p *panel) answerQuestion(value string, cancelled bool, useTool ...bool) {
	question := p.state.permission
	if question == nil || question.RequestID == "" || p.questionPending {
		return
	}
	root, requestID, kind := p.state.root, question.RequestID, question.Kind
	toolPrompt := ""
	if len(useTool) > 0 && useTool[0] {
		for _, option := range question.Options {
			if option.ID == value {
				name, _, _ := strings.Cut(option.Name, " — ")
				if name != option.Name {
					toolPrompt = "Use the " + name + " tool to "
				}
				break
			}
		}
	}
	p.questionPending = true
	go func() {
		var err error
		switch kind {
		case "input", "editor":
			err = p.host.AnswerText(p.window.Context(), root, requestID, value, cancelled)
		case "confirm":
			err = p.host.AnswerConfirm(p.window.Context(), root, requestID, value == "yes", cancelled)
		default:
			if cancelled {
				value = ""
			}
			err = p.host.Answer(p.window.Context(), root, requestID, value)
		}
		p.update(root, func() {
			p.questionPending = false
			if err != nil {
				p.state.error = err.Error()
				return
			}
			if p.state.permission != nil && p.state.permission.RequestID == requestID {
				p.state.permission = nil
			}
			if toolPrompt != "" {
				p.state.draft = strings.TrimSpace(p.state.draft + " " + toolPrompt)
				p.state.selectedTab = "chat"
			}
		})
	}()
}

func (p *panel) dismissError() { p.state.error = "" }

func (p *panel) openArtifact(file review.ResearchArtifact) {
	if file.Error != "" {
		p.state.error = file.Error
		return
	}
	root := p.state.root
	if file.Missing {
		p.runUpdate(func() (func(), error) {
			path, err := p.host.RecoverArtifact(p.window.Context(), root, file.Path)
			if err != nil {
				return nil, err
			}
			return func() {
				p.refreshFiles(root)
				p.openDocumentPath(path, false, "")
			}, nil
		})
		return
	}
	if file.Target == "" {
		p.state.error = "This research artifact has no published workspace file."
		return
	}
	if !file.Proposal {
		p.openDocumentPath(file.Target, false, "")
		return
	}
	p.runUpdate(func() (func(), error) {
		proposed, err := p.host.ReadArtifact(p.window.Context(), root, file.Path)
		if err != nil {
			return nil, err
		}
		if p.review == nil {
			return nil, errors.New("Document review is unavailable.")
		}
		return func() { p.openDocumentPath(file.Target, true, proposed) }, nil
	})
}

func (p *panel) openDocumentPath(path string, proposal bool, proposed string) {
	root := p.state.root
	p.window.OpenResearchDocument(path, func(snapshot workspace.Snapshot, err error) {
		if err != nil {
			p.state.error = err.Error()
			return
		}
		if !proposal {
			return
		}
		if snapshot.State.WorkspaceRoot != root || snapshot.ID == "" || snapshot.State.Path == "" {
			p.state.error = "The artifact opened outside its research workspace."
			return
		}
		if snapshot.Text == proposed {
			return
		}
		if err := submitProposal(p.review, root, snapshot.State, snapshot, proposed); err != nil {
			p.state.error = err.Error()
		}
	})
}

func (p *panel) selectSession(id string) {
	if p.state.status != statusReady || p.state.draft != "" || len(p.state.sources) > 0 {
		p.state.error = "Finish the current draft and remove attached sources before switching conversations."
		return
	}
	root := p.state.root
	p.state.status = statusConnecting
	p.runUpdate(func() (func(), error) {
		err := p.host.SelectSession(p.window.Context(), root, id)
		if err != nil {
			return func() { p.state.status = statusReady; p.state.error = err.Error() }, nil
		}
		state, stateErr := p.host.State(p.window.Context(), root)
		rows, rowsErr := p.host.Sessions(p.window.Context(), root)
		if stateErr != nil {
			return func() { p.state.status = statusReady; p.state.error = stateErr.Error() }, nil
		}
		if rowsErr != nil {
			return func() { p.state.status = statusReady; p.state.error = rowsErr.Error() }, nil
		}
		return func() {
			p.state.status = statusReady
			p.state.session = state
			p.state.title = state.SessionName
			p.state.sessionRows = rows
			p.state.sessionsOpen = false
		}, nil
	})
}

func (p *panel) newSession() {
	if p.state.status != statusReady {
		return
	}
	root := p.state.root
	p.run(func() error { return p.host.NewSession(p.window.Context(), root, "") })
}

func (p *panel) deleteSession(id string) {
	root := p.state.root
	p.runUpdate(func() (func(), error) {
		if err := p.host.DeleteSession(p.window.Context(), root, id); err != nil {
			return nil, err
		}
		rows, err := p.host.Sessions(p.window.Context(), root)
		return func() { p.state.sessionRows = rows }, err
	})
}

func (p *panel) renameSession() {
	root, name := p.state.root, strings.TrimSpace(p.state.renameDraft)
	if name == "" {
		return
	}
	p.runUpdate(func() (func(), error) {
		if err := p.host.SetSessionName(p.window.Context(), root, name); err != nil {
			return nil, err
		}
		return func() {
			p.state.title = name
			p.state.session.SessionName = name
			p.state.renameDraft = ""
		}, nil
	})
}

func (p *panel) cloneSession() {
	root := p.state.root
	p.runUpdate(func() (func(), error) {
		cloned, err := p.host.Clone(p.window.Context(), root)
		if err != nil || !cloned {
			if err == nil {
				err = errors.New("Feynman could not clone this conversation.")
			}
			return nil, err
		}
		rows, err := p.host.Sessions(p.window.Context(), root)
		return func() { p.state.sessionRows = rows }, err
	})
}

func (p *panel) compactSession(instructions string) {
	root := p.state.root
	p.runUpdate(func() (func(), error) {
		result, err := p.host.Compact(p.window.Context(), root, instructions)
		return func() {
			if result.Summary != "" {
				p.state.messages = append(p.state.messages, message{kind: "notice", text: "Conversation compacted: " + result.Summary})
			}
		}, err
	})
}
