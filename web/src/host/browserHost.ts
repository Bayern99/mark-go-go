import type {
  Disposable,
  DocState,
  GenerateProgress,
  GenerateRequest,
  GenerationSettings,
  HostAdapter,
  HostDocSnapshot,
  ViewSnapshot,
} from './types'
import { cancelled, failureOf, GenerationError } from './generationError'

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
}

export interface BrowserHost extends HostAdapter {
  /** Simulate an external change (used by tests and the demo reload button). */
  reload(text: string): void
  getState(): DocState
  /** Simulate another tool writing (text) or removing (null) the view file. */
  writeView(text: string | null): void
  /** Requests generate() received, in order. */
  readonly generations: GenerateRequest[]
}

/**
 * A scripted stand-in for the agent. It gets the request and a `progress`
 * function; what it returns becomes the view file. Throwing a
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
  /** Plays the agent; without one, Generate fails with a message. */
  generator?: FakeGenerator
  /** Stored settings; the defaults of the desktop host when omitted. */
  settings?: GenerationSettings
  /** Open the document as never saved: it has no path, so no view file. */
  untitled?: boolean
}

export const DEFAULT_SETTINGS: GenerationSettings = { model: 'gemini-3.8-flash', agentCommand: 'agy' }

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
  let version = 1
  let state: DocState = {
    ...INITIAL_STATE,
    name: initialName,
    path: options.untitled ? '' : `/demo/${initialName}`,
  }
  let view: string | null = options.view ?? null
  const reloadListeners = new Set<(snap: HostDocSnapshot) => void>()
  const stateListeners = new Set<(state: DocState) => void>()
  const viewListeners = new Set<(snap: ViewSnapshot) => void>()
  let settings: GenerationSettings = { ...(options.settings ?? DEFAULT_SETTINGS) }
  let generating = false
  const generations: GenerateRequest[] = []

  const viewSnapshot = (): ViewSnapshot =>
    view === null ? { exists: false, text: '', error: '' } : { exists: true, text: view, error: '' }

  const snapshot = (): HostDocSnapshot => ({ text, version, state: { ...state } })

  const emitState = () => {
    const copy = { ...state }
    for (const fn of stateListeners) fn({ ...copy })
  }

  const writeView = (next: string | null) => {
    view = next
    const snap = viewSnapshot()
    for (const fn of viewListeners) fn(snap)
  }

  return {
    generations,
    async load() {
      return snapshot()
    },
    async update(next: string) {
      text = next
      if (!state.edited) {
        state = { ...state, edited: true }
        emitState()
      }
    },
    onReload(fn) {
      reloadListeners.add(fn)
      return disposable(() => reloadListeners.delete(fn))
    },
    onState(fn) {
      stateListeners.add(fn)
      return disposable(() => stateListeners.delete(fn))
    },
    reload(next: string) {
      text = next
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
      // Like the Go host, the first snapshot comes once the watch runs.
      queueMicrotask(() => {
        if (viewListeners.has(fn)) fn(viewSnapshot())
      })
      return disposable(() => viewListeners.delete(fn))
    },
    writeView,
    // Same rules as the desktop host: one run at a time, a saved document,
    // and the view file written only when the run succeeds.
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
      generations.push(req)
      if (!options.generator) {
        throw new GenerationError({
          kind: 'agent-not-found',
          message: 'No agent is available in the browser preview.',
          hint: 'Open the document in the Mark Go Go app to generate a view.',
        })
      }
      generating = true
      try {
        onProgress({ phase: 'starting', text: 'Starting the agent' })
        const stopped = new Promise<never>((_, reject) => {
          const stop = () => reject(cancelled())
          if (signal.aborted) stop()
          else signal.addEventListener('abort', stop, { once: true })
        })
        const body = await Promise.race([options.generator(req, onProgress, signal), stopped])
        onProgress({ phase: 'checking', text: 'Checking the view' })
        writeView(body)
      } catch (error) {
        if (signal.aborted) throw cancelled()
        throw error instanceof GenerationError ? error : new GenerationError(failureOf(error))
      } finally {
        generating = false
      }
    },
    async getSettings() {
      return { ...settings }
    },
    async setSettings(next) {
      settings = { model: next.model, agentCommand: next.agentCommand || DEFAULT_SETTINGS.agentCommand }
    },
  }
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
