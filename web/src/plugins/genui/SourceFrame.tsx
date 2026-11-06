import { createContext, useContext, useSyncExternalStore, type ReactNode } from 'react'
import type { SourceStatus, SourceStore } from './sources'

export interface SourceActions {
  store: SourceStore
  /** Show the first source of a component in rich text, selected. */
  edit(ids: readonly string[]): void
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

/**
 * The frame every content component of the catalog draws in: where its
 * content came from, whether that source changed since the view was made,
 * and the way to edit it in the document.
 */
export function SourceFrame({
  sources,
  className,
  style,
  children,
}: {
  sources: readonly string[] | undefined
  className: string
  style?: React.CSSProperties
  children: ReactNode
}): React.ReactElement {
  const actions = useContext(SourceContext)
  // Re-render when the document's sections are re-read.
  useSyncExternalStore(
    actions?.store.subscribe ?? noSubscription,
    actions?.store.getRevision ?? noRevision,
  )
  const ids = sources ?? []
  const status = actions && ids.length > 0 ? actions.store.status(ids) : 'unknown'
  const headings = actions ? actions.store.headings(ids) : []
  const label = STATUS_LABEL[status]

  return (
    <article className={`genui-block ${className}`} style={style} data-source-status={status}>
      {children}
      {actions && ids.length > 0 ? (
        <footer className="genui-source">
          <span className="genui-source-from">From {headings.join(', ')}</span>
          {label ? (
            <span className="genui-source-status" title={STATUS_HINT[status]}>
              {label}
            </span>
          ) : null}
          <button
            type="button"
            className="genui-source-edit"
            disabled={status === 'missing'}
            onClick={() => actions.edit(ids)}
          >
            Edit in source
          </button>
        </footer>
      ) : null}
    </article>
  )
}
