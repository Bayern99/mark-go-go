import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react'
import { createRoot } from 'react-dom/client'
import type { PluginContext, ViewStatusDot } from '../../core/types'
import { autoGrowTextarea } from '../../editor/noteDialog'
import type {
  Disposable,
  GenerateFailure,
  GenerationSettings,
  SettingsPatch,
  ViewSnapshot,
} from '../../host/types'
import { loadSurface, ViewSurface, type LoadedSurface } from './a2ui'
import { GenerationController, type GenerationRun, type GenerationState } from './generation'
import { ModelMenu } from './ModelMenu'
import type { SourceActions } from './SourceFrame'
import { SourceStore } from './sources'
import { parseViewFile, type ViewFile } from './viewFile'
import { ReadingStore } from './reading'
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

interface ChangedComponentItem {
  id: string
  title: string
  subtitle: string
  status: 'changed' | 'missing'
}

function getChangedItems(state: GenUIState): ChangedComponentItem[] {
  if (state.kind !== 'view') return []
  const { surface, sources } = state
  const items: ChangedComponentItem[] = []
  for (const [id, component] of surface.componentsModel.entries) {
    const rawIds: unknown = component.properties.sources
    if (!Array.isArray(rawIds)) continue
    const sourceIds = rawIds.filter((s): s is string => typeof s === 'string')
    if (sourceIds.length === 0) continue
    const status = sources.status(sourceIds)
    if (status !== 'changed' && status !== 'missing') continue
    const rawTitle = component.properties.title
    const title = typeof rawTitle === 'string' && rawTitle.trim() !== '' ? rawTitle : id
    const headings = sources.headings(sourceIds)
    const subtitle =
      headings.length > 0
        ? `From ${headings.join(', ')}`
        : status === 'missing'
          ? 'Source not found'
          : 'Source changed'
    items.push({ id, title, subtitle, status })
  }
  return items
}

/**
 * The view's components whose sources are out of date in the document, in the
 * surface's order: what the right panel lists and what "Regenerate changed" replaces.
 */
