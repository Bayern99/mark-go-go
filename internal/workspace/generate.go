package workspace

import (
	"context"
	"errors"
	"github.com/Bayern99/mark-go-go/internal/desktop"
	"strings"
	"time"

	"github.com/Bayern99/mark-go-go/internal/acp"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/egoist/mygo"
)

const errGenBusy = "A view is being generated; wait for it to finish, or cancel it."

var (
	errPathChanged = genui.NewFailure(genui.FailAgentError, "The document's path changed while the agent worked, so the view was not written.", "Generate again.")
	errAbandoned   = genui.NewFailure(genui.FailCancelled, "Generation was cancelled.", "")
)

type WorkspaceRef struct {
	ID     string
	Root   string
	Name   string
	Window *mygo.Window
}

type CheckpointDoc struct {
	WorkspaceRoot string
	Path          string
	Content       string
	Edited        bool
	Conflict      bool
	Deleted       bool
}

func (s *Service) SetPrepareJob(fn func(*acp.GenerationJob)) {
	s.mu.Lock()
	s.prepareJob = fn
	s.mu.Unlock()
}

func (s *Service) UserDataDir() string {
	if s.settings == nil {
		return ""
	}
	return s.settings.dir
}

func (s *Service) WindowWorkspace(ctx context.Context, root string) (WorkspaceRef, error) {
	return s.windowWorkspace(desktop.Window(ctx), root)
}

func (s *Service) windowWorkspace(win *mygo.Window, root string) (WorkspaceRef, error) {
	if win != nil && root != "" {
		root = fsutil.CanonicalPath(root)
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, w := range s.windowWorkspacesLocked(win) {
			if w.root == root {
				return WorkspaceRef{
					ID:     w.id,
					Root:   w.root,
					Name:   w.name(),
					Window: w.win,
				}, nil
			}
		}
	}
	return WorkspaceRef{}, errNoWorkspace
}

func (s *Service) EditableDocument(ctx context.Context, root, documentID string) error {
	return s.editableDocument(desktop.Window(ctx), root, documentID)
}

func (s *Service) editableDocument(win *mygo.Window, root, documentID string) error {
	if win == nil || root == "" || documentID == "" {
		return errors.New("The selected document is no longer editable in this workspace.")
	}
	root = fsutil.CanonicalPath(root)
	s.mu.Lock()
	sess := s.byID[documentID]
	var w *workspace
	for _, cand := range s.windowWorkspacesLocked(win) {
		if cand.root == root {
			w = cand
			break
		}
	}
	valid := w != nil && sess != nil && !sess.closed && sess.ws == w && sess.win == w.win
	var state DocumentInfo
	if valid {
		state = sess.infoLocked()
		valid = !state.ReadOnly && !state.Conflict && !state.Deleted
	}
	s.mu.Unlock()
	if !valid {
		return errors.New("The selected document is no longer editable in this workspace.")
	}
	return nil
}

func (s *Service) CheckpointDocument(ctx context.Context, documentID, path string) (CheckpointDoc, error) {
	return s.checkpointDocument(desktop.Window(ctx), documentID, path)
}

func (s *Service) checkpointDocument(win *mygo.Window, documentID, path string) (CheckpointDoc, error) {
	s.mu.Lock()
	sess := s.byID[documentID]
	w := s.byWindow[win]
	if win == nil || sess == nil || sess.closed || sess.doc == nil || w == nil || sess.ws != w || w.active != documentID {
		s.mu.Unlock()
		return CheckpointDoc{}, errNoDocument
	}
	root := w.root
	s.mu.Unlock()
	if path == "" || sess.currentPath() != path {
		return CheckpointDoc{}, errors.New("Select the saved document again.")
	}
	st := sess.documentState()
	return CheckpointDoc{
		WorkspaceRoot: root,
		Path:          st.Path,
		Content:       sess.doc.Content(),
		Edited:        st.Edited,
		Conflict:      st.Conflict,
		Deleted:       st.Deleted,
	}, nil
}

func (st *settingsStore) SetModels(backend string, models []string) error {
	return st.setModels(backend, models)
}

func (s *docSession) viewFollower() *genui.ViewFollower {
	s.host.mu.Lock()
	if s.view == nil {
		s.view = genui.NewViewFollower(func(snap genui.ViewSnapshot) {
			s.host.mu.Lock()
			win, id, closed := s.win, s.id, s.closed
			s.host.mu.Unlock()
			if !closed && win != nil && !win.IsDestroyed() {
				_ = genui.ViewChanged.Emit(win, genui.ViewEvent{
					ID:      id,
					Exists:  snap.Exists,
					Text:    snap.Text,
					Error:   snap.Error,
					CanUndo: snap.CanUndo,
				})
			}
		})
	}
	f := s.view
	s.host.mu.Unlock()
	return f
}

func (s *docSession) unfollowView() {
	s.host.mu.Lock()
	f := s.view
	s.host.mu.Unlock()
	if f != nil {
		f.Unwatch()
	}
}

func (s *docSession) viewMoved(docPath string) {
	s.host.mu.Lock()
	f := s.view
	s.host.mu.Unlock()
	if f != nil {
		f.RetargetIfWatching(genui.ViewPath(docPath))
	}
}

func (h *Service) WatchView(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	path := genui.ViewPath(s.currentPath())
	return s.viewFollower().Watch(path)
}

