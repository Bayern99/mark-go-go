import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react'
import { createRoot } from 'react-dom/client'
import type { PluginContext } from '../../core/types'
import { askForNote } from '../../editor/noteDialog'
import type { Disposable, GenerateFailure, GenerateFailureKind, GenerationSettings, ViewSnapshot } from '../../host/types'
import { loadSurface, ViewSurface, type LoadedSurface } from './a2ui'
import { GenerationController, type GenerationRun, type GenerationState } from './generation'
import type { SourceActions } from './SourceFrame'
import { SourceStore } from './sources'
import { parseViewFile, type ViewFile } from './viewFile'
import './genui.css'

/** `canUndo`: the view the last generation replaced is kept, to go back to once. */
type GenUIState =
  | { kind: 'loading' }
  | { kind: 'none'; canUndo: boolean }
  | { kind: 'error'; error: string; canUndo: boolean }
  | {
      kind: 'view'
      view: ViewFile
      surface: Extract<LoadedSurface, { ok: true }>['surface']
      sources: SourceStore
      /** The view file's text: the same text is the same view. */
      text: string
      /** Counts loaded files, so each new file renders as a new surface. */
      revision: number
      canUndo: boolean
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
    const { canUndo } = snap
    if (snap.error !== '') return { kind: 'error', error: snap.error, canUndo }
    if (!snap.exists) return { kind: 'none', canUndo }
    const parsed = parseViewFile(snap.text)
    if (!parsed.ok) return { kind: 'error', error: parsed.error, canUndo }
    const loaded = loadSurface(parsed.view.messages)
    if (!loaded.ok) return { kind: 'error', error: loaded.error, canUndo }
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
      canUndo,
    }
  }
}

const noSubscription = () => () => {}
const noRevision = () => 0

/**
 * The view's components whose sources changed in the document, in the
 * surface's order: what "Regenerate changed" replaces.
 */
