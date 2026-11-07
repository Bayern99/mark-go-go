import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from 'react'
import { autoGrowTextarea } from '../../editor/noteDialog'
import type { SourceStatus, SourceStore } from './sources'

export interface SourceActions {
  store: SourceStore
  /** Component ID currently highlighted by the right-pane out-of-date list. */
  highlightedId?: string | null
  /**
   * Show a component's source in rich text, selected: where `text` is in its
   * remaining source sections when given and found, otherwise its first
   * remaining source section.
   */
  edit(ids: readonly string[], text?: string): void
  /**
   * Add `text` as a note to the end of the component's first remaining source
   * section. False when it could not be added.
   */
  addNote(ids: readonly string[], text: string): Promise<boolean>
}

export const SourceContext = createContext<SourceActions | null>(null)

const STATUS_LABEL: Partial<Record<SourceStatus, string>> = {
  changed: 'Source changed',
  missing: 'Source not found',
}

const STATUS_HINT: Partial<Record<SourceStatus, string>> = {
  changed: 'The document changed here since this view was made.',
  missing: 'A heading this part was made from is no longer in the document.',
}

const NONE_LEFT = "None of this part's sources are in the document now."

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
  addNote: ((text: string) => Promise<boolean>) | undefined
}

/**
 * What a component's `sources` say now, re-read whenever the document's
 * sections are. Aggregate `status` is still `missing` when any source is gone;
 * `edit` and `addNote` are absent without sources or when none of them is still
 * in the document.
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
  const available = actions.store.hasSource(ids)
  return {
    status,
    headings: actions.store.headings(ids),
    notes: actions.store.notes(ids),
    edit: available ? (text) => actions.edit(ids, text) : undefined,
    addNote: available ? (text) => actions.addNote(ids, text) : undefined,
  }
}

/**
 * The frame every content component of the catalog draws in: where its
 * content came from, whether that source changed since the view was made,
 * the notes on that source, and the ways to edit and note it in the document.
 */
export function SourceFrame({
  source,
  componentId,
  className,
  style,
  title,
  children,
}: {
  source: ComponentSource
  componentId?: string
  className: string
  style?: React.CSSProperties
  title?: string
  children: ReactNode
}): React.ReactElement {
  const actions = useContext(SourceContext)
  const highlighted = Boolean(componentId && actions?.highlightedId === componentId)
  const label = STATUS_LABEL[source.status]
  const [composingNote, setComposingNote] = useState(false)
  const [noteDraft, setNoteDraft] = useState('')
  const [noteFailed, setNoteFailed] = useState(false)
  const noteInputRef = useRef<HTMLTextAreaElement | null>(null)

  useEffect(() => {
    if (!composingNote || !noteInputRef.current) return
    const input = noteInputRef.current
    autoGrowTextarea(input)
    input.focus()
    input.setSelectionRange(input.value.length, input.value.length)
  }, [composingNote])

  const cancelCompose = () => {
    setComposingNote(false)
    setNoteDraft('')
  }

  const saveCompose = async () => {
    const trimmed = noteDraft.trim()
    if (trimmed === '' || !source.addNote) return
    setComposingNote(false)
    setNoteDraft('')
    const added = await source.addNote(trimmed)
    setNoteFailed(!added)
  }

  return (
    <article
      className={`genui-block ${className}${highlighted ? ' is-highlighted' : ''}`}
      style={style}
      data-source-status={source.status}
      data-component-id={componentId}
      data-highlighted={highlighted ? 'true' : undefined}
    >
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
          {!source.edit && source.status === 'missing' ? (
            <span className="genui-source-unavailable">{NONE_LEFT}</span>
          ) : null}
          <button
            type="button"
            className="genui-source-edit"
            disabled={!source.edit}
            title={!source.edit ? NONE_LEFT : undefined}
            onClick={() => source.edit?.()}
          >
            Edit in source
          </button>
          <button
            type="button"
            className="genui-source-note"
            disabled={!source.addNote}
            title={!source.addNote ? NONE_LEFT : undefined}
            onClick={() => {
              setNoteFailed(false)
              setNoteDraft('')
              setComposingNote(true)
            }}
          >
            Note
          </button>
        </footer>
      ) : null}
      {composingNote ? (
        <div className="composer mgo-note-composer" role="region" aria-label="Note composer">
          <textarea
            ref={noteInputRef}
            className="mgo-note-input"
            rows={2}
            aria-label="Note"
            value={noteDraft}
            placeholder="Write a note. It is saved in the document, out of sight in other Markdown viewers."
            onChange={(event) => {
              setNoteDraft(event.target.value)
              autoGrowTextarea(event.currentTarget)
            }}
            onKeyDown={(event) => {
              if (event.key === 'Escape') {
                event.preventDefault()
                event.stopPropagation()
                cancelCompose()
              } else if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
                event.preventDefault()
                event.stopPropagation()
                void saveCompose()
              }
            }}
          />
          <div className="composer-foot composer-bar">
            <span className="composer-hint hint meta">⌘↩ to save · Esc to cancel</span>
            <button type="button" className="pill xs quiet" onClick={cancelCompose}>
              Cancel
            </button>
            <button
              type="submit"
              className="pill xs primary"
              disabled={noteDraft.trim() === ''}
              onClick={() => void saveCompose()}
            >
              Save
            </button>
          </div>
        </div>
      ) : null}
    </article>
  )
}
