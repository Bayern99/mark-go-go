import { useEffect, useMemo, useRef, useSyncExternalStore } from 'react'
import { createRoot } from 'react-dom/client'
import type { PluginContext } from '../../core/types'
import type { Disposable, ViewSnapshot } from '../../host/types'
import { loadSurface, ViewSurface, type LoadedSurface } from './a2ui'
import type { SourceActions } from './SourceFrame'
import { SourceStore } from './sources'
import { parseViewFile, type ViewFile } from './viewFile'
import './genui.css'

type GenUIState =
  | { kind: 'loading' }
  | { kind: 'none' }
  | { kind: 'error'; error: string }
  | {
      kind: 'view'
      view: ViewFile
      surface: Extract<LoadedSurface, { ok: true }>['surface']
      sources: SourceStore
      /** The view file's text: the same text is the same view. */
      text: string
      /** Counts loaded files, so each new file renders as a new surface. */
      revision: number
    }

/**
 * The view file as the host last reported it, applied to a surface and
 * checked against the document. Each new file gets a fresh surface and
 * source store; the previous ones are disposed.
 */
class GenUIStore {
  private state: GenUIState = { kind: 'loading' }
  private release: (() => void) | undefined
  private revision = 0
  private readonly listeners = new Set<() => void>()

  constructor(private readonly documentText: () => string) {}

  readonly subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  readonly getSnapshot = (): GenUIState => this.state

  setView(snap: ViewSnapshot): void {
    this.release?.()
    this.release = undefined
    this.state = this.load(snap)
    for (const listener of this.listeners) listener()
  }

  documentChanged(text: string): void {
    if (this.state.kind === 'view') this.state.sources.schedule(text)
  }

  dispose(): void {
    this.release?.()
    this.release = undefined
    this.listeners.clear()
  }

  private load(snap: ViewSnapshot): GenUIState {
    if (snap.error !== '') return { kind: 'error', error: snap.error }
    if (!snap.exists) return { kind: 'none' }
    const parsed = parseViewFile(snap.text)
    if (!parsed.ok) return { kind: 'error', error: parsed.error }
    const loaded = loadSurface(parsed.view.messages)
    if (!loaded.ok) return { kind: 'error', error: loaded.error }
    const sources = new SourceStore(parsed.view.sources, this.documentText())
    this.release = () => {
      loaded.dispose()
      sources.dispose()
    }
    return {
      kind: 'view',
      view: parsed.view,
      surface: loaded.surface,
      sources,
      text: snap.text,
      revision: ++this.revision,
    }
  }
}

/**
 * Where the reader was in the view. Kept outside the mounted view so that
 * leaving GenUI to edit the source and coming back lands at the same place;
 * a different view file starts at the top.
 */
export interface ScrollMemory {
  /** Text of the view file the position belongs to. */
  text: string | null
  top: number
}

/** How long a remembered position waits for the view's content to grow tall enough. */
const RESTORE_WINDOW_MS = 1000

/**
 * Puts the scroll position back for the same view file. A2UI renders its
 * tree after the first commit, so the position is applied as the content
 * grows, until it is reached or the window closes.
 */
function useScrollMemory(
  scroller: HTMLElement,
  memory: ScrollMemory,
  text: string | null,
  content: React.RefObject<HTMLElement | null>,
): void {
  useEffect(() => {
    if (text === null) return
    const target = memory.text === text ? memory.top : 0
    memory.text = text
    memory.top = target
    const apply = () => {
      scroller.scrollTop = target
      return Math.abs(scroller.scrollTop - target) < 1
    }
    if (target === 0 || apply()) return
    const element = content.current
    if (!element) return
    const observer = new ResizeObserver(() => {
      if (apply()) observer.disconnect()
    })
    observer.observe(element)
    const timer = window.setTimeout(() => observer.disconnect(), RESTORE_WINDOW_MS)
    return () => {
      observer.disconnect()
      window.clearTimeout(timer)
    }
  }, [scroller, memory, text, content])
}

function GenUIView({
  store,
  ctx,
  scroller,
  memory,
}: {
  store: GenUIStore
  ctx: PluginContext
  scroller: HTMLElement
  memory: ScrollMemory
}): React.ReactElement {
  const state = useSyncExternalStore(store.subscribe, store.getSnapshot)
  const content = useRef<HTMLDivElement>(null)
  useScrollMemory(scroller, memory, state.kind === 'view' ? state.text : null, content)

  const sources = state.kind === 'view' ? state.sources : null
  const actions = useMemo<SourceActions | null>(
    () =>
      sources && {
        store: sources,
        edit(ids) {
          const target = sources.target(ids)
          if (!target) return
          // A scroll the reader just made may not have reported its event yet.
          memory.top = scroller.scrollTop
          void ctx.commands.execute('editor.select', target.start, target.end)
        },
      },
    [sources, ctx, scroller, memory],
  )

  return (
    <div className="genui-view" data-testid="genui-view" data-state={state.kind} ref={content}>
      {state.kind === 'none' && (
        <div className="genui-empty">
          <p>No view yet.</p>
          <button type="button" className="genui-generate" disabled>
            Generate
          </button>
          <p className="genui-note">Generating is coming soon.</p>
        </div>
      )}

      {state.kind === 'error' && (
        <p className="genui-file-error" role="alert">
          {state.error}
        </p>
      )}

      {state.kind === 'view' && actions && (
        <>
          {(state.view.model || state.view.generatedAt) && (
            <p className="genui-meta">
              {[state.view.model, state.view.generatedAt].filter(Boolean).join(' · ')}
            </p>
          )}
          {/* A new file is a new surface: remount rather than diff across surfaces. */}
          <ViewSurface key={state.revision} surface={state.surface} sources={actions} />
        </>
      )}
    </div>
  )
}

/**
 * Shows the document's GenUI view in `el`, which is its scroll area, and
 * follows the view file and the document until disposed. `memory` outlives
 * the mount.
 */
export function mountGenUIView(el: HTMLElement, ctx: PluginContext, memory: ScrollMemory): Disposable {
  const store = new GenUIStore(() => ctx.documents.snapshot().text)
  const root = createRoot(el)
  root.render(<GenUIView store={store} ctx={ctx} scroller={el} memory={memory} />)
  const watch = ctx.viewFile.watch((snap) => store.setView(snap))
  const changes = ctx.documents.onDidChange((snapshot) => store.documentChanged(snapshot.text))
  // Read while mounted: once the view is removed, its element no longer scrolls.
  const remember = () => {
    memory.top = el.scrollTop
  }
  el.addEventListener('scroll', remember, { passive: true })
  return {
    dispose() {
      el.removeEventListener('scroll', remember)
      watch.dispose()
      changes.dispose()
      // Defer: React may be committing the host tree when the view is
      // removed, and a nested root must not unmount during that render.
      queueMicrotask(() => {
        root.unmount()
        store.dispose()
      })
    },
  }
}
