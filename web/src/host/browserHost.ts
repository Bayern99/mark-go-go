import type {
  CloseResult,
  Disposable,
  DocState,
  DocumentInfo,
  GenerateProgress,
  GenerateRequest,
  GenerationSettings,
  HostAdapter,
  HostDocSnapshot,
  NavSearchResult,
  NavSnapshot,
  ViewSnapshot,
  WorkspaceAdapter,
  WorkspaceKind,
  WorkspaceSnapshot,
} from './types'
import { cancelled, failureOf, GenerationError } from './generationError'
import { BrowserNavIndex, liveTexts, type BrowserNavSeed } from '../workspace/nav'

export const DEMO_MARKDOWN = `# Quarterly Report

Welcome to **Mark Go Go**, a local-first Markdown editor. Everything you type is
pushed straight back into the document that the desktop host owns.

## Overview

The editor keeps the original Markdown bytes for every block you have not
touched. Only the block you are editing is re-serialized.

> [!NOTE]
> Alert-style blockquotes are locked so their GitHub syntax survives a round trip.

## Metrics

| Metric           | Value |
| ---------------- | ----- |
| Documents edited | 12    |
| Round trips      | 100%  |
| Blocks locked    | 3     |

## Next steps

1. Open a real file through the MyGo host.
2. Toggle the generated UI panel off and on again.
3. Scroll to a heading from the panel.

\`\`\`go
func main() {
    fmt.Println("local first")
}
\`\`\`
`

const INITIAL_STATE: DocState = {
  name: 'welcome.md',
  path: '/demo/welcome.md',
  edited: false,
  conflict: false,
  deleted: false,
  readOnly: false,
  generating: false,
}

export interface BrowserHost extends HostAdapter {
  /** Simulate an external change (used by tests and the demo reload button). */
  reload(text: string): void
  getState(): DocumentInfo
  /** Overlay file-status flags so workspace tests can drive tab badges. */
  patchState(partial: Partial<DocState>): void
  /** Simulate another tool writing (text) or removing (null) the view file; the kept view stays. */
  writeView(text: string | null): void
  /** The kept view the last generation replaced, or null. */
  keptView(): string | null
  readonly generations: GenerateRequest[]
  /** Settings (model, agent command) captured at each generate start. */
  readonly capturedSettings: GenerationSettings[]
  /** Emit the first view snapshot when `holdFirstView` deferred it. */
  releaseFirstView(): void
  /** Simulate an external path change while a generation is running. */
  setPath(path: string): void
  /** Abort an in-flight generation as a real close of this document would. */
  abandonGeneration(): void
}

/**
 * A scripted stand-in for the agent. It gets the request and a `progress`
 * function; what it returns becomes the view file, for an update as well
 * (the desktop host merges the agent's message itself). Throwing a
 * `GenerationError` plays that failure; any other error is an `agent-error`.
 * `signal` aborts when the page cancels.
 */
export type FakeGenerator = (
  req: GenerateRequest,
  progress: (step: GenerateProgress) => void,
  signal: AbortSignal,
) => Promise<string>

export interface BrowserHostOptions {
  /** Initial view file text; none when omitted. */
  view?: string
  /** Initial kept view, the one to go back to; none when omitted. */
  kept?: string
  /** Plays the agent; without one, Generate fails with a message. */
  generator?: FakeGenerator
  /** Stored settings; the defaults of the desktop host when omitted. */
  settings?: GenerationSettings
  /** Shared settings object, as the desktop host's one settings file. */
  settingsRef?: GenerationSettings
  /** Open the document as never saved: it has no path, so no view file. */
  untitled?: boolean
  /** Do not emit the first watch snapshot until `releaseFirstView`. */
  holdFirstView?: boolean
}

export const DEFAULT_MODELS = ['gemini-3.8-flash', 'gemini-4-argon', 'claude-opus-5-5'] as const

export const DEFAULT_SETTINGS: GenerationSettings = {
  model: 'gemini-3.8-flash',
  agentCommand: 'agy',
  models: [...DEFAULT_MODELS],
}

/**
 * In-memory HostAdapter used when the page runs outside MyGo and in tests.
 * It mirrors the semantics the Go side promises: full-text updates, dirty
 * state, and reload events that replace the whole document.
 */