function useChangedComponents(state: GenUIState): ChangedComponentItem[] {
  const surface = state.kind === 'view' ? state.surface : null
  const sources = state.kind === 'view' ? state.sources : null
  const revision = useSyncExternalStore(sources?.subscribe ?? noSubscription, sources?.getRevision ?? noRevision)
  return useMemo(() => {
    if (!surface || !sources) return []
    return getChangedItems(state)
    // `revision` changes when the statuses may have.
  }, [state, surface, sources, revision])
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

const DOT_DANGER: ViewStatusDot = { tone: 'danger', title: 'Generation failed' }
const DOT_VIEW_ERROR: ViewStatusDot = { tone: 'danger', title: 'GenUI view error' }
const DOT_WARN: ViewStatusDot = { tone: 'warn', title: 'A source changed' }

function LoadingSkeleton(): React.ReactElement {
  return (
    <div className="genui-skeleton" aria-hidden="true">
      <div className="genui-skeleton-line" />
      <div className="genui-skeleton-row">
        <div className="genui-skeleton-block" />
        <div className="genui-skeleton-block" />
      </div>
      <div className="genui-skeleton-block genui-skeleton-wide" />
    </div>
  )
}

function InitialLoading(): React.ReactElement {
  return (
    <div className="gen-empty genui-empty" data-testid="genui-loading" role="status" aria-live="polite">
      <h2>Loading the GenUI view</h2>
      <p>Reading the view file for this document.</p>
      <LoadingSkeleton />
    </div>
  )
}

/**
 * Shared state between the center GenUI view (`genui`) and the right-column
 * GenUI management panel (`genui.panel`).
 */
export class GenUISession {
  readonly store: GenUIStore
  readonly generation: GenerationController
  private activeMounts = 0
  /** This document's mounted center view scroll area; null while it is not shown. */
  private viewElement: HTMLElement | null = null
  private watchSub: Disposable | undefined
  private readonly docSub: Disposable
  private readonly unsubStore: () => void
  private readonly unsubGen: () => void
  private unsubSources: (() => void) | undefined
  private highlightedId: string | null = null
  private readonly highlightListeners = new Set<() => void>()
  private statusDot: ViewStatusDot = null

  constructor(
    private readonly ctx: PluginContext,
    private readonly memory: GenUIMemory,
  ) {
    this.store = new GenUIStore(() => ctx.documents.snapshot().text)
    this.generation = memory.generation ?? new GenerationController(ctx)
    memory.generation = this.generation

    this.docSub = ctx.documents.onDidChange((snapshot) => this.store.documentChanged(snapshot.text))

    this.unsubStore = this.store.subscribe(() => {
      this.unsubSources?.()
      this.unsubSources = undefined
      const state = this.store.getSnapshot()
      if (state.kind === 'view') {
        this.unsubSources = state.sources.subscribe(() => this.syncStatusDot())
      }
      this.syncStatusDot()
    })

    this.unsubGen = this.generation.subscribe(() => {
      this.syncStatusDot()
    })
  }

  retainWatch(): Disposable {
    this.activeMounts += 1
    if (this.activeMounts === 1) {
      this.watchSub = this.ctx.viewFile.watch((snap) => this.store.setView(snap))
    }
    let released = false
    return {
      dispose: () => {
        if (released) return
        released = true
        this.activeMounts -= 1
        if (this.activeMounts === 0) {
          this.watchSub?.dispose()
          this.watchSub = undefined
        }
      },
    }
  }

  readonly subscribeHighlight = (listener: () => void): (() => void) => {
    this.highlightListeners.add(listener)
    return () => {
      this.highlightListeners.delete(listener)
    }
  }

  readonly getHighlightedId = (): string | null => this.highlightedId

  highlightComponent(id: string): void {
    this.highlightedId = id
    for (const listener of this.highlightListeners) listener()
    void this.ctx.commands.execute('centerPane.show', 'genui').catch(() => {})
    const scrollToCard = () => {
      // Another document's GenUI view stays mounted while its tab is hidden and
      // can carry the same component ids; only this document's center view may
      // be highlighted or scrolled.
      const scope = this.viewElement
      if (!scope) return false
      const cards = scope.querySelectorAll<HTMLElement>('[data-component-id]')
      for (const el of cards) el.classList.remove('mgo-jump')
      const card = Array.from(cards).find((el) => el.getAttribute('data-component-id') === id)
      if (!card) return false
      card.classList.add('mgo-jump')
      card.scrollIntoView({ block: 'center', behavior: 'auto' })
      return true
    }
    if (!scrollToCard()) {
      requestAnimationFrame(() => {
        scrollToCard()
      })
    }
  }

  /**
   * Tracks the center view while it is mounted. The session, not the whole
   * document, is the boundary for highlight and scroll.
   */
  retainView(el: HTMLElement): Disposable {
    this.viewElement = el
    let released = false
    return {
      dispose: () => {
        if (released) return
        released = true
        if (this.viewElement === el) this.viewElement = null
      },
    }
  }

  getStatusDot(): ViewStatusDot {
    return this.statusDot
  }

  private syncStatusDot(): void {
    const next = this.computeStatusDot()
    if (next === this.statusDot) return
    this.statusDot = next
    for (const listener of this.memory.statusListeners) listener()
  }

  private computeStatusDot(): ViewStatusDot {
    const run = this.generation.getSnapshot()
    if (run.kind === 'failed') return DOT_DANGER
    const state = this.store.getSnapshot()
    if (state.kind === 'error') return DOT_VIEW_ERROR
    if (state.kind === 'view' && getChangedItems(state).length > 0) {
      return DOT_WARN
    }
    return null
  }

  dispose(): void {
    this.watchSub?.dispose()
    this.watchSub = undefined
    this.docSub.dispose()
    this.unsubSources?.()
    this.unsubSources = undefined
    this.unsubStore()
    this.unsubGen()
    this.store.dispose()
    this.highlightListeners.clear()
  }
}

/**
 * What outlives the GenUI view's mounts: where the reader was in the view,
 * this document's generation, the request being written, and the shared session.
 */
export interface GenUIMemory extends ScrollMemory {
  generation: GenerationController | undefined
  draft: string | undefined
  session: GenUISession | undefined
  reading: ReadingStore
  statusListeners: Set<() => void>
}

function ensureSession(ctx: PluginContext, memory: GenUIMemory): GenUISession {
  memory.reading ??= new ReadingStore()
  memory.session ??= new GenUISession(ctx, memory)
  return memory.session
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
  session,
  ctx,
  scroller,
  memory,
}: {
  session: GenUISession
  ctx: PluginContext
  scroller: HTMLElement
  memory: GenUIMemory
}): React.ReactElement {
  const state = useSyncExternalStore(session.store.subscribe, session.store.getSnapshot)
  const run = useSyncExternalStore(session.generation.subscribe, session.generation.getSnapshot)
  const highlightedId = useSyncExternalStore(session.subscribeHighlight, session.getHighlightedId)
  const content = useRef<HTMLDivElement>(null)
  const running = run.kind === 'running'
  const replacing =
    running && run.run.mode === 'update' ? new Set(run.run.replaceIds) : undefined
  const awaitingFirstView = state.kind === 'loading'
  const generatingWithoutView = running && state.kind !== 'view' && !awaitingFirstView
  useScrollMemory(scroller, memory, state.kind === 'view' && !running ? state.text : null, content)

  const sources = state.kind === 'view' ? state.sources : null
  const actions = useMemo<SourceActions | null>(
    () =>
      sources && {
        store: sources,
        highlightedId,
        replacing,
        edit(ids, text) {
          const target = sources.target(ids, text)
          if (!target) return
          // A scroll the reader just made may not have reported its event yet.
          memory.top = scroller.scrollTop
          void ctx.commands.execute('editor.select', target.start, target.end)
        },
        async addNote(ids, text) {
          const trimmed = text.trim()
          if (trimmed === '') return true
          const target = sources.target(ids)
          return target !== undefined && (await ctx.commands.execute('editor.addNote', target.end, trimmed)) === true
        },
      },
    [sources, highlightedId, replacing, ctx, scroller, memory],
  )

  const openPanel = () => {
    void ctx.commands.execute('rightPane.show', 'genui.panel')
  }

  return (
    <div
      className="genui-view"
      data-testid="genui-view"
      data-state={state.kind}
      aria-busy={awaitingFirstView || generatingWithoutView || undefined}
      ref={content}
    >
      {awaitingFirstView ? <InitialLoading /> : null}
      {generatingWithoutView ? <LoadingSkeleton /> : null}

      {!running && state.kind === 'none' ? (
        <div className="gen-empty genui-empty" data-testid="genui-empty">
          <h2>No view yet.</h2>
          <p>
            {run.kind === 'failed'
              ? 'The last generation failed. What went wrong, and what to do next, is in the GenUI panel.'
              : 'A view lays this document out with the catalog’s components. The document itself stays as it is.'}
          </p>
          <button type="button" className="pill quiet" onClick={openPanel}>
            {run.kind === 'failed' ? 'Open the GenUI panel' : 'Generate from the GenUI panel'}
          </button>
        </div>
      ) : null}

      {!running && state.kind === 'error' ? (
        <div className="gen-empty genui-empty" data-testid="genui-empty">
          <h2>Cannot show this view</h2>
          <p className="genui-file-error">{state.error}</p>
          <button type="button" className="pill quiet" onClick={openPanel}>
            Open the GenUI panel
          </button>
        </div>
      ) : null}

      {state.kind === 'view' && actions ? (
        /* A new file is a new surface: remount rather than diff across surfaces. */
        <ViewSurface key={state.revision} surface={state.surface} sources={actions} reading={memory.reading} />
      ) : null}
    </div>
  )
}

