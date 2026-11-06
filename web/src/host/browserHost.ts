import type { Disposable, DocState, HostAdapter, HostDocSnapshot } from './types'

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
}

/**
 * In-memory HostAdapter used when the page runs outside MyGo and in tests.
 * It mirrors the semantics the Go side promises: full-text updates, dirty
 * state, and reload events that replace the whole document.
 */
export function createBrowserHost(
  initial: string = DEMO_MARKDOWN,
  initialName = INITIAL_STATE.name,
): BrowserHost {
  let text = initial
  let version = 1
  let state: DocState = { ...INITIAL_STATE, name: initialName }
  const reloadListeners = new Set<(snap: HostDocSnapshot) => void>()
  const stateListeners = new Set<(state: DocState) => void>()

  const snapshot = (): HostDocSnapshot => ({ text, version, state: { ...state } })

  const emitState = () => {
    const copy = { ...state }
    for (const fn of stateListeners) fn({ ...copy })
  }

  return {
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
