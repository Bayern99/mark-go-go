import type { HTMLAttributes, ReactElement } from 'react'
import { OverlayScrollbarsComponent } from 'overlayscrollbars-react'
import type { EventListeners, OverlayScrollbars, PartialOptions } from 'overlayscrollbars'

export interface ScrollAreaProps extends HTMLAttributes<HTMLDivElement> {
  axis?: 'x' | 'y'
  edgeFade?: boolean
  contentClassName?: string
  onViewport?: (viewport: HTMLElement | null) => void
  onViewportScroll?: (event: Event) => void
}

function syncEdgeFade(instance: OverlayScrollbars, axis: 'x' | 'y', enabled: boolean): void {
  const { host, scrollOffsetElement } = instance.elements()
  if (!enabled || axis !== 'y') {
    delete host.dataset.scrollTop
    delete host.dataset.scrollBottom
    return
  }
  const top = scrollOffsetElement.scrollTop > 2
  const bottom =
    scrollOffsetElement.scrollTop + scrollOffsetElement.clientHeight <
    scrollOffsetElement.scrollHeight - 2
  host.dataset.scrollTop = String(top)
  host.dataset.scrollBottom = String(bottom)
}

export function ScrollArea({
  axis = 'y',
  edgeFade = false,
  className,
  contentClassName,
  onViewport,
  onViewportScroll,
  children,
  ...props
}: ScrollAreaProps): ReactElement {
  const options: PartialOptions = {
    overflow: {
      x: axis === 'x' ? 'scroll' : 'hidden',
      y: axis === 'y' ? 'scroll' : 'hidden',
    },
    scrollbars: {
      theme: 'os-theme-mgo',
      autoHide: 'move',
      autoHideDelay: 600,
      autoHideSuspend: false,
      dragScroll: true,
    },
  }
  const events: EventListeners = {
    initialized: (instance) => {
      syncEdgeFade(instance, axis, edgeFade)
      onViewport?.(instance.elements().scrollOffsetElement)
    },
    updated: (instance) => syncEdgeFade(instance, axis, edgeFade),
    destroyed: () => onViewport?.(null),
    scroll: (instance, event) => {
      syncEdgeFade(instance, axis, edgeFade)
      onViewportScroll?.(event)
    },
  }

  return (
    <OverlayScrollbarsComponent
      element="div"
      className={['scroll-area', edgeFade ? 'scroll-edge-fade' : '', className].filter(Boolean).join(' ')}
      options={options}
      events={events}
      defer={false}
      data-overlayscrollbars-initialize
      {...props}
    >
      <div className={['scroll-area-content', contentClassName].filter(Boolean).join(' ')}>{children}</div>
    </OverlayScrollbarsComponent>
  )
}