export function createBrowserHost(
  initial: string = DEMO_MARKDOWN,
  initialName = INITIAL_STATE.name,
  options: BrowserHostOptions = {},
): BrowserHost {
  let text = initial
  let saved = initial
  let version = 1
  const id = `browser:${initialName}:${Math.random().toString(36).slice(2, 10)}`
  let state: DocumentInfo = {
    ...INITIAL_STATE,
    id,
    name: initialName,
    path: options.untitled ? '' : `/demo/${initialName}`,
  }
  let view: string | null = options.view ?? null
  let kept: string | null = options.kept ?? null
  const reloadListeners = new Set<(snap: HostDocSnapshot) => void>()
  const stateListeners = new Set<(state: DocumentInfo) => void>()
  const viewListeners = new Set<(snap: ViewSnapshot) => void>()
  let settings: GenerationSettings = options.settingsRef ?? {
    model: options.settings?.model ?? DEFAULT_SETTINGS.model,
    agentCommand: options.settings?.agentCommand || DEFAULT_SETTINGS.agentCommand,
    models: options.settings?.models?.length ? [...options.settings.models] : [...DEFAULT_MODELS],
  }
  let generating = false
  let capturedPath = ''
  let abandoned = false
  let abortInFlight: (() => void) | undefined
  const generations: GenerateRequest[] = []
  const capturedSettingsLog: GenerationSettings[] = []

  const viewSnapshot = (): ViewSnapshot =>
    view === null
      ? { exists: false, text: '', error: '', canUndo: kept !== null }
      : { exists: true, text: view, error: '', canUndo: kept !== null }

  const snapshot = (): HostDocSnapshot => ({ id, text, version, state: { ...state } })

  const emitState = () => {
    const copy = { ...state }
    for (const fn of stateListeners) fn({ ...copy })
  }

  const writeView = (next: string | null) => {
    view = next
    const snap = viewSnapshot()
    for (const fn of viewListeners) fn(snap)
  }

  const hostSelf: BrowserHost = {
    id,
    generations,
    capturedSettings: capturedSettingsLog,
    async load() {
      return snapshot()
    },
    async update(next: string) {
      text = next
      const edited = next !== saved
      if (state.edited !== edited) {
        state = { ...state, edited }
        emitState()
      }
    },
    async flush(next: string) {
      text = next
      const edited = next !== saved
      if (state.edited !== edited) {
        state = { ...state, edited }
        emitState()
      }
    },
    async save() {
      saved = text
      if (state.edited) {
        state = { ...state, edited: false }
        emitState()
      }
    },
    async saveAs() {
      if (generating) throw new Error('A view is being generated; wait for it to finish, or cancel it.')
      await this.save()
    },
    async rename() {
      if (generating) throw new Error('A view is being generated; wait for it to finish, or cancel it.')
    },
    async moveTo() {
      if (generating) throw new Error('A view is being generated; wait for it to finish, or cancel it.')
    },
    async revert() {},
    async setViewMode(mode: string) {
      state = { ...state, viewMode: mode }
      emitState()
    },
    bindFlush(getText) {
      flushReaders.set(hostSelf, getText)
      return disposable(() => {
        if (flushReaders.get(hostSelf) === getText) flushReaders.delete(hostSelf)
      })
    },
    onReload(fn) {
      reloadListeners.add(fn)
      return disposable(() => reloadListeners.delete(fn))
    },
    onState(fn) {
      stateListeners.add(fn)
      return disposable(() => stateListeners.delete(fn))
    },
    patchState(partial) {
      state = { ...state, ...partial }
      emitState()
    },
    reload(next: string) {
      text = next
      saved = next
      version += 1
      state = { ...state, edited: false }
      const snap = snapshot()
      for (const fn of reloadListeners) fn(snap)
      emitState()
    },
    getState() {
      return { ...state }
    },
    watchView(fn) {
      viewListeners.add(fn)
      // Like the Go host, the first snapshot is the file as it is now.
      if (!options.holdFirstView) {
        queueMicrotask(() => {
          if (viewListeners.has(fn)) fn(viewSnapshot())
        })
      }
      return disposable(() => viewListeners.delete(fn))
    },
    releaseFirstView() {
      const snap = viewSnapshot()
      for (const fn of viewListeners) fn(snap)
    },
    writeView,
    keptView: () => kept,
    setPath(path: string) {
      state = { ...state, path }
      emitState()
    },
    abandonGeneration() {
      abandoned = true
      abortInFlight?.()
    },
    async undoView() {
      if (generating) throw new Error('A view is being generated; wait for it to finish, or cancel it.')
      if (kept === null) throw new Error('There is no earlier view to go back to.')
      const back = kept
      kept = null
      writeView(back)
    },
    async generate(req, onProgress, signal) {
      if (state.path === '') {
        throw new GenerationError({ kind: 'unsaved', message: 'Save the document first: the view is stored next to it.' })
      }
      if (generating) {
        throw new GenerationError({
          kind: 'busy',
          message: 'A view is already being generated.',
          hint: 'Wait for it to finish, or cancel it.',
        })
      }
      const captured: GenerationSettings = {
        model: settings.model,
        agentCommand: settings.agentCommand,
        models: settings.models ? [...settings.models] : [...DEFAULT_MODELS],
      }
      capturedPath = state.path
      abandoned = false
      generations.push(req)
      capturedSettingsLog.push(captured)
      if (!options.generator) {
        throw new GenerationError({
          kind: 'agent-not-found',
          message: 'No agent is available in the browser preview.',
          hint: 'Open the document in the Mark Go Go app to generate a view.',
        })
      }
      generating = true
      state = { ...state, generating: true }
      emitState()
      try {
        onProgress({ phase: 'starting', text: 'Starting the agent' })
        let stop!: (error: Error) => void
        const stopped = new Promise<never>((_, reject) => {
          stop = reject
        })
        abortInFlight = () => stop(cancelled())
        if (signal.aborted || abandoned) abortInFlight()
        else signal.addEventListener('abort', abortInFlight, { once: true })
        const body = await Promise.race([options.generator(req, onProgress, signal), stopped])
        onProgress({ phase: 'checking', text: 'Checking the view' })
        if (abandoned) throw cancelled()
        if (state.path === '' || state.path !== capturedPath) {
          throw new GenerationError({
            kind: 'agent-error',
            message: "The document's path changed while the agent worked, so the view was not written.",
            hint: 'Generate again.',
          })
        }
        if (view !== null) kept = view
        writeView(body)
      } catch (error) {
        if (signal.aborted || abandoned) throw cancelled()
        throw error instanceof GenerationError ? error : new GenerationError(failureOf(error))
      } finally {
        abortInFlight = undefined
        generating = false
        if (state.generating) {
          state = { ...state, generating: false }
          emitState()
        }
      }
    },
    async getSettings() {
      return {
        ...settings,
        models: settings.models ? [...settings.models] : [...DEFAULT_MODELS],
      }
    },
    async setSettings(next) {
      settings.model = next.model
      settings.agentCommand = next.agentCommand || DEFAULT_SETTINGS.agentCommand
      if (next.models?.length) settings.models = [...next.models]
    },
  }
  return hostSelf
}

