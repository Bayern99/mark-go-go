package feynman

import (
	"context"
	"errors"
	"github.com/Bayern99/mark-go-go/internal/desktop"
	"slices"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/egoist/mygo"
)

var FocusRequested = desktop.Event[ResearchFocusEvent]{}

func (h *ResearchManager) WatchFocus(ctx context.Context, root string) error {
	w, err := h.workspace(ctx, root)
	if err != nil {
		return err
	}
	h.ensureFocusWatch(w)
	return nil
}

func (h *ResearchManager) ensureFocusWatch(w WorkspaceRef) {
	if w.Window == nil {
		return
	}
	root := fsutil.CanonicalPath(w.Root)
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	if h.focusWatchers == nil {
		h.focusWatchers = make(map[*mygo.Window]*focusWatcher)
	}
	if existing := h.focusWatchers[w.Window]; existing != nil {
		existing.root = root
		h.mu.Unlock()
		return
	}
	fw := &focusWatcher{root: root}
	h.focusWatchers[w.Window] = fw
	h.mu.Unlock()

	win := w.Window
	offFocus := win.OnFocus(func() {
		h.mu.Lock()
		closed := h.closed
		current := h.focusWatchers[win]
		r := ""
		if current != nil {
			r = current.root
		}
		h.mu.Unlock()
		if closed || r == "" {
			return
		}
		go func() {
			_ = FocusRequested.Emit(win, ResearchFocusEvent{Root: r})
		}()
	})
	offClosed := win.OnClosed(func() {
		h.mu.Lock()
		delete(h.focusWatchers, win)
		h.mu.Unlock()
	})
	offAll := func() {
		offFocus()
		offClosed()
	}

	h.mu.Lock()
	if h.closed || h.focusWatchers[win] != fw {
		h.mu.Unlock()
		offAll()
		return
	}
	fw.off = offAll
	h.mu.Unlock()
}

func (h *ResearchManager) RefreshModels(ctx context.Context, root string) ([]ResearchModel, error) {
	root = fsutil.CanonicalPath(root)
	h.mu.Lock()
	s, err := h.activeLocked(ctx, root)
	if err != nil {
		h.mu.Unlock()
		return nil, err
	}
	select {
	case <-s.ready:
		if s.startErr != nil {
			h.mu.Unlock()
			return nil, s.startErr
		}
	default:
		h.mu.Unlock()
		return nil, errors.New("Feynman is connecting.")
	}
	for s.busy {
		turn := s.turn
		h.mu.Unlock()
		select {
		case <-turn:
		case <-s.stopped:
			return nil, errors.New("Feynman disconnected before the model refresh.")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		h.mu.Lock()
		active, err := h.activeLocked(ctx, root)
		if err != nil || active != s {
			h.mu.Unlock()
			return nil, errors.New("Feynman session changed before the model refresh.")
		}
	}
	if s.configuring || s.switching {
		modelsCopy := slices.Clone(s.models)
		h.mu.Unlock()
		return modelsCopy, nil
	}
	h.mu.Unlock()

	var modelsResp struct {
		Models []ResearchModel `json:"models"`
	}
	if err := s.rpc.Call("get_available_models", nil, &modelsResp); err != nil {
		return nil, err
	}
	var state ResearchSessionState
	hasState := s.rpc.Call("get_state", nil, &state) == nil
	var levelsResp struct {
		Levels []string `json:"levels"`
	}
	_ = s.rpc.Call("get_available_thinking_levels", nil, &levelsResp)

	h.mu.Lock()
	s.models = modelsResp.Models
	if hasState {
		s.state = state
		if state.ThinkingLevel != "" {
			s.thinkingLevel = state.ThinkingLevel
		}
	} else {
		state = s.state
	}
	if len(levelsResp.Levels) > 0 {
		s.thinkingLevels = levelsResp.Levels
	}
	s.imagePrompts = modelSupportsImages(state.Model, s.models)
	cfg := buildResearchConfig(state, s.models, s.thinkingLevels)
	s.config = cfg
	modelsCopy := slices.Clone(s.models)
	levelsCopy := slices.Clone(s.thinkingLevels)
	h.mu.Unlock()

	s.emit(ResearchUpdate{
		Kind:           "config",
		Config:         cfg,
		Models:         modelsCopy,
		ThinkingLevels: levelsCopy,
		ThinkingLevel:  state.ThinkingLevel,
		State:          &state,
	})
	return modelsCopy, nil
}
