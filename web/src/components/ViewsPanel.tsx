import { useEffect, useRef, useState } from 'react'
import type { ViewRegistry } from '../core/ViewRegistry'
import type { ViewRegistration } from '../core/types'

export function ViewsPanel({ views }: { views: ViewRegistry }): React.ReactElement | null {
  const [entries, setEntries] = useState<ViewRegistration[]>(() => views.getAll())

  useEffect(() => {
    setEntries(views.getAll())
    const subscription = views.onDidChange(() => setEntries(views.getAll()))
    return () => subscription.dispose()
  }, [views])

  if (entries.length === 0) return null

  return (
    <>
      {entries.map((view) => (
        <RegisteredView key={view.id} view={view} />
      ))}
    </>
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
