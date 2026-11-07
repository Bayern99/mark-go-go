import { createContext, useContext, useState, useSyncExternalStore, type ReactNode } from 'react'
import type { SourceStatus, SourceStore } from './sources'

export interface SourceActions {
  store: SourceStore
  /**
   * Show a component's source in rich text, selected: where `text` is in its
   * source sections when given and found, otherwise its first source section.
   */
  edit(ids: readonly string[], text?: string): void
  /**
   * Ask for a note and add it to the end of the component's first source
   * section. False when it could not be added; true also when cancelled.
   */
  addNote(ids: readonly string[]): Promise<boolean>
}

export const SourceContext = createContext<SourceActions | null>(null)

const STATUS_LABEL: Partial<Record<SourceStatus, string>> = {
  changed: 'Source changed',
  missing: 'Source not found',
}

const STATUS_HINT: Partial<Record<SourceStatus, string>> = {
  changed: 'The document changed here since this view was made.',
  missing: 'The heading this part was made from is no longer in the document.',
}

const noSubscription = () => () => {}
const noRevision = () => 0

export interface ComponentSource {
  status: SourceStatus
  headings: string[]
  /** The notes in its source sections. */
  notes: string[]
  /** Opens the source in the editor; absent when there is nowhere to go. */
  edit: ((text?: string) => void) | undefined
  /** Adds a note to the source; absent when there is nowhere to put it. */
  addNote: (() => Promise<boolean>) | undefined
}

/**
 * What a component's `sources` say now, re-read whenever the document's
 * sections are; `edit` and `addNote` are absent without sources or when one is
 * missing.
 */
export function useComponentSource(sources: readonly string[] | undefined): ComponentSource {
  const actions = useContext(SourceContext)
  useSyncExternalStore(
    actions?.store.subscribe ?? noSubscription,
    actions?.store.getRevision ?? noRevision,
  )
  const ids = sources ?? []
  if (!actions || ids.length === 0) {
    return { status: 'unknown', headings: [], notes: [], edit: undefined, addNote: undefined }
  }
  const status = actions.store.status(ids)
  return {
    status,
    headings: actions.store.headings(ids),
    notes: actions.store.notes(ids),
    edit: status === 'missing' ? undefined : (text) => actions.edit(ids, text),
    addNote: status === 'missing' ? undefined : () => actions.addNote(ids),
  }
}

/**
 * The frame every content component of the catalog draws in: where its
 * content came from, whether that source changed since the view was made,
 * the notes on that source, and the ways to edit and note it in the document.
 */
export function SourceFrame({
  source,
  className,
  style,
  title,
  children,
}: {
  source: ComponentSource
  className: string
  style?: React.CSSProperties
  title?: string
  children: ReactNode
}): React.ReactElement {
  const label = STATUS_LABEL[source.status]
  const [noteFailed, setNoteFailed] = useState(false)
  return (
    <article className={`genui-block ${className}`} style={style} data-source-status={source.status}>
      {title ? <h4>{title}</h4> : null}
      {children}
      {source.notes.length > 0 ? (
        <ul className="genui-notes" aria-label="Notes">
          {source.notes.map((note, index) => (
            <li key={index} className="genui-note">
              {note}
            </li>
          ))}
        </ul>
      ) : null}
      {source.headings.length > 0 ? (
        <footer className="genui-source">
          <span className="genui-source-from">From {source.headings.join(', ')}</span>
          {label ? (
            <span className="genui-source-status" title={STATUS_HINT[source.status]}>
              {label}
            </span>
          ) : null}
          {noteFailed ? (
            <span className="genui-note-failed" role="alert">
              The note could not be added: the document is read only.
            </span>
          ) : null}
          <button
            type="button"
            className="genui-source-edit"
            disabled={!source.edit}
            onClick={() => source.edit?.()}
          >
            Edit in source
          </button>
          <button
            type="button"
            className="genui-source-note"
            disabled={!source.addNote}
            onClick={() => {
              void source.addNote?.().then((added) => setNoteFailed(!added))
            }}
          >
            Note
          </button>
        </footer>
      ) : null}
    </article>
  )
}