function useChangedComponents(state: GenUIState): string[] {
  const surface = state.kind === 'view' ? state.surface : null
  const sources = state.kind === 'view' ? state.sources : null
  const revision = useSyncExternalStore(sources?.subscribe ?? noSubscription, sources?.getRevision ?? noRevision)
  return useMemo(() => {
    if (!surface || !sources) return []
    const changed: string[] = []
    for (const [id, component] of surface.componentsModel.entries) {
      const ids: unknown = component.properties.sources
      if (Array.isArray(ids) && sources.status(ids.filter((s): s is string => typeof s === 'string')) === 'changed') {
        changed.push(id)
      }
    }
    return changed
    // `revision` changes when the statuses may have.
  }, [surface, sources, revision])
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

/**
 * What outlives the GenUI view's mounts: where the reader was in the view,
 * the generation of the window, and the request being written.
 */
export interface GenUIMemory extends ScrollMemory {
  generation: GenerationController | undefined
  draft: string | undefined
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

function useDocumentPath(ctx: PluginContext): string {
  const subscribe = useMemo(
    () => (listener: () => void) => {
      const subscription = ctx.documents.onDidChange(listener)
      return () => subscription.dispose()
    },
    [ctx],
  )
  return useSyncExternalStore(subscribe, () => ctx.documents.snapshot().path)
}

function GenUIView({
  store,
  ctx,
  scroller,
  memory,
  generation,
}: {
  store: GenUIStore
  ctx: PluginContext
  scroller: HTMLElement
  memory: GenUIMemory
  generation: GenerationController
}): React.ReactElement {
  const state = useSyncExternalStore(store.subscribe, store.getSnapshot)
  const run = useSyncExternalStore(generation.subscribe, generation.getSnapshot)
  const path = useDocumentPath(ctx)
  const content = useRef<HTMLDivElement>(null)
  const running = run.kind === 'running'
  useScrollMemory(scroller, memory, state.kind === 'view' && !running ? state.text : null, content)

  const sources = state.kind === 'view' ? state.sources : null
  const actions = useMemo<SourceActions | null>(
    () =>
      sources && {
        store: sources,
        edit(ids, text) {
          const target = sources.target(ids, text)
          if (!target) return
          // A scroll the reader just made may not have reported its event yet.
          memory.top = scroller.scrollTop
          void ctx.commands.execute('editor.select', target.start, target.end)
        },
        async addNote(ids) {
          const text = await askForNote()
          if (text === null) return true
          // Taken after the dialog: the document may have changed behind it.
          const target = sources.target(ids)
          return target !== undefined && (await ctx.commands.execute('editor.addNote', target.end, text)) === true
        },
      },
    [sources, ctx, scroller, memory],
  )

  const savedRequest = state.kind === 'view' ? (state.view.request ?? '') : ''
  const changed = useChangedComponents(state)

  return (
    <div className="genui-view" data-testid="genui-view" data-state={state.kind} ref={content}>
      {state.kind === 'loading' ? null : (
        <GeneratePanel
          key={state.kind === 'view' ? state.text : state.kind}
          generation={generation}
          run={run}
          memory={memory}
          savedRequest={savedRequest}
          untitled={path === ''}
          hasView={state.kind === 'view'}
          changed={changed}
          undo={state.canUndo ? () => ctx.viewFile.undo() : null}
        />
      )}

      {running ? <GenerationProgress run={run} /> : null}

      {!running && state.kind === 'none' && <p className="genui-empty">No view yet.</p>}

      {!running && state.kind === 'error' && (
        <p className="genui-file-error" role="alert">
          {state.error}
        </p>
      )}

      {!running && state.kind === 'view' && actions && (
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
 * Generate, with the reader's request and the agent's settings. The
 * request starts as the one the current view was made with. With a view,
 * "Regenerate changed" replaces only the components whose sources changed
 * and "Regenerate all" makes a new one; `undo`, when the view the last
 * generation replaced is kept, goes back to it.
 */
function GeneratePanel({
  generation,
  run,
  memory,
  savedRequest,
  untitled,
  hasView,
  changed,
  undo,
}: {
  generation: GenerationController
  run: GenerationState
  memory: GenUIMemory
  savedRequest: string
  untitled: boolean
  hasView: boolean
  changed: string[]
  undo: (() => Promise<void>) | null
}): React.ReactElement {
  const [request, setRequest] = useState(memory.draft ?? savedRequest)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [undoError, setUndoError] = useState('')
  const running = run.kind === 'running'
  const change = (next: string) => {
    memory.draft = next
    setRequest(next)
  }
  const start = (run: GenerationRun) => {
    memory.draft = undefined
    setUndoError('')
    void generation.generate(run)
  }
  const goBack = (undo: () => Promise<void>) => {
    setUndoError('')
    undo().catch((error: unknown) => setUndoError(error instanceof Error ? error.message : String(error)))
  }

  return (
    <section className="genui-generate-panel" aria-label="Generate a view">
      <label className="genui-request">
        <span>What should the view focus on? (optional)</span>
        <textarea
          value={request}
          rows={2}
          disabled={running}
          placeholder="For example: compare the options in a table"
          onChange={(event) => change(event.target.value)}
        />
      </label>
      <div className="genui-generate-actions">
        {running ? (
          <button type="button" className="genui-cancel" onClick={() => generation.cancel()}>
            Cancel
          </button>
        ) : hasView ? (
          <>
            <button
              type="button"
              className="genui-generate"
              disabled={untitled || changed.length === 0}
              title={changed.length === 0 ? 'No component’s source has changed.' : undefined}
              onClick={() => start({ mode: 'update', request, replaceIds: changed })}
            >
              {changed.length === 0 ? 'Regenerate changed' : `Regenerate changed (${changed.length})`}
            </button>
            <button
              type="button"
              className="genui-generate-all"
              disabled={untitled}
              onClick={() => start({ mode: 'full', request })}
            >
              Regenerate all
            </button>
          </>
        ) : (
          <button
            type="button"
            className="genui-generate"
            disabled={untitled}
            onClick={() => start({ mode: 'full', request })}
          >
            Generate
          </button>
        )}
        {undo ? (
          <button type="button" className="genui-undo" disabled={running} onClick={() => goBack(undo)}>
            Undo last generation
          </button>
        ) : null}
        <button
          type="button"
          className="genui-settings-toggle"
          aria-expanded={settingsOpen}
          disabled={running}
          onClick={() => setSettingsOpen((open) => !open)}
        >
          Agent settings
        </button>
      </div>
      {untitled ? <p className="genui-note">Save the document first: the view is stored next to it.</p> : null}
      {undoError ? (
        <p className="genui-file-error" role="alert">
          {undoError}
        </p>
      ) : null}
      {run.kind === 'failed' ? (
        <GenerationFailure failure={run.failure} generation={generation} openSettings={() => setSettingsOpen(true)} />
      ) : null}
      {settingsOpen && !running ? <SettingsForm generation={generation} onDone={() => setSettingsOpen(false)} /> : null}
    </section>
  )
}

/** The kinds whose fix is in the agent settings. */
const SETTINGS_FIXES: Partial<Record<GenerateFailureKind, true>> = { 'agent-not-found': true, 'model-invalid': true }

/** Why the last run failed, what to do about it, and the agent's own words. */
function GenerationFailure({
  failure,
  generation,
  openSettings,
}: {
  failure: GenerateFailure
  generation: GenerationController
  openSettings: () => void
}): React.ReactElement {
  return (
    <div className="genui-file-error genui-run-error" role="alert">
      <p>{failure.message}</p>
      {failure.hint ? <p className="genui-run-hint">{failure.hint}</p> : null}
      {failure.models?.length ? (
        <>
          <p>Models it offers:</p>
          <ul className="genui-run-models">
            {failure.models.map((model) => (
              <li key={model}>
                <code>{model}</code>
              </li>
            ))}
          </ul>
        </>
      ) : null}
      {failure.detail ? (
        <figure className="genui-run-detail">
          <figcaption>The agent said:</figcaption>
          <pre>{failure.detail}</pre>
        </figure>
      ) : null}
      <div className="genui-generate-actions">
        <button type="button" onClick={() => void generation.retry()}>
          Retry
        </button>
        {SETTINGS_FIXES[failure.kind] ? (
          <button type="button" onClick={openSettings}>
            Open agent settings
          </button>
        ) : null}
        <button type="button" onClick={() => generation.dismissError()}>
          Dismiss
        </button>
      </div>
    </div>
  )
}

function SettingsForm({
  generation,
  onDone,
}: {
  generation: GenerationController
  onDone: () => void
}): React.ReactElement {
  const [settings, setSettings] = useState<GenerationSettings | null>(null)
  const [error, setError] = useState<string | null>(null)
  useEffect(() => {
    let live = true
    generation.getSettings().then(
      (loaded) => live && setSettings(loaded),
      (reason: unknown) => live && setError(String(reason)),
    )
    return () => {
      live = false
    }
  }, [generation])

  if (!settings) {
    return <p className="genui-note">{error ?? 'Loading settings…'}</p>
  }
  const save = (event: React.FormEvent) => {
    event.preventDefault()
    generation.setSettings(settings).then(onDone, (reason: unknown) => setError(String(reason)))
  }
  return (
    <form className="genui-settings" onSubmit={save}>
      <label>
        <span>Model</span>
        <input
          value={settings.model}
          placeholder="The agent's own choice"
          onChange={(event) => setSettings({ ...settings, model: event.target.value })}
        />
      </label>
      <label>
        <span>Agent command</span>
        <input
          value={settings.agentCommand}
          placeholder="agy"
          onChange={(event) => setSettings({ ...settings, agentCommand: event.target.value })}
        />
      </label>
      <p className="genui-note">Runs as “{settings.agentCommand || 'agy'} acp” in a folder that holds only a copy of the document.</p>
      {error ? (
        <p className="genui-file-error" role="alert">
          {error}
        </p>
      ) : null}
      <div className="genui-generate-actions">
        <button type="submit">Save</button>
        <button type="button" onClick={onDone}>
          Close
        </button>
      </div>
    </form>
  )
}

/** The steps so far and a skeleton where the view will be. */
function GenerationProgress({ run }: { run: Extract<GenerationState, { kind: 'running' }> }): React.ReactElement {
  const latest = run.progress.at(-1)
  return (
    <div className="genui-progress" data-testid="genui-progress">
      <p className="genui-progress-now" role="status" aria-live="polite">
        {latest ? latest.text : 'Starting the agent'}…
      </p>
      <ol className="genui-progress-steps">
        {run.progress.map((step, index) => (
          <li key={index} data-phase={step.phase}>
            {step.text}
          </li>
        ))}
      </ol>
      <div className="genui-skeleton" aria-hidden="true">
        <div className="genui-skeleton-line" />
        <div className="genui-skeleton-row">
          <div className="genui-skeleton-block" />
          <div className="genui-skeleton-block" />
        </div>
        <div className="genui-skeleton-block genui-skeleton-wide" />
      </div>
    </div>
  )
}

/**
 * Shows the document's GenUI view in `el`, which is its scroll area, and
 * follows the view file and the document until disposed. `memory` outlives
 * the mount.
 */
export function mountGenUIView(el: HTMLElement, ctx: PluginContext, memory: GenUIMemory): Disposable {
  const store = new GenUIStore(() => ctx.documents.snapshot().text)
  memory.generation ??= new GenerationController(ctx)
  const generation = memory.generation
  const root = createRoot(el)
  root.render(<GenUIView store={store} ctx={ctx} scroller={el} memory={memory} generation={generation} />)
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