func (h *Service) UnwatchView(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	s.unfollowView()
	return nil
}

func (h *Service) UndoView(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	if err := h.rejectPathChange(s); err != nil {
		return err
	}
	path := genui.ViewPath(s.currentPath())
	if path == "" {
		return errors.New("There is no earlier view to go back to.")
	}
	return genui.UndoView(path)
}

func (h *Service) Generate(ctx context.Context, documentID string, req genui.GenerateRequest, progress *mygo.Channel[genui.GenerateProgress]) error {
	send := func(p genui.GenerateProgress) error {
		if progress == nil {
			return nil
		}
		return progress.Send(p)
	}
	if err := <-h.StartGeneration(ctx, documentID, req, send); err != nil {
		return genui.ReportFailure(err, send)
	}
	return nil
}

func (h *Service) StartGeneration(ctx context.Context, documentID string, req genui.GenerateRequest, send func(genui.GenerateProgress) error) <-chan error {
	result := make(chan error, 1)
	refuse := func(err error) <-chan error { result <- err; close(result); return result }

	s, err := h.docSession(documentID)
	if err != nil {
		return refuse(err)
	}
	docPath := s.currentPath()
	if docPath == "" {
		return refuse(genui.NewFailure(genui.FailUnsaved, "Save the document first: the view is stored next to it.", ""))
	}
	var settings Settings
	if h.settings != nil {
		settings = h.settings.get()
	}
	h.mu.Lock()
	if s.closed {
		h.mu.Unlock()
		return refuse(errNoDocument)
	}
	if s.generating {
		h.mu.Unlock()
		return refuse(genui.NewFailure(genui.FailBusy, "A view is already being generated.", genui.HintBusy))
	}
	runCtx, cancel := context.WithCancel(ctx)
	slot := &generationSlot{cancel: cancel, path: docPath}
	s.generating = true
	s.genSlot = slot
	h.runs.Add(1)
	w := s.ws
	win := s.win
	prepareJob := h.prepareJob
	h.mu.Unlock()
	h.emitWorkspace(w)
	if win != nil {
		_ = DocumentStateChanged.Emit(win, s.info())
	}
	h.refreshPathMenus()

	var models acp.ModelUpdater
	if h.settings != nil {
		models = h.settings
	}
	backendID, backend := settings.GenUI.Active()
	if b, ok := genui.BackendByID(backendID); ok && strings.TrimSpace(backend.Command) == "" {
		return refuse(genui.NewFailure(genui.FailAgentNotFound, "The "+b.Name+" backend has no command yet.", b.InstallHint))
	}
	job := acp.GenerationJob{
		Req:          req,
		Backend:      backendID,
		AgentCommand: backend.Command,
		Args:         backend.Args,
		Model:        backend.UsableModel(),
		ThoughtLevel: backend.ThoughtLevel,
		Selection:    settingsSelection(h.settings),
		ViewPath:     genui.ViewPath(docPath),
		Now:          time.Now,
		Models:       models,
		AgentCache:   h.agentCache,
		BeforeWrite: func() error {
			return h.commitGenerationWrite(s, slot)
		},
	}
	if prepareJob != nil {
		prepareJob(&job)
	}
	go func() {
		defer h.runs.Done()
		defer close(result)
		err := acp.RunGeneration(runCtx, job, send)
		cancel()
		h.releaseGeneration(s, slot)
		result <- err
	}()
	return result
}

// settingsSelection records what the agent confirmed, so the next run uses the
// values that took effect rather than the ones asked for.
func settingsSelection(st *settingsStore) acp.SelectionUpdater {
	if st == nil {
		return nil
	}
	return st
}

func (h *Service) commitGenerationWrite(s *docSession, slot *generationSlot) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s.closed || slot.abandoned || s.genSlot != slot {
		return errAbandoned
	}
	now := s.currentPath()
	if now == "" || now != slot.path {
		return errPathChanged
	}
	return nil
}

func (h *Service) cancelGeneration(s *docSession) {
	h.mu.Lock()
	slot := s.genSlot
	if slot != nil {
		slot.abandoned = true
		if slot.cancel != nil {
			slot.cancel()
		}
	}
	h.mu.Unlock()
}

func (h *Service) releaseGeneration(s *docSession, slot *generationSlot) {
	h.mu.Lock()
	if s.genSlot == slot {
		s.genSlot = nil
		s.generating = false
	}
	w := s.ws
	win := s.win
	closed := s.closed
	h.mu.Unlock()
	if closed {
		return
	}
	h.emitWorkspace(w)
	if win != nil {
		_ = DocumentStateChanged.Emit(win, s.info())
	}
	h.refreshPathMenus()
}

func (h *Service) rejectPathChange(s *docSession) error {
	h.mu.Lock()
	busy := s.generating
	h.mu.Unlock()
	if busy {
		return errors.New(errGenBusy)
	}
	return nil
}

func (h *Service) WaitForGenerations(limit time.Duration) {
	done := make(chan struct{})
	go func() {
		h.runs.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(limit):
	}
}

func (h *Service) GetSettings() (Settings, error) {
	return h.settings.get(), nil
}

func (h *Service) SetSettings(p SettingsPatch) (Settings, error) {
	return h.settings.apply(p)
}