function disposable(fn: () => void): Disposable {
  let done = false
  return {
    dispose() {
      if (done) return
      done = true
      fn()
    },
  }
}

const flushReaders = new WeakMap<HostAdapter, () => string>()


/** One-document workspace around a HostAdapter, for existing pane tests. */
export function wrapHostAsWorkspace(host: HostAdapter, name?: string): WorkspaceAdapter {
  const info = (): DocumentInfo => {
    const state = isBrowserHost(host)
      ? host.getState()
      : {
          name: name ?? 'document',
          path: '',
          edited: false,
          conflict: false,
          deleted: false,
          readOnly: false,
          generating: false,
        }
    return {
      id: host.id,
      name: name ?? state.name,
      path: state.path,
      edited: state.edited,
      conflict: state.conflict,
      deleted: state.deleted,
      readOnly: state.readOnly,
      generating: state.generating ?? false,
      viewMode: state.viewMode,
      restoreError: state.restoreError,
    }
  }
  const snap = (): WorkspaceSnapshot => ({
    id: 'ws-test',
    root: '',
    name: info().name,
    kind: info().path ? 'folder' : 'untitled',
    documents: [info()],
    activeId: host.id,
  })
  const listeners = new Set<(s: WorkspaceSnapshot) => void>()
  host.onState(() => {
    const current = snap()
    for (const fn of listeners) fn(current)
  })
  const idleNav = (): NavSnapshot => ({
    root: '',
    status: 'idle',
    indexed: 0,
    total: 0,
    entries: [],
    collapsedCount: 0,
  })
  const idleSearch = (): NavSearchResult => ({
    query: '',
    status: 'idle',
    matched: 0,
    indexed: 0,
    total: 0,
    incomplete: false,
    hits: [],
  })
  return {
    snapshot: async () => snap(),
    subscribe(fn) {
      listeners.add(fn)
      return disposable(() => listeners.delete(fn))
    },
    async open() {
      return info()
    },
    async openWorkspace() {},
    async newDocument() {
      return info()
    },
    async duplicate() {
      return info()
    },
    async activate() {},
    async closeDocument() {
      return { closed: false, canceled: false } satisfies CloseResult
    },
    async closeWorkspace() {
      return { closed: false, canceled: false }
    },
    document(id: string) {
      if (id !== host.id) throw new Error(`unknown document ${id}`)
      return host
    },
    async navSnapshot() {
      return idleNav()
    },
    subscribeNav() {
      return disposable(() => {})
    },
    async setFilter() {
      return idleSearch()
    },
    subscribeSearch() {
      return disposable(() => {})
    },
    async toggleFolder() {},
    async revealPath() {},
    async pickFolder() {
      return ''
    },
    async recentWorkspaces() {
      return []
    },
  }
}

