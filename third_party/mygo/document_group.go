package mygo

import (
	"slices"
)

// docGroup is the documents of one window: tab order and the active
// document. documentFor returns the active document; attaching a second
// document appends it instead of overwriting a single mapping.
type docGroup struct {
	win     *Window
	docs    []*docBase
	active  *docBase
	hooked  bool
	closing bool
}

// documentFor returns the active document of w, or nil.
func documentFor(w *Window) *docBase {
	if w == nil {
		return nil
	}
	docRegistry.Lock()
	defer docRegistry.Unlock()
	g := docRegistry.byWindow[w]
	if g == nil {
		return nil
	}
	return g.active
}

// documentsFor returns the documents of w in tab order.
func documentsFor(w *Window) []*docBase {
	if w == nil {
		return nil
	}
	docRegistry.Lock()
	defer docRegistry.Unlock()
	g := docRegistry.byWindow[w]
	if g == nil {
		return nil
	}
	return slices.Clone(g.docs)
}

func groupOf(w *Window) *docGroup {
	if w == nil {
		return nil
	}
	docRegistry.Lock()
	defer docRegistry.Unlock()
	return docRegistry.byWindow[w]
}

// ID is the document's stable identity, independent of its path and window.
func (b *docBase) ID() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.id
}

// ForWindowAll returns the documents w shows, in tab order.
func (ds *Documents[T]) ForWindowAll(w *Window) []*Document[T] {
	var docs []*Document[T]
	for _, b := range documentsFor(w) {
		if b.kind == ds.kind {
			docs = append(docs, b.ops.(*Document[T]))
		}
	}
	return docs
}

// SetActive makes d the active document of its window. Menus, the title
// and save dialogs then target d.
func (ds *Documents[T]) SetActive(d *Document[T]) {
	if d == nil {
		return
	}
	onMain(func() { d.setActive() })
}

// Attach shows d in w, leaving any previous window. d becomes active.
func (ds *Documents[T]) Attach(d *Document[T], w *Window) {
	if d == nil || w == nil {
		return
	}
	onMain(func() {
		d.attach(w)
		d.setActive()
	})
}

// Detach removes d from its window without closing d or destroying the window. Remaining documents stay; the last remaining becomes active.
func (ds *Documents[T]) Detach(d *Document[T]) {
	if d == nil {
		return
	}
	onMain(func() { d.detachFromGroup() })
}

// CloseDocument asks to save d when it must, then closes d without
// destroying a shared window. It returns ErrCanceled when the user cancels.
func (d *Document[T]) CloseDocument() error {
	return d.docBase.closeDocument()
}

// ConfirmClose asks to save d when it must, without closing it.
func (d *Document[T]) ConfirmClose() error {
	return d.docBase.confirmClose()
}

// CloseDocumentGroup asks to save the documents w shows, then closes w.
// A canceled confirmation leaves every document open.
func (ds *Documents[T]) CloseDocumentGroup(w *Window) error {
	if ds == nil {
		return ErrCanceled
	}
	if w == nil {
		return nil
	}
	g := groupOf(w)
	if g == nil {
		return onMainValue(func() error {
			if w.native == nil || w.IsDestroyed() || groupOf(w) != nil {
				return ErrCanceled
			}
			w.destroy()
			return nil
		})
	}
	if !g.beginClosing() {
		return ErrCanceled
	}
	defer g.endClosing()
	docs := documentsFor(w)
	discarded, err := g.confirmCloseDocuments(docs)
	if err != nil {
		return err
	}
	return onMainValue(func() error {
		return g.commitCloseDocuments(w, docs, discarded)
	})
}

// OnClosed is called on the main thread after the document is closed.
func (b *docBase) OnClosed(fn func()) (off func()) {
	return b.onClosed.add(fn, false)
}

