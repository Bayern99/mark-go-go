import { createContext, useContext, useSyncExternalStore, type ReactNode } from 'react'
import type { SourceStatus, SourceStore } from './sources'

export interface SourceActions {
  store: SourceStore
  /**
   * Show a component's source in rich text, selected: where `text` is in its
   * source sections when given and found, otherwise its first source section.
   */
  edit(ids: readonly string[], text?: string): void
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
  /** Opens the source in the editor; absent when there is nowhere to go. */
  edit: ((text?: string) => void) | undefined
}

/**
 * What a component's `sources` say now, re-read whenever the document's
 * sections are; `edit` is absent without sources or when one is missing.
 */
export function useComponentSource(sources: readonly string[] | undefined): ComponentSource {
  const actions = useContext(SourceContext)
  useSyncExternalStore(
    actions?.store.subscribe ?? noSubscription,
    actions?.store.getRevision ?? noRevision,
  )
  const ids = sources ?? []
  if (!actions || ids.length === 0) return { status: 'unknown', headings: [], edit: undefined }
  const status = actions.store.status(ids)
  return {
    status,
    headings: actions.store.headings(ids),
    edit: status === 'missing' ? undefined : (text) => actions.edit(ids, text),
  }
}

/**
 * The frame every content component of the catalog draws in: where its
 * content came from, whether that source changed since the view was made,
 * and the way to edit it in the document.
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
  return (
    <article className={`genui-block ${className}`} style={style} data-source-status={source.status}>
      {title ? <h4>{title}</h4> : null}
      {children}
      {source.headings.length > 0 ? (
        <footer className="genui-source">
          <span className="genui-source-from">From {source.headings.join(', ')}</span>
          {label ? (
            <span className="genui-source-status" title={STATUS_HINT[source.status]}>
              {label}
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
        </footer>
      ) : null}
    </article>
  )
}
