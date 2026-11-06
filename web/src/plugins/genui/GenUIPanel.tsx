import { useMemo, useSyncExternalStore } from 'react'
import type { PluginContext } from '../../core/types'
import { renderBlock } from './catalog'
import { extractHeadings } from './headings'
import type { GenUIDocument, SourceRef } from './schema'

export class SnapshotStore {
  private value: { text: string; version: number }
  private readonly listeners = new Set<() => void>()

  constructor(initial: { text: string; version: number }) {
    this.value = initial
  }

  readonly subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  readonly getSnapshot = (): { text: string; version: number } => this.value

  set(next: { text: string; version: number }): void {
    if (next.text === this.value.text && next.version === this.value.version) return
    this.value = next
    for (const listener of this.listeners) listener()
  }
}

export function GenUIPanel({
  ctx,
  store,
  document,
}: {
  ctx: PluginContext
  store: SnapshotStore
  document: GenUIDocument
}): React.ReactElement {
  const snapshot = useSyncExternalStore(store.subscribe, store.getSnapshot)
  const headings = useMemo(() => extractHeadings(snapshot.text), [snapshot.text])

  const reveal = (source: SourceRef) => {
    void ctx.commands.execute('editor.revealHeading', source.heading)
  }

  return (
    <div className="genui-panel">
      <header className="genui-header">
        <h2>{document.title}</h2>
        <p>Generated from {document.blocks.length} components</p>
      </header>

      <section className="genui-headings">
        <h3>Headings in this document</h3>
        {headings.length === 0 ? (
          <p className="genui-empty">No headings yet.</p>
        ) : (
          <ul>
            {headings.map((heading, index) => (
              <li key={index} style={{ paddingLeft: `${(heading.level - 1) * 12}px` }}>
                <button
                  type="button"
                  className="genui-heading-link"
                  onClick={() => {
                    void ctx.commands.execute('editor.revealHeading', heading.text)
                  }}
                >
                  {heading.text}
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>

      {document.blocks.map((block, index) => (
        <div key={index}>{renderBlock(block, reveal)}</div>
      ))}
    </div>
  )
}
