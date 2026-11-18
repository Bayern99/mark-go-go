import { useEffect, useRef, useState } from 'react'
import type { ViewRegistry } from '../core/ViewRegistry'
import type { ViewLocation, ViewRegistration } from '../core/types'
import { ScrollArea } from './ScrollArea'

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

export function ViewHost({
  view,
  className,
  contentClassName,
  scrollable = false,
}: {
  view: ViewRegistration
  className?: string
  contentClassName?: string
  scrollable?: boolean
}): React.ReactElement {
  const host = useRef<HTMLDivElement>(null)
  const [scrollElement, setScrollElement] = useState<HTMLElement | null>(null)

  useEffect(() => {
    const element = host.current
    if (!element || (scrollable && !scrollElement)) return
    const mounted = view.mount(element, scrollElement ?? undefined)
    return () => mounted.dispose()
  }, [view, scrollable, scrollElement])

  const mount = <div className={scrollable ? 'view-mount' : className} ref={host} />
  if (!scrollable) return mount
  return (
    <ScrollArea className={className} contentClassName={contentClassName} onViewport={setScrollElement}>
      {mount}
    </ScrollArea>
  )
}