const DEFAULT_MODELS = ['gemini-3.8-flash', 'gemini-4-argon', 'claude-opus-5-5']

/**
 * The right-column GenUI management panel:
 * 1. Scrollable upper region (`.genui-panel-scroll`):
 *    - Layer 1: Status & single primary action for the current state (running, failed,
 *      no view, changed components with jump list, or up to date).
 *    - Layer 2: Generation request (`Focus`), read-only by default when a view exists,
 *      expanding an in-place paper `.composer` on `Edit` (`⌘↩` / `Save` to commit, `Escape` / `Cancel` to revert).
 * 2. Pinned footer (`.genui-panel-footer`, outside scroll area):
 *    - Read-only `Agent: Antigravity (agy)`, `ModelMenu`, `Generated <time>`, and `Undo`.
 */
function GenUIPanel({
  session,
  ctx,
  memory,
}: {
  session: GenUISession
  ctx: PluginContext
  memory: GenUIMemory
}): React.ReactElement {
  const state = useSyncExternalStore(session.store.subscribe, session.store.getSnapshot)
  const run = useSyncExternalStore(session.generation.subscribe, session.generation.getSnapshot)
  const path = useDocumentPath(ctx)
  const changedItems = useChangedComponents(state)

  const hasView = state.kind === 'view'
  const savedRequest = state.kind === 'view' ? (state.view.request ?? '') : ''
  const generatedAt = state.kind === 'view' ? state.view.generatedAt : undefined
  const canUndo = state.kind !== 'loading' && state.canUndo
  const untitled = path === ''
  const running = run.kind === 'running'

  const [request, setRequest] = useState(memory.draft ?? savedRequest)
  const [editingFocus, setEditingFocus] = useState(false)
  const [focusDraft, setFocusDraft] = useState(request)
  const focusTextareaRef = useRef<HTMLTextAreaElement>(null)
  const inlineTextareaRef = useRef<HTMLTextAreaElement>(null)

  // The one settings store's committed value. Placeholders until the first
  // read: the host's defaults are what it answers with anyway.
  const [settings, setSettings] = useState<GenerationSettings>({
    model: 'gemini-3.8-flash',
    agentCommand: 'agy',
    models: DEFAULT_MODELS,
  })
  // Bumped by every applied read or change, so a slow initial read cannot
  // overwrite a newer value that already arrived.
  const settingsRevision = useRef(0)
  const [undoError, setUndoError] = useState('')

  const applySettings = useCallback((next: GenerationSettings) => {
    settingsRevision.current += 1
    setSettings({
      model: next.model,
      agentCommand: next.agentCommand || 'agy',
      models: next.models?.length ? next.models : DEFAULT_MODELS,
    })
  }, [])

  // Sync request when a new view file arrives and the user hasn't drafted a separate request.
  const viewKey = state.kind === 'view' ? state.text : state.kind
  const lastViewKey = useRef(viewKey)
  useEffect(() => {
    if (lastViewKey.current !== viewKey) {
      lastViewKey.current = viewKey
      const next = memory.draft ?? savedRequest
      setRequest(next)
      setFocusDraft(next)
      setEditingFocus(false)
    }
  }, [viewKey, memory.draft, savedRequest])

  // Settings are one store shared by every document and window, so this panel
  // follows it: subscribing first means a change made in another panel while
  // the first read is in flight is not missed, and the revision guard keeps
  // that read from landing on top of it.
  useEffect(() => {
    let live = true
    const followed = session.generation.onSettings((next) => {
      if (live) applySettings(next)
    })
    const at = settingsRevision.current
    session.generation.getSettings().then(
      (loaded) => {
        if (!live || settingsRevision.current !== at) return
        applySettings(loaded)
      },
      () => {},
    )
    return () => {
      live = false
      followed.dispose()
    }
  }, [session.generation, applySettings])

  useEffect(() => {
    if (!editingFocus) return
    const el = focusTextareaRef.current
    if (!el) return
    autoGrowTextarea(el)
  }, [editingFocus, focusDraft])

  useEffect(() => {
    if (hasView) return
    const el = inlineTextareaRef.current
    if (!el) return
    autoGrowTextarea(el)
  }, [hasView, request])

  const models =
    run.kind === 'failed' && run.failure.models?.length
      ? run.failure.models
      : (settings.models ?? DEFAULT_MODELS)

  const changeInlineRequest = (next: string) => {
    memory.draft = next
    setRequest(next)
    setFocusDraft(next)
  }

  const openFocusEditor = () => {
    if (running) return
    setFocusDraft(request)
    setEditingFocus(true)
  }

  const saveFocusEdit = () => {
    memory.draft = focusDraft
    setRequest(focusDraft)
    setEditingFocus(false)
  }

  const cancelFocusEdit = () => {
    setFocusDraft(request)
    setEditingFocus(false)
  }

  const effectiveRequest = editingFocus ? focusDraft : request

  const start = (nextRun: GenerationRun) => {
    if (editingFocus) {
      setRequest(focusDraft)
      setEditingFocus(false)
    }
    memory.draft = undefined
    setUndoError('')
    void session.generation.generate(nextRun)
  }

  const goBack = () => {
    setUndoError('')
    ctx.viewFile.undo().catch((error: unknown) => {
      setUndoError(error instanceof Error ? error.message : String(error))
    })
  }

  // Sends only what the reader changed: the other window's command and the
  // handshake's cached models are not this panel's to write. The committed
  // value arrives through the store's announcement, in write order, so a
  // write's own echo cannot land on a newer one.
  const changeSettings = (patch: SettingsPatch) => {
    session.generation.setSettings(patch).catch((error: unknown) => {
      setUndoError(error instanceof Error ? error.message : String(error))
    })
  }

  const selectModel = (nextModel: string) => {
    changeSettings({ model: nextModel })
  }

  return (
    <div className="genui-panel gp" data-testid="genui-panel">
      <div className="genui-panel-scroll r-scroll">
        {running ? (
          <GenerationProgress run={run} onCancel={() => session.generation.cancel()} />
        ) : run.kind === 'failed' ? (
          <GenerationFailure
            failure={run.failure}
            generation={session.generation}
            settings={settings}
            onSetSettings={changeSettings}
          />
        ) : state.kind === 'loading' ? (
          <div className="gp-block" data-testid="genui-status-loading">
            <div className="gp-status">
              <b>Loading view</b>
              <span>Reading the view file for this document.</span>
            </div>
          </div>
        ) : !hasView ? (
          <div className="gp-block" data-testid="genui-status-empty">
            <div className={`gp-status${state.kind === 'error' ? ' bad' : ''}`}>
              <b>{state.kind === 'error' ? 'Cannot load view' : 'No GenUI view yet'}</b>
              <span>
                {state.kind === 'error'
                  ? state.error
                  : 'Your agent reads a copy of this document and lays it out with the catalog’s components.'}
              </span>
            </div>
            <div className="composer">
              <textarea
                ref={inlineTextareaRef}
                aria-label="Generation request"
                value={request}
                rows={2}
                disabled={running}
                placeholder="What should the view focus on? (optional)"
                onChange={(event) => {
                  changeInlineRequest(event.target.value)
                  autoGrowTextarea(event.currentTarget)
                }}
                onKeyDown={(event) => {
                  if (event.key === 'Escape') {
                    event.preventDefault()
                    event.stopPropagation()
                    event.currentTarget.blur()
                  } else if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
                    event.preventDefault()
                    event.stopPropagation()
                    if (!untitled && !running) {
                      start({ mode: 'full', request })
                    }
                  }
                }}
              />
            </div>
            <div className="gp-acts">
              <button
                type="button"
                className="pill primary full genui-generate"
                disabled={untitled}
                onClick={() => start({ mode: 'full', request })}
              >
                Generate
              </button>
            </div>
            {untitled ? (
              <p className="genui-note">Save the document first: the view is stored next to it.</p>
            ) : null}
          </div>
        ) : changedItems.length > 0 ? (
          <div className="gp-block" data-testid="genui-status-changed">
            <div className="gp-status warn">
              <b>
                {changedItems.length === 1
                  ? '1 component out of date'
                  : `${changedItems.length} components out of date`}
              </b>
              <span>The document changed since this view was generated.</span>
            </div>
            <div className="gp-list changed-list" role="list" aria-label="Out of date components">
              {changedItems.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  className="gp-item changed-item"
                  data-testid={`changed-item-${item.id}`}
                  data-component-id={item.id}
                  onClick={() => session.highlightComponent(item.id)}
                >
                  <span className={`rdot${item.status === 'missing' ? ' bad' : ''}`} aria-hidden="true" />
                  <span className="t">
                    <i
                      className={`ph ${item.status === 'missing' ? 'ph-warning-circle' : 'ph-warning'}`}
                      aria-hidden="true"
                    />
                    {item.title}
                  </span>
                  <span className="s">{item.subtitle}</span>
                </button>
              ))}
            </div>
            <div className="gp-acts">
              <button
                type="button"
                className="pill primary full genui-generate"
                disabled={untitled}
                onClick={() =>
                  start({
                    mode: 'update',
                    request: effectiveRequest,
                    replaceIds: changedItems.map((item) => item.id),
                  })
                }
              >
                {`Regenerate changed (${changedItems.length})`}
              </button>
              <button
                type="button"
                className="pill genui-generate-all"
                disabled={untitled}
                onClick={() => start({ mode: 'full', request: effectiveRequest })}
              >
                Regenerate all
              </button>
            </div>
            {untitled ? (
              <p className="genui-note">Save the document first: the view is stored next to it.</p>
            ) : null}
          </div>
        ) : (
          <div className="gp-block" data-testid="genui-status-fresh">
            <div className="gp-status">
              <b>Up to date</b>
              <span>Matches every source section in the document.</span>
            </div>
            <div className="gp-acts">
              <button
                type="button"
                className="pill full genui-generate-all"
                disabled={untitled}
                onClick={() => start({ mode: 'full', request: effectiveRequest })}
              >
                Regenerate all
              </button>
            </div>
            {untitled ? (
              <p className="genui-note">Save the document first: the view is stored next to it.</p>
            ) : null}
          </div>
        )}

        {undoError ? (
          <p className="genui-file-error" role="alert">
            {undoError}
          </p>
        ) : null}

        {hasView ? (
          <div className="gp-req" data-testid="genui-focus-section">
            <div className="gp-req-h">
              <span>Focus</span>
              {!editingFocus ? (
                <button
                  type="button"
                  data-testid="focus-edit"
                  disabled={running}
                  onClick={openFocusEditor}
                >
                  Edit
                </button>
              ) : null}
            </div>
            {editingFocus ? (
              <div className="composer" data-testid="focus-composer">
                <textarea
                  ref={focusTextareaRef}
                  aria-label="Generation request"
                  value={focusDraft}
                  rows={2}
                  disabled={running}
                  placeholder="What should the view focus on? (optional)"
                  onChange={(event) => setFocusDraft(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Escape') {
                      event.preventDefault()
                      event.stopPropagation()
                      cancelFocusEdit()
                    } else if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
                      event.preventDefault()
                      event.stopPropagation()
                      saveFocusEdit()
                    }
                  }}
                />
                <div className="composer-bar">
                  <span className="hint meta">⌘↩ to save</span>
                  <button type="button" className="pill xs quiet" onClick={cancelFocusEdit}>
                    Cancel
                  </button>
                  <button type="button" className="pill xs primary" onClick={saveFocusEdit}>
                    Save
                  </button>
                </div>
              </div>
            ) : (
              <button
                type="button"
                className={`gp-req-v${request.trim() ? '' : ' empty'}`}
                data-testid="focus-value"
                disabled={running}
                onClick={openFocusEditor}
              >
                {request.trim() ? request : 'No focus set'}
              </button>
            )}
          </div>
        ) : null}
      </div>

      <footer className="genui-panel-footer rp-foot r-foot" data-testid="genui-panel-footer">
        <div className="gp-kv">
          <span>Agent</span>
          <span className="meta">Antigravity (agy)</span>
        </div>
        <div className="gp-kv">
          <span>Model</span>
          <ModelMenu value={settings.model} models={models} disabled={running} onChange={selectModel} />
        </div>
        {generatedAt || canUndo ? (
          <div className="gp-when">
            <span className="meta" data-testid="genui-generated-at">
              {generatedAt ? `Generated ${generatedAt}` : ''}
            </span>
            {canUndo ? (
              <button type="button" className="genui-undo" disabled={running} onClick={goBack}>
                Undo
              </button>
            ) : null}
          </div>
        ) : null}
      </footer>
    </div>
  )
}

