package nativeui

import (
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

// LoadResearchDocument returns the document currently published by the
// workspace service, including its stable identity and revision.
func (w *Window) LoadResearchDocument(id string) (workspace.Snapshot, error) {
	return w.host.Load(w.ctx, id)
}

// OpenResearchDocument opens a published research file in this window and
// returns its current workspace snapshot after the native shell refreshes.
func (w *Window) OpenResearchDocument(path string, done func(workspace.Snapshot, error)) {
	go func() {
		var snapshot workspace.Snapshot
		document, err := w.host.Open(w.ctx, path)
		if err == nil {
			snapshot, err = w.host.Load(w.ctx, document.ID)
		}
		w.win.Update(func() {
			if w.win.IsDestroyed() {
				return
			}
			if err == nil {
				w.refresh()
			}
			if done != nil {
				done(snapshot, err)
			}
		})
	}()
}