// attach makes w a window of b. Main thread only. A second document
// appends to the window's group instead of overwriting a single mapping.
func (b *docBase) attach(w *Window) {
	if w == nil {
		return
	}
	b.mu.Lock()
	prev := b.window
	b.mu.Unlock()
	if prev != nil && prev != w {
		b.detachFromGroup()
	}
	docRegistry.Lock()
	if docRegistry.byWindow == nil {
		docRegistry.byWindow = map[*Window]*docGroup{}
	}
	g := docRegistry.byWindow[w]
	if g == nil {
		g = &docGroup{win: w}
		docRegistry.byWindow[w] = g
	}
	if slices.Index(g.docs, b) < 0 {
		g.docs = append(g.docs, b)
	}
	g.active = b
	docRegistry.Unlock()
	b.mu.Lock()
	b.window = w
	b.mu.Unlock()
	g.hook()
	b.applyChrome()
}

func (b *docBase) setActive() {
	w := b.Window()
	if w == nil || w.IsDestroyed() {
		return
	}
	docRegistry.Lock()
	g := docRegistry.byWindow[w]
	if g != nil {
		g.active = b
	}
	docRegistry.Unlock()
	b.applyChrome()
	refreshDocumentMenus()
}

func (b *docBase) applyChrome() {
	w := b.Window()
	if w == nil || w.IsDestroyed() {
		return
	}
	s := b.State()
	w.SetDocumentEdited(s.Edited)
	w.SetRepresentedFile(s.Path)
	w.SetTitle(documentTitle(s.Name))
	emitDocumentState(w, s)
}

func (b *docBase) isActive() bool {
	w := b.Window()
	return w != nil && documentFor(w) == b
}

func (b *docBase) focus() {
	onMain(func() {
		b.setActive()
		if w := b.parent(); w != nil {
			w.Show()
			w.Focus()
		}
	})
}

func (b *docBase) detachFromGroup() {
	b.mu.Lock()
	w := b.window
	b.mu.Unlock()
	if w == nil {
		return
	}
	docRegistry.Lock()
	g := docRegistry.byWindow[w]
	if g != nil {
		if i := slices.Index(g.docs, b); i >= 0 {
			g.docs = slices.Delete(g.docs, i, i+1)
		}
		if g.active == b {
			if n := len(g.docs); n > 0 {
				g.active = g.docs[n-1]
			} else {
				g.active = nil
			}
		}
	}
	docRegistry.Unlock()
	b.mu.Lock()
	b.window = nil
	b.mu.Unlock()
	if g := groupOf(w); g != nil && g.active != nil {
		g.active.applyChrome()
	}
}

func (g *docGroup) hook() {
	if g.hooked {
		return
	}
	g.hooked = true
	w := g.win
	w.OnClose(g.windowClosing)
	w.OnClosed(g.windowClosed)
	w.OnFocus(func() {
		for _, b := range documentsFor(w) {
			go b.checkDisk()
		}
		refreshDocumentMenus()
	})
	w.OnBlur(func() {
		for _, b := range documentsFor(w) {
			b.mu.Lock()
			now := b.edited && b.autosavesLocked()
			b.mu.Unlock()
			if now {
				go b.persist()
			}
		}
		refreshDocumentMenus()
	})
	if p := w.Page(); p != nil {
		p.OnPageTitleUpdated(func(e *TitleEvent) { e.PreventDefault() })
	}
}