export interface BrowserWorkspaceSeed {
  host: HostAdapter
  name?: string
  path?: string
}

export interface BrowserWorkspaceOptions {
  id?: string
  root?: string
  name?: string
  kind?: WorkspaceKind
  /** Close-ask: canceled keeps every tab and the adapter cache. */
  confirmClose?: (id: string) => CloseResult | Promise<CloseResult>
  confirmCloseWorkspace?: () => CloseResult | Promise<CloseResult>
  nav?: BrowserNavSeed
  restoreError?: string
}

function isBrowserHost(host: HostAdapter): host is BrowserHost {
  return typeof (host as BrowserHost).getState === 'function'
}

function asSeed(doc: HostAdapter | BrowserWorkspaceSeed): BrowserWorkspaceSeed {
  if (typeof (doc as BrowserWorkspaceSeed).host?.id === 'string') {
    return doc as BrowserWorkspaceSeed
  }
  return { host: doc as HostAdapter }
}

function infoFrom(seed: BrowserWorkspaceSeed): DocumentInfo {
  const state = isBrowserHost(seed.host)
    ? seed.host.getState()
    : { name: 'untitled.md', path: '', edited: false, conflict: false, deleted: false, readOnly: false, generating: false }
  return {
    id: seed.host.id,
    name: seed.name ?? state.name,
    path: seed.path ?? state.path,
    edited: state.edited,
    conflict: state.conflict,
    deleted: state.deleted,
    readOnly: state.readOnly,
    generating: state.generating ?? false,
    viewMode: state.viewMode,
    restoreError: state.restoreError,
  }
}

/**
 * In-memory multi-document WorkspaceAdapter for workspace tests and the
 * browser preview. There is no hidden second window: every document is a
 * tab in this one snapshot. `document(id)` returns the same HostAdapter
 * for as long as this window hosts that id.
 */