/** Why the last run failed, what to do about it, and the agent's own words. */
function GenerationFailure({
  failure,
  generation,
  settings,
  onSetSettings,
}: {
  failure: GenerateFailure
  generation: GenerationController
  settings: GenerationSettings
  onSetSettings: (patch: SettingsPatch) => void
}): React.ReactElement {
  const [locatingAgent, setLocatingAgent] = useState(false)
  const [agentCommandDraft, setAgentCommandDraft] = useState(settings.agentCommand || 'agy')
  const agentInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!locatingAgent) return
    const input = agentInputRef.current
    if (!input) return
    input.focus()
    input.select()
  }, [locatingAgent])

  const openLocateAgent = () => {
    setAgentCommandDraft(settings.agentCommand || 'agy')
    setLocatingAgent(true)
  }

  const saveAgentCommand = () => {
    const trimmed = agentCommandDraft.trim() || 'agy'
    onSetSettings({ agentCommand: trimmed })
    setLocatingAgent(false)
  }

  const cancelLocateAgent = () => {
    setAgentCommandDraft(settings.agentCommand || 'agy')
    setLocatingAgent(false)
  }

  return (
    <div className="gp-block genui-file-error genui-run-error" role="alert">
      <div className="gp-status bad">
        <b>{failure.message}</b>
        {failure.hint ? <span className="genui-run-hint">{failure.hint}</span> : null}
      </div>
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
      {failure.kind === 'agent-not-found' && locatingAgent ? (
        <div className="composer" data-testid="locate-agent-composer">
          <input
            ref={agentInputRef}
            type="text"
            aria-label="Agent command"
            value={agentCommandDraft}
            placeholder="agy or /path/to/agy"
            onChange={(event) => setAgentCommandDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Escape') {
                event.preventDefault()
                event.stopPropagation()
                cancelLocateAgent()
              } else if (event.key === 'Enter') {
                event.preventDefault()
                event.stopPropagation()
                saveAgentCommand()
              }
            }}
          />
          <div className="composer-bar">
            <span className="hint meta">⌘↩ to save</span>
            <button type="button" className="pill xs quiet" onClick={cancelLocateAgent}>
              Cancel
            </button>
            <button type="button" className="pill xs primary" onClick={saveAgentCommand}>
              Save
            </button>
          </div>
        </div>
      ) : null}
      <div className="gp-acts genui-generate-actions">
        <button type="button" className="pill primary full" onClick={() => void generation.retry()}>
          Retry
        </button>
        {failure.kind === 'agent-not-found' && !locatingAgent ? (
          <button type="button" className="pill" onClick={openLocateAgent}>
            Locate agent…
          </button>
        ) : null}
        <button type="button" className="pill quiet" onClick={() => generation.dismissError()}>
          Dismiss
        </button>
      </div>
    </div>
  )
}