func (g *docGroup) windowClosing(e *CloseEvent) {
	if e.DefaultPrevented() || App.quitting {
		return
	}
	if g.isClosing() {
		e.PreventDefault()
		return
	}
	seen := map[*docKind]struct{}{}
	var kinds []*docKind
	for _, b := range documentsFor(g.win) {
		k := b.kind
		if k == nil {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		kinds = append(kinds, k)
	}
	if len(kinds) == 0 {
		if k := defaultDocKind(); k != nil {
			kinds = []*docKind{k}
		}
	}
	for _, k := range kinds {
		if k.beforeClose != nil && k.beforeClose(g.win) {
			e.PreventDefault()
			return
		}
	}
	docs := documentsFor(g.win)
	need := false
	for _, b := range docs {
		b.mu.Lock()
		edited := b.edited
		b.mu.Unlock()
		if edited {
			need = true
			break
		}
	}
	if !need {
		return
	}
	e.PreventDefault()
	if !g.beginClosing() {
		return
	}
	go func() {
		discarded, err := g.confirmCloseDocuments(docs)
		onMain(func() {
			if err == nil {
				err = g.commitCloseDocuments(e.Window, docs, discarded)
			}
			g.endClosing()
		})
	}()
}

func (g *docGroup) isClosing() bool {
	docRegistry.Lock()
	defer docRegistry.Unlock()
	return g.closing
}

func (g *docGroup) beginClosing() bool {
	docRegistry.Lock()
	defer docRegistry.Unlock()
	if g.closing {
		return false
	}
	g.closing = true
	return true
}

func (g *docGroup) endClosing() {
	docRegistry.Lock()
	g.closing = false
	docRegistry.Unlock()
}

func (g *docGroup) confirmCloseDocuments(docs []*docBase) ([]*docBase, error) {
	return confirmCloseGroup(docs, func(b *docBase) (bool, error) {
		return b.confirmCloseDeferred()
	})
}

func (g *docGroup) commitCloseDocuments(w *Window, docs, discarded []*docBase) error {
	if w == nil || w.native == nil || w.IsDestroyed() || groupOf(w) != g || !slices.Equal(docs, documentsFor(w)) {
		return ErrCanceled
	}
	for _, b := range discarded {
		b.mu.Lock()
		b.discarded = true
		b.mu.Unlock()
	}
	w.destroy()
	return nil
}

func confirmCloseGroup(docs []*docBase, confirm func(*docBase) (bool, error)) ([]*docBase, error) {
	var discarded []*docBase
	for _, b := range docs {
		discard, err := confirm(b)
		if err != nil {
			return nil, err
		}
		if discard {
			discarded = append(discarded, b)
		}
	}
	return discarded, nil
}

func (g *docGroup) windowClosed() {
	docs := documentsFor(g.win)
	for _, b := range docs {
		b.finishClose(true)
	}
	docRegistry.Lock()
	delete(docRegistry.byWindow, g.win)
	docRegistry.Unlock()
}

func (b *docBase) confirmClose() error {
	discard, err := b.confirmCloseDeferred()
	if err != nil {
		return err
	}
	if discard {
		b.discard()
	}
	return nil
}

// confirmCloseDeferred records a discard decision for a group close to
// commit only after every document has confirmed.
func (b *docBase) confirmCloseDeferred() (discard bool, err error) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return false, nil
	}
	if b.discarded {
		b.mu.Unlock()
		return false, nil
	}
	edited, autosaves, path := b.edited, b.autosavesLocked(), b.path
	b.mu.Unlock()
	if !edited {
		return false, nil
	}
	if autosaves {
		if _, err := b.write(path, writeAutosave); err == nil {
			return false, nil
		}
	}
	ok, discard := b.askToSaveDecision()
	if !ok {
		return false, ErrCanceled
	}
	return discard, nil
}

func (b *docBase) closeDocument() error {
	if err := b.confirmClose(); err != nil {
		return err
	}
	onMain(func() { b.finishClose(false) })
	return nil
}

// finishClose unregisters b. keepIfEdited writes a recovery copy when the
// window vanished with unsaved changes; an explicit close does not.
func (b *docBase) finishClose(keepIfEdited bool) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	w := b.window
	b.closed = true
	b.window = nil
	keep := b.edited && !b.discarded
	if b.timer != nil {
		b.timer.Stop()
	}
	b.mu.Unlock()
	docRegistry.Lock()
	if g := docRegistry.byWindow[w]; g != nil {
		if i := slices.Index(g.docs, b); i >= 0 {
			g.docs = slices.Delete(g.docs, i, i+1)
		}
		if g.active == b {
			if n := len(g.docs); n > 0 {
				g.active = g.docs[n-1]
			} else {
				g.active = nil
			}
		}
	}
	docRegistry.Unlock()
	k := b.kind
	k.mu.Lock()
	if i := slices.Index(k.docs, b); i >= 0 {
		k.docs = slices.Delete(k.docs, i, i+1)
	}
	k.mu.Unlock()
	b.watch()
	if keepIfEdited && keep {
		docWork.Go(b.backup)
	} else {
		docWork.Go(func() { k.recovery.remove(b) })
	}
	fire(&b.onClosed)
	if g := groupOf(w); g != nil && g.active != nil && !w.IsDestroyed() {
		g.active.applyChrome()
	}
	refreshDocumentMenus()
}
