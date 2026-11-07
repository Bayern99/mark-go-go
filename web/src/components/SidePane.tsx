import { useState } from 'react'
import type { ViewRegistry } from '../core/ViewRegistry'
import type { DocState } from '../host/types'
import { useViews, ViewHost } from './ViewHost'

function workspaceFolder(path: string | undefined): string {
  if (!path) return 'Workspace'
  const normalized = path.replace(/\\/g, '/')
  const lastSlash = normalized.lastIndexOf('/')
  if (lastSlash <= 0) return normalized
  return normalized.slice(0, lastSlash)
}

/**
 * The left workspace column: workspace header, filter input, current open
 * document item, and any registered `location: 'left'` views.
 */
export function SidePane({
  views,
  doc,
}: {
  views: ViewRegistry
  doc?: DocState | null
}): React.ReactElement | null {
  const entries = useViews(views, 'left')
  const [filter, setFilter] = useState('')

  if (doc === undefined && entries.length === 0) return null

  const docName = doc?.name ?? 'Loading…'
  const docPath = doc?.path ?? ''
  const query = filter.trim().toLowerCase()
  const showDoc =
    query === '' ||
    docName.toLowerCase().includes(query) ||
    docPath.toLowerCase().includes(query)

  return (
    <aside className="side-pane left-pane side" data-testid="left-pane" aria-label="Workspace">
      {doc !== undefined ? (
        <>
          <div className="ws">
            <div className="ws-name">Mark Go Go</div>
            <div className="ws-meta meta">
              <span className="p">{workspaceFolder(doc?.path)}</span>
              <span>·</span>
              <span>1 file</span>
            </div>
          </div>
          <label className={`filter${filter ? ' has' : ''}`}>
            <input
              type="text"
              placeholder="Filter files and headings"
              aria-label="Filter files and headings"
              autoComplete="off"
              value={filter}
              onChange={(event) => setFilter(event.target.value)}
            />
            {filter ? (
              <button
                type="button"
                className="clear"
                aria-label="Clear filter"
                onClick={() => setFilter('')}
              >
                ×
              </button>
            ) : null}
          </label>
          <div className="list" role="tree" aria-label="Files">
            {showDoc ? (
              <button
                type="button"
                className="row file sel"
                data-testid="workspace-current-file"
                aria-current="true"
                title={docPath || docName}
              >
                <span className="ed-slot">
                  {doc?.edited ? <span className="edited" title="Unsaved edits" /> : null}
                </span>
                <span className="body">
                  <span className="l1">
                    <span className="nm">{docName}</span>
                  </span>
                  <span className="l2">
                    <span className="s">{docPath || 'Untitled'}</span>
                  </span>
                </span>
              </button>
            ) : null}
          </div>
        </>
      ) : null}
      {entries.map((view) => (
        <section key={view.id} className="side-view" data-view-id={view.id} aria-label={view.title}>
          <header className="side-view-header">{view.title}</header>
          <ViewHost view={view} className="side-view-body" />
        </section>
      ))}
    </aside>
  )
}
