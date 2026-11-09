import { useSyncExternalStore } from 'react'
import type { PluginContext } from '../../core/types'
import type { MarkdownHeading } from '../../markdown/headings'
import { extractHeadings } from '../../markdown/headings'

/** Heading text is re-read this long after the last document change. */
export const OUTLINE_DEBOUNCE_MS = 300

export interface OutlineState {
  headings: MarkdownHeading[]
  cursorOffset: number
}

/**
 * Outline data for one plugin activation. Headings follow debounced document
 * changes; the cursor offset comes straight from the editor so highlighting
 * stays responsive while typing.
 */
export class OutlineStore {
  private state: OutlineState
  private timer: number | undefined
  private readonly listeners = new Set<() => void>()

  constructor(initialText: string) {
    this.state = { headings: extractHeadings(initialText), cursorOffset: 0 }
  }

  readonly subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  readonly getSnapshot = (): OutlineState => this.state

  setCursorOffset(offset: number): void {
    if (offset === this.state.cursorOffset) return
    this.state = { ...this.state, cursorOffset: offset }
    this.emit()
  }

  schedule(text: string): void {
    if (this.timer !== undefined) window.clearTimeout(this.timer)
    this.timer = window.setTimeout(() => {
      this.timer = undefined
      const headings = extractHeadings(text)
      if (sameHeadings(headings, this.state.headings)) return
      this.state = { ...this.state, headings }
      this.emit()
    }, OUTLINE_DEBOUNCE_MS)
  }

  cancelPending(): void {
    if (this.timer === undefined) return
    window.clearTimeout(this.timer)
    this.timer = undefined
  }

  private emit(): void {
    for (const listener of this.listeners) listener()
  }
}

/** Index of the section containing `cursorOffset`, or -1 before the first heading. */
function activeHeadingIndex(headings: MarkdownHeading[], cursorOffset: number): number {
  let active = -1
  headings.forEach((heading, index) => {
    if (heading.offset <= cursorOffset) active = index
  })
  return active
}

function sameHeadings(a: MarkdownHeading[], b: MarkdownHeading[]): boolean {
  if (a.length !== b.length) return false
  return a.every((heading, index) => {
    const other = b[index] as MarkdownHeading
    return (
      heading.offset === other.offset &&
      heading.level === other.level &&
      heading.text === other.text
    )
  })
}

export function OutlinePanel({
  ctx,
  store,
}: {
  ctx: PluginContext
  store: OutlineStore
}): React.ReactElement {
  const { headings, cursorOffset } = useSyncExternalStore(store.subscribe, store.getSnapshot)
  const active = activeHeadingIndex(headings, cursorOffset)

  return (
    <nav className="outline-panel" aria-label="Document outline">
      {headings.length === 0 ? (
        <p className="outline-empty" data-testid="outline-empty">
          No headings in this document.
        </p>
      ) : (
        <>
          <p className="outline-count" data-testid="outline-count">
            {headings.length === 1 ? '1 heading' : `${headings.length} headings`}
          </p>
          <ul className="outline-list" data-testid="outline-list">
          {headings.map((heading, index) => (
            <li key={heading.offset} className="outline-item">
              <button
                type="button"
                className="outline-link ol-row"
                style={
                  {
                    '--l': heading.level - 1,
                    paddingLeft: `${8 + (heading.level - 1) * 16}px`,
                  } as React.CSSProperties
                }
                aria-current={index === active ? 'true' : undefined}
                title={heading.text}
                onClick={() => {
                  void ctx.commands.execute('editor.reveal', heading.offset)
                }}
              >
                {heading.text}
              </button>
            </li>
          ))}
        </ul>
        </>
      )}
    </nav>
  )
}
