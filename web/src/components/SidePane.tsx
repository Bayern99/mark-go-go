import { useEffect, useRef, useState } from 'react'
import type { ViewRegistry } from '../core/ViewRegistry'
import type { ViewRegistration } from '../core/types'

type ViewLocation = 'left' | 'right'

/** Live view registrations for one sidebar, filtered by location. */
function useSideViews(views: ViewRegistry, location: ViewLocation): ViewRegistration[] {
  const [entries, setEntries] = useState<ViewRegistration[]>(() => views.getAll())

  useEffect(() => {
    const select = () => {
      setEntries(views.getAll().filter((view) => (view.location ?? 'right') === location))
    }
    select()
    const subscription = views.onDidChange(select)
    return () => subscription.dispose()
  }, [views, location])

  return entries
}

export function SidePane({
  views,
  location,
}: {
  views: ViewRegistry
  location: ViewLocation
}): React.ReactElement | null {
  const entries = useSideViews(views, location)
  if (entries.length === 0) return null

  return (
    <aside
      className={`side-pane side-pane-${location}`}
      data-testid={location === 'left' ? 'left-pane' : 'side-pane'}
    >
      {entries.map((view) => (
        <RegisteredView key={view.id} view={view} />
      ))}
    </aside>
  )
}

function RegisteredView({ view }: { view: ViewRegistration }): React.ReactElement {
  const host = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const element = host.current
    if (!element) return
    const mounted = view.mount(element)
    return () => mounted.dispose()
  }, [view])

  return (
    <section className="side-view" data-view-id={view.id}>
      <header className="side-view-header">{view.title}</header>
      <div className="side-view-body" ref={host} />
    </section>
  )
}
