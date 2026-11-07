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

function AppLogo() {
  return (
    <svg className="ws-logo" width="16" height="16" viewBox="0 0 32 32" fill="none" aria-hidden="true">
      <rect x="1" y="1" width="30" height="30" rx="7" fill="#1e2330" stroke="#333b4e" strokeWidth="1" />
      <path d="M7 9.5C7 8.95 7.45 8.5 8 8.5H10.5C10.9 8.5 11.26 8.74 11.42 9.11L14 15.2L12 19L8.5 10.8V22.5C8.5 23.05 8.05 23.5 7.5 23.5H7.5C6.95 23.5 6.5 23.05 6.5 22.5V9.5C6.5 8.95 6.95 8.5 7.5 8.5Z" fill="#3b82f6" />
      <path d="M12 9.5C12 8.95 12.45 8.5 13 8.5H14.8C15.25 8.5 15.65 8.8 15.78 9.23L18.8 15.5C18.95 15.82 18.95 16.18 18.8 16.5L15.78 22.77C15.65 23.2 15.25 23.5 14.8 23.5H13C12.45 23.5 12 23.05 12 22.5V21L15.5 16L12 9.5Z" fill="#8b5cf6" />
      <path d="M18.5 9.5C18.5 8.95 18.95 8.5 19.5 8.5H21.3C21.75 8.5 22.15 8.8 22.28 9.23L25.3 15.5C25.45 15.82 25.45 16.18 25.3 16.5L22.28 22.77C22.15 23.2 21.75 23.5 21.3 23.5H19.5C18.95 23.5 18.5 23.05 18.5 22.5V21L22 16L18.5 9.5Z" fill="#06b6d4" />
    </svg>
  )
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
            <div className="ws-name">
              <AppLogo />
              <span>Mark Go Go</span>
            </div>
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
