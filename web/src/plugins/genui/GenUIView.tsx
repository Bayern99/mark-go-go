import { useSyncExternalStore } from 'react'
import { createRoot } from 'react-dom/client'
import type { PluginContext } from '../../core/types'
import type { Disposable, ViewSnapshot } from '../../host/types'
import { loadSurface, ViewSurface, type LoadedSurface } from './a2ui'
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
      /** Counts loaded files, so each new file renders as a new surface. */
      revision: number
    }

/**
 * The view file as the host last reported it, applied to a surface. Each new
 * file gets a fresh surface; the previous one is disposed.
 */
class GenUIStore {
  private state: GenUIState = { kind: 'loading' }
  private disposeSurface: (() => void) | undefined
  private revision = 0
  private readonly listeners = new Set<() => void>()

  readonly subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  readonly getSnapshot = (): GenUIState => this.state

  setView(snap: ViewSnapshot): void {
    this.release()
    this.state = this.load(snap)
    for (const listener of this.listeners) listener()
  }

  dispose(): void {
    this.release()
    this.listeners.clear()
  }

  private load(snap: ViewSnapshot): GenUIState {
    if (snap.error !== '') return { kind: 'error', error: snap.error }
    if (!snap.exists) return { kind: 'none' }
    const parsed = parseViewFile(snap.text)
    if (!parsed.ok) return { kind: 'error', error: parsed.error }
    const loaded = loadSurface(parsed.view.messages)
    if (!loaded.ok) return { kind: 'error', error: loaded.error }
    this.disposeSurface = loaded.dispose
    return { kind: 'view', view: parsed.view, surface: loaded.surface, revision: ++this.revision }
  }

  private release(): void {
    this.disposeSurface?.()
    this.disposeSurface = undefined
  }
}

function GenUIView({ store }: { store: GenUIStore }): React.ReactElement {
  const state = useSyncExternalStore(store.subscribe, store.getSnapshot)
  return (
    <div className="genui-view" data-testid="genui-view" data-state={state.kind}>
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

      {state.kind === 'view' && (
        <>
          {(state.view.model || state.view.generatedAt) && (
            <p className="genui-meta">
              {[state.view.model, state.view.generatedAt].filter(Boolean).join(' · ')}
            </p>
          )}
          {/* A new file is a new surface: remount rather than diff across surfaces. */}
          <ViewSurface key={state.revision} surface={state.surface} />
        </>
      )}
    </div>
  )
}

/** Shows the document's GenUI view in `el` and follows the view file until disposed. */
export function mountGenUIView(el: HTMLElement, ctx: PluginContext): Disposable {
  const store = new GenUIStore()
  const root = createRoot(el)
  root.render(<GenUIView store={store} />)
  const watch = ctx.viewFile.watch((snap) => store.setView(snap))
  return {
    dispose() {
      watch.dispose()
      // Defer: React may be committing the host tree when the view is
      // removed, and a nested root must not unmount during that render.
      queueMicrotask(() => {
        root.unmount()
        store.dispose()
      })
    },
  }
}
