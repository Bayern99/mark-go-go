import type { ViewRegistry } from '../core/ViewRegistry'
import { useViews, ViewHost } from './ViewHost'

/** The left sidebar: every `location: 'left'` view, or nothing when there is none. */
export function SidePane({ views }: { views: ViewRegistry }): React.ReactElement | null {
  const entries = useViews(views, 'left')
  if (entries.length === 0) return null

  return (
    <aside className="side-pane" data-testid="left-pane">
      {entries.map((view) => (
        <section key={view.id} className="side-view" data-view-id={view.id} aria-label={view.title}>
          <header className="side-view-header">{view.title}</header>
          <ViewHost view={view} className="side-view-body" />
        </section>
      ))}
    </aside>
  )
}