export function createBrowserWorkspace(
  docs: Array<HostAdapter | BrowserWorkspaceSeed> = [createBrowserHost()],
  options: BrowserWorkspaceOptions = {},
): WorkspaceAdapter {
  const adapters = new Map<string, HostAdapter>()
  const infos = new Map<string, DocumentInfo>()
  const order: string[] = []
  const stateSubs = new Map<string, Disposable>()
  const listeners = new Set<(snap: WorkspaceSnapshot) => void>()
  let activeId = ''

  const workspaceId = options.id ?? 'browser-workspace'
  const root = options.root ?? '/demo'
  const workspaceName = options.name ?? 'demo'
  const kind: WorkspaceKind = options.kind ?? (root === '' ? 'untitled' : 'folder')

  const snap = (): WorkspaceSnapshot => ({
    id: workspaceId,
    root,
    name: workspaceName,
    kind,
    documents: order.map((id) => {
      const info = infos.get(id)
      if (!info) throw new Error(`missing document ${id}`)
      return { ...info }
    }),
    activeId,
    restoreError: options.restoreError,
  })

  const nav = new BrowserNavIndex(root, options.nav)

  const emit = () => {
    const current = snap()
    for (const fn of listeners) fn(current)
    void refreshNav()
  }

  const refreshNav = async () => {
    const docs = snap().documents
    const texts = await liveTexts(docs, (id) => {
      const host = adapters.get(id)
      if (!host) throw new Error(`unknown document ${id}`)
      return host
    })
    nav.emit(docs, texts)
  }

  const drop = (id: string) => {
    const host = adapters.get(id)
    if (host && isBrowserHost(host)) host.abandonGeneration()
    stateSubs.get(id)?.dispose()
    stateSubs.delete(id)
    adapters.delete(id)
    infos.delete(id)
    const index = order.indexOf(id)
    if (index >= 0) order.splice(index, 1)
    if (activeId === id) activeId = order[0] ?? ''
  }

  const add = (seed: BrowserWorkspaceSeed, activate = true): DocumentInfo => {
    const host = seed.host
    const existing = adapters.get(host.id)
    if (existing) {
      if (activate) activeId = host.id
      return { ...(infos.get(host.id) as DocumentInfo) }
    }
    const byPath = seed.path ?? (isBrowserHost(host) ? host.getState().path : '')
    if (byPath !== '') {
      for (const [id, info] of infos) {
        if (info.path === byPath) {
          if (activate) activeId = id
          return { ...info }
        }
      }
    }
    adapters.set(host.id, host)
    const info = infoFrom(seed)
    const overlayName = seed.name
    const overlayPath = seed.path
    infos.set(host.id, info)
    order.push(host.id)
    stateSubs.set(
      host.id,
      host.onState((state) => {
        const current = infos.get(host.id)
        if (!current) return
        infos.set(host.id, {
          ...current,
          name: overlayName ?? state.name,
          path: overlayPath ?? state.path,
          edited: state.edited,
          conflict: state.conflict,
          deleted: state.deleted,
          readOnly: state.readOnly,
          generating: state.generating ?? false,
          viewMode: state.viewMode,
          restoreError: state.restoreError,
        })
        emit()
      }),
    )
    if (activate || activeId === '') activeId = host.id
    return { ...info }
  }

  for (const doc of docs) add(asSeed(doc), false)
  if (!activeId && order[0]) activeId = order[0]

  return {
    snapshot: async () => snap(),
    subscribe(fn) {
      listeners.add(fn)
      return disposable(() => listeners.delete(fn))
    },
    async open(path: string) {
      for (const [id, info] of infos) {
        if (info.path === path) {
          activeId = id
          emit()
          return { ...info }
        }
      }
      const name = path.replace(/\\/g, '/').split('/').pop() || 'untitled.md'
      const created = add({ host: createBrowserHost('', name), name, path })
      emit()
      return created
    },
    async openWorkspace() {},
    async newDocument() {
      const created = add({ host: createBrowserHost('', 'Untitled.md', { untitled: true }), name: 'Untitled.md', path: '' })
      emit()
      return created
    },
    async duplicate(id: string) {
      const host = adapters.get(id)
      const info = infos.get(id)
      if (!host || !info) throw new Error(`unknown document ${id}`)
      const snapshot = await host.load()
      const created = add({
        host: createBrowserHost(snapshot.text, info.name, { untitled: true }),
        name: info.name,
        path: '',
      })
      emit()
      return created
    },
    async activate(id: string) {
      if (!adapters.has(id)) throw new Error(`unknown document ${id}`)
      activeId = id
      emit()
    },
    async closeDocument(id: string, text?: string) {
      const host = adapters.get(id)
      if (!host) throw new Error(`unknown document ${id}`)
      if (text !== undefined) await host.flush(text)
      else {
        const reader = flushReaders.get(host)
        if (reader) await host.flush(reader())
      }
      const result = options.confirmClose
        ? await options.confirmClose(id)
        : { closed: true, canceled: false }
      if (result.canceled || !result.closed) return result
      drop(id)
      emit()
      return result
    },
    async closeWorkspace() {
      for (const id of [...order]) {
        const host = adapters.get(id)
        if (!host) continue
        const reader = flushReaders.get(host)
        if (reader) await host.flush(reader())
      }
      const result = options.confirmCloseWorkspace
        ? await options.confirmCloseWorkspace()
        : { closed: true, canceled: false }
      if (result.canceled || !result.closed) return result
      for (const id of [...order]) drop(id)
      emit()
      return result
    },
    document(id: string) {
      const host = adapters.get(id)
      if (!host) throw new Error(`unknown document ${id}`)
      return host
    },
    async navSnapshot() {
      const docs = snap().documents
      const texts = await liveTexts(docs, (id) => {
        const host = adapters.get(id)
        if (!host) throw new Error(`unknown document ${id}`)
        return host
      })
      return nav.snapshot(docs, texts)
    },
    subscribeNav(fn) {
      return disposable(nav.subscribeNav(fn))
    },
    async setFilter(query: string) {
      nav.setQuery(query)
      const docs = snap().documents
      const texts = await liveTexts(docs, (id) => {
        const host = adapters.get(id)
        if (!host) throw new Error(`unknown document ${id}`)
        return host
      })
      const result = nav.search(docs, texts)
      nav.emit(docs, texts)
      return result
    },
    subscribeSearch(fn) {
      return disposable(nav.subscribeSearch(fn))
    },
    async toggleFolder(rel: string) {
      nav.toggleFolder(rel)
      await refreshNav()
    },
    async revealPath(path: string) {
      nav.reveal(path)
      await refreshNav()
    },
    async pickFolder() {
      if (nav.pickFolderImpl) {
        const chosen = await nav.pickFolderImpl()
        if (chosen) nav.remember(chosen)
        return chosen
      }
      return root
    },
    async recentWorkspaces() {
      return [...nav.recents]
    },
  }
}
