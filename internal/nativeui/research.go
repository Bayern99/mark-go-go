package nativeui

import (
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

func (w *Window) LoadResearchDocument(id string) (workspace.Snapshot, error) {
	return w.host.Load(w.ctx, id)
}

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
