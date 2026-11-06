import { useEffect, useRef, useState } from 'react'
import type { ViewRegistry } from '../core/ViewRegistry'
import type { ViewLocation, ViewRegistration } from '../core/types'

/** Live view registrations at one location, in registration order. */
export function useViews(views: ViewRegistry, location: ViewLocation): ViewRegistration[] {
  const [entries, setEntries] = useState(() => select(views, location))

  useEffect(() => {
    const update = () => setEntries(select(views, location))
    update()
    const subscription = views.onDidChange(update)
    return () => subscription.dispose()
  }, [views, location])

  return entries
}

function select(views: ViewRegistry, location: ViewLocation): ViewRegistration[] {
  return views.getAll().filter((view) => view.location === location)
}

/** Mounts a registered view into its own element for as long as it is rendered. */
export function ViewHost({
  view,
  className,
}: {
  view: ViewRegistration
  className?: string
}): React.ReactElement {
  const host = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const element = host.current
    if (!element) return
    const mounted = view.mount(element)
    return () => mounted.dispose()
  }, [view])

  return <div className={className} ref={host} />
}