/** The steps so far and a Cancel action in the right panel. */
function GenerationProgress({
  run,
  onCancel,
}: {
  run: Extract<GenerationState, { kind: 'running' }>
  onCancel: () => void
}): React.ReactElement {
  const latest = run.progress.at(-1)
  return (
    <div className="gp-block genui-progress" data-testid="genui-progress">
      <div className="gp-status">
        <b className="genui-progress-now" role="status" aria-live="polite">
          {latest ? latest.text : 'Starting the agent'}…
        </b>
      </div>
      {run.progress.length > 0 ? (
        <ol className="genui-progress-steps">
          {run.progress.map((step, index) => (
            <li key={index} data-phase={step.phase}>
              {step.text}
            </li>
          ))}
        </ol>
      ) : null}
      <LoadingSkeleton />
      <div className="gp-acts">
        <button type="button" className="pill full genui-cancel" onClick={onCancel}>
          Cancel
        </button>
      </div>
    </div>
  )
}

/**
 * Shows the document's center GenUI reading view in `el`, which is its scroll
 * area, and follows the view file while mounted.
 */
export function mountGenUIView(el: HTMLElement, ctx: PluginContext, memory: GenUIMemory): Disposable {
  const session = ensureSession(ctx, memory)
  const watch = session.retainWatch()
  const view = session.retainView(el)
  const root = createRoot(el)
  root.render(<GenUIView session={session} ctx={ctx} scroller={el} memory={memory} />)
  // Read while mounted: once the view is removed, its element no longer scrolls.
  const remember = () => {
    memory.top = el.scrollTop
  }
  el.addEventListener('scroll', remember, { passive: true })
  return {
    dispose() {
      el.removeEventListener('scroll', remember)
      view.dispose()
      watch.dispose()
      // Defer: React may be committing the host tree when the view is
      // removed, and a nested root must not unmount during that render.
      queueMicrotask(() => {
        root.unmount()
      })
    },
  }
}

/**
 * Shows the right-column GenUI management panel in `el` and follows the view
 * file while mounted.
 */
export function mountGenUIPanel(el: HTMLElement, ctx: PluginContext, memory: GenUIMemory): Disposable {
  const session = ensureSession(ctx, memory)
  const watch = session.retainWatch()
  const root = createRoot(el)
  root.render(<GenUIPanel session={session} ctx={ctx} memory={memory} />)
  return {
    dispose() {
      watch.dispose()
      queueMicrotask(() => {
        root.unmount()
      })
    },
  }
}
