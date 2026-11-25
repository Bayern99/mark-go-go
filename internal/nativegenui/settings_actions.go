package nativegenui

import (
	"strings"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

type backendEdit struct {
	command, arguments             string
	commandEdited, argumentsEdited bool
	advancedOpen, advancedChosen   bool
}

// backendStatus is what the last Test & Refresh learned about one backend.
type backendStatus struct {
	probe   genui.ProbeResult
	catalog genui.ModelCatalog
	tested  bool
	busy    bool
}

func (p *panel) status(id string) *backendStatus {
	if p.backends == nil {
		p.backends = map[string]*backendStatus{}
	}
	s := p.backends[id]
	if s == nil {
		s = &backendStatus{}
		p.backends[id] = s
	}
	return s
}

func (p *panel) editFor(id string) *backendEdit {
	if p.edits == nil {
		p.edits = map[string]*backendEdit{}
	}
	edit := p.edits[id]
	if edit == nil {
		edit = &backendEdit{}
		p.edits[id] = edit
	}
	backend := p.settings.GenUI.For(id)
	if !edit.commandEdited {
		edit.command = backend.Command
	}
	if !edit.argumentsEdited {
		edit.arguments = strings.Join(backend.Args, " ")
	}
	if !edit.advancedChosen {
		edit.advancedOpen = id == genui.BackendCustom
		edit.advancedChosen = true
	}
	return edit
}

func (p *panel) selectBackend(id string) {
	if p.save == nil {
		return
	}
	next, err := p.save(workspace.SettingsPatch{GenUI: &workspace.GenUIPatch{Backend: &id}})
	if err != nil {
		p.settingsMessage = err.Error()
		return
	}
	p.settingsMessage = ""
	p.settings = next
}

func (p *panel) saveBackend(id string, patch workspace.GenUIBackendPatch) {
	if p.save == nil {
		return
	}
	next, err := p.save(workspace.SettingsPatch{GenUI: &workspace.GenUIPatch{Backends: map[string]workspace.GenUIBackendPatch{
		id: patch,
	}}})
	if err != nil {
		p.settingsMessage = err.Error()
		return
	}
	p.settingsMessage = ""
	p.settings = next
}

func (p *panel) testAndRefresh(id string) {
	p.startConnectionWork(id, true)
}

// probeOnFocus re-tests a backend the user already tested, so a sign-in
// finished in Terminal shows up when the window comes back. It never opens a
// session for models: that stays behind Test & Refresh.
func (p *panel) probeOnFocus() {
	id, _ := p.settings.GenUI.Active()
	if state := p.status(id); !state.tested || state.busy {
		return
	}
	p.startConnectionWork(id, false)
}

func (p *panel) startConnectionWork(id string, refreshModels bool) {
	state := p.status(id)
	if state.busy {
		return
	}
	state.busy = true
	p.settingsMessage = ""
	probe, refresh := p.probe, p.refresh
	go func() {
		var result genui.ProbeResult
		var probeErr error
		if probe != nil {
			result, probeErr = probe(id)
		}
		var catalog genui.ModelCatalog
		var refreshErr error
		if refreshModels && probeErr == nil && result.Status != genui.AgentMissing && result.Status != genui.AgentOutdated && refresh != nil {
			catalog, refreshErr = refresh(id)
		}
		p.after(func() {
			state := p.status(id)
			state.busy = false
			state.tested = true
			state.probe = result
			if len(catalog.Models) > 0 || catalog.CurrentModel != "" {
				state.catalog = catalog
			}
			p.settingsMessage = message(firstError(probeErr, refreshErr))
		})
	}()
}

func (p *panel) signInInTerminal(id string) {
	signIn := p.signIn
	if signIn == nil {
		return
	}
	go func() {
		err := signIn(id)
		p.after(func() { p.settingsMessage = message(err) })
	}()
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// after returns work to the UI thread, or runs it inline when no window exists.
func (p *panel) after(fn func()) {
	if p.runOnUI != nil {
		p.runOnUI(fn)
		return
	}
	if p.win != nil {
		if native := p.win.Native(); native != nil {
			native.Update(fn)
			return
		}
	}
	fn()
}
