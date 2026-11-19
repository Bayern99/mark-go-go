import { useEffect, useLayoutEffect, useRef, useState, useSyncExternalStore } from 'react'
import type { ViewRegistry } from '../core/ViewRegistry'
import type { ViewBadgeSource, ViewLocation, ViewRegistration, ViewStatusSource } from '../core/types'
import { ScrollArea } from './ScrollArea'

export function ViewStatusIndicator({ status, testId }: { status: ViewStatusSource; testId: string }) {
  const dot = useSyncExternalStore(status.subscribe, status.getSnapshot)
  if (!dot) return null
  return (
    <span
      className={`rdot${dot.tone === 'danger' ? ' bad' : ''}`}
      data-testid={testId}
      data-tone={dot.tone}
      role="img"
      aria-label={dot.title ?? (dot.tone === 'danger' ? 'Error' : 'Warning')}
      title={dot.title}
    />
  )
}

export function ViewBadgeIndicator({ badge, testId }: { badge: ViewBadgeSource; testId: string }) {
  const count = useSyncExternalStore(badge.subscribe, badge.getSnapshot)
  if (!count || count <= 0) return null
  return (
    <span className="rp-badge meta" data-testid={testId} aria-label={`${count} pending`}>
      {count}
    </span>
  )
}

export function useViews(views: ViewRegistry, location: ViewLocation): ViewRegistration[] {
  const [entries, setEntries] = useState(() => select(views, location))

  useLayoutEffect(() => {
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
  edgeFade = false,
}: {
  view: ViewRegistration
  className?: string
  contentClassName?: string
  scrollable?: boolean
  edgeFade?: boolean
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
    <ScrollArea
      className={className}
      contentClassName={contentClassName}
      edgeFade={edgeFade}
      onViewport={setScrollElement}
    >
      {mount}
    </ScrollArea>
  )
}
