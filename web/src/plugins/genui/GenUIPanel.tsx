import { useSyncExternalStore } from 'react'
import type { PluginContext } from '../../core/types'
import type { ViewSnapshot } from '../../host/types'
import { extractSections, findSection, type MarkdownSection } from '../../markdown/sections'
import { renderBlock } from './catalog'
import { parseViewFile, type ParsedView, type ViewSection } from './schema'

/** Section freshness is re-checked this long after the last document change, like the outline. */
export const GENUI_DEBOUNCE_MS = 300

/**
 * - `fresh`: the section's text is what the view was generated from.
 * - `stale`: the section's text changed since.
 * - `missing`: the document has no such heading (or occurrence) any more.
 * - `unchecked`: freshness is unknown: the view gives no `sourceHash`, or the
 *   section names no valid heading (its problems say why).
 */
export type SectionStatus = 'fresh' | 'stale' | 'missing' | 'unchecked'

export type GenUIState =
  | { kind: 'loading' }
  | { kind: 'none' }
  | { kind: 'error'; error: string }
  | {
      kind: 'view'
      generatedAt?: string
      model?: string
      sections: Array<{ section: ViewSection; status: SectionStatus }>
    }

/**
 * GenUI data for one plugin activation: the view file as the host last
 * reported it, checked against the document's sections. A new view file is
 * shown at once, against the latest text; document edits are debounced.
 * Sections are only parsed while there is a view to check them against.
 */
export class GenUIStore {
  private state: GenUIState = { kind: 'loading' }
  private parsed: ParsedView | null | undefined
  private text: string
  /** Sections of `text`, computed when a view needs them. */
  private sections: MarkdownSection[] | null = null
  private timer: number | undefined
  private readonly listeners = new Set<() => void>()

  constructor(initialText: string) {
    this.text = initialText
  }

  readonly subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  readonly getSnapshot = (): GenUIState => this.state

  setView(snap: ViewSnapshot): void {
    if (snap.error !== '') this.parsed = { ok: false, error: snap.error }
    else this.parsed = snap.exists ? parseViewFile(snap.text) : null
    // A pending edit is part of what the new view is checked against.
    this.cancelPending()
    this.publish()
  }

  schedule(text: string): void {
    this.text = text
    this.sections = null
    if (this.timer !== undefined) window.clearTimeout(this.timer)
    this.timer = window.setTimeout(() => {
      this.timer = undefined
      this.publish()
    }, GENUI_DEBOUNCE_MS)
  }

  cancelPending(): void {
    if (this.timer === undefined) return
    window.clearTimeout(this.timer)
    this.timer = undefined
  }

  private publish(): void {
    const next = this.nextState()
    if (next === undefined || sameState(this.state, next)) return
    this.state = next
    for (const listener of this.listeners) listener()
  }

  private nextState(): GenUIState | undefined {
    const parsed = this.parsed
    if (parsed === undefined) return undefined
    if (parsed === null) return { kind: 'none' }
    if (!parsed.ok) return { kind: 'error', error: parsed.error }
    this.sections ??= extractSections(this.text)
    const sections = this.sections
    return {
      kind: 'view',
      generatedAt: parsed.view.generatedAt,
      model: parsed.view.model,
      sections: parsed.view.sections.map((section) => ({
        section,
        status: sectionStatus(sections, section),
      })),
    }
  }
}

/** Whether a re-check changed nothing the panel shows. */
function sameState(a: GenUIState, b: GenUIState): boolean {
  if (a.kind === 'view' && b.kind === 'view') {
    return (
      a.sections.length === b.sections.length &&
      a.sections.every(
        (entry, index) =>
          entry.section === b.sections[index]?.section && entry.status === b.sections[index]?.status,
      )
    )
  }
  if (a.kind === 'error' && b.kind === 'error') return a.error === b.error
  return a.kind === b.kind && a.kind !== 'view' && a.kind !== 'error'
}

function sectionStatus(sections: readonly MarkdownSection[], section: ViewSection): SectionStatus {
  if (section.heading === '' || section.occurrence < 1) return 'unchecked'
  const current = findSection(sections, section.heading, section.occurrence)
  if (!current) return 'missing'
  if (section.sourceHash === undefined) return 'unchecked'
  return current.hash === section.sourceHash ? 'fresh' : 'stale'
}

const STATUS_LABEL: Partial<Record<SectionStatus, string>> = {
  stale: 'Stale',
  missing: 'Missing',
}

const STATUS_HINT: Partial<Record<SectionStatus, string>> = {
  stale: 'This section changed since the view was generated.',
  missing: 'This heading is no longer in the document.',
}

export function GenUIPanel({
  ctx,
  store,
}: {
  ctx: PluginContext
  store: GenUIStore
}): React.ReactElement {
  const state = useSyncExternalStore(store.subscribe, store.getSnapshot)

  const reveal = (section: ViewSection) => {
    // Resolve against the current text: the statuses may lag one debounce behind.
    const target = findSection(
      extractSections(ctx.documents.snapshot().text),
      section.heading,
      section.occurrence,
    )
    if (target) void ctx.commands.execute('editor.reveal', target.start)
  }

  return (
    <div className="genui-panel" data-testid="genui-panel">
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
          {(state.model || state.generatedAt) && (
            <p className="genui-meta">
              {[state.model, state.generatedAt].filter(Boolean).join(' · ')}
            </p>
          )}
          {state.sections.map(({ section, status }, index) => (
            <section key={index} className="genui-section" data-status={status}>
              <header className="genui-section-header">
                <h3>
                  <button
                    type="button"
                    className="genui-heading"
                    disabled={status === 'missing' || section.heading === '' || section.occurrence < 1}
                    onClick={() => reveal(section)}
                  >
                    {section.heading || 'Untitled section'}
                  </button>
                </h3>
                {STATUS_LABEL[status] && (
                  <span className="genui-status" title={STATUS_HINT[status]}>
                    {STATUS_LABEL[status]}
                  </span>
                )}
              </header>
              {section.problems.length > 0 && (
                <ul className="genui-problems" role="alert">
                  {section.problems.map((problem, problemIndex) => (
                    <li key={problemIndex}>{problem}</li>
                  ))}
                </ul>
              )}
              {section.blocks.map((block, blockIndex) => (
                <div key={blockIndex}>{renderBlock(block)}</div>
              ))}
            </section>
          ))}
        </>
      )}
    </div>
  )
}
