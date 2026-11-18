import type { HTMLAttributes, ReactElement } from 'react'
import { OverlayScrollbarsComponent } from 'overlayscrollbars-react'
import type { EventListeners, PartialOptions } from 'overlayscrollbars'

export interface ScrollAreaProps extends HTMLAttributes<HTMLDivElement> {
  axis?: 'x' | 'y'
  contentClassName?: string
  onViewport?: (viewport: HTMLElement | null) => void
  onViewportScroll?: (event: Event) => void
}

export function ScrollArea({
  axis = 'y',
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
    initialized: (instance) => onViewport?.(instance.elements().scrollOffsetElement),
    destroyed: () => onViewport?.(null),
    scroll: (_instance, event) => onViewportScroll?.(event),
  }

  return (
    <OverlayScrollbarsComponent
      element="div"
      className={['scroll-area', className].filter(Boolean).join(' ')}
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
