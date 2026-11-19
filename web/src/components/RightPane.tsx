import { useLayoutEffect, useMemo, useRef, useState } from 'react'
import { ViewRegistry } from '../core/ViewRegistry'
import { useViews, ViewHost } from './ViewHost'
import {
  readSavedRightWidth,
  RIGHT_DEFAULT,
  RIGHT_MAX,
  RIGHT_MAX_RATIO,
  RIGHT_MIN,
  saveRightWidth,
} from './paneLayout'

function ResearchDivider({
  paneWidth,
  targetWidth,
  maxWidth,
  panelTitle,
  onWidthChange,
}: {
  paneWidth: number
  targetWidth: number
  maxWidth: number
  panelTitle: string
  onWidthChange: (width: number) => void
}) {
  const drag = useRef<{ x: number; width: number } | null>(null)
  const latestWidth = useRef(targetWidth)
  latestWidth.current = targetWidth

  const resize = (size: number) => {
    const value = Math.min(maxWidth, Math.max(RIGHT_MIN, size))
    latestWidth.current = value
    onWidthChange(value)
    return value
  }

  return (
    <div
      role="separator"
      aria-label={`Resize ${panelTitle.toLowerCase()} panel`}
      aria-orientation="vertical"
      aria-valuemin={RIGHT_MIN}
      aria-valuemax={maxWidth}
      aria-valuenow={Math.round(paneWidth)}
      tabIndex={0}
      className="fy-resize"
      onPointerDown={(e) => {
        if (e.button !== 0) return
        drag.current = { x: e.clientX, width: e.currentTarget.parentElement!.getBoundingClientRect().width }
        e.currentTarget.setPointerCapture(e.pointerId)
        e.preventDefault()
      }}
      onPointerMove={(e) => {
        if (drag.current) resize(drag.current.width + drag.current.x - e.clientX)
      }}
      onLostPointerCapture={() => {
        drag.current = null
        saveRightWidth(latestWidth.current)
      }}
      onDoubleClick={() => saveRightWidth(resize(RIGHT_DEFAULT))}
      onKeyDown={(e) => {
        if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
        e.preventDefault()
        const current = e.currentTarget.parentElement!.getBoundingClientRect().width
        saveRightWidth(resize(current + (e.key === 'ArrowLeft' ? 24 : -24)))
      }}
    />
  )
}

export function RightPane({
  views,
  children,
  open,
  floating,
  onClose,
  width: controlledWidth,
  targetWidth: controlledTargetWidth,
  maxWidth: controlledMaxWidth,
  onWidthChange,
  panelId = 'right-pane-panel',
}: {
  views?: ViewRegistry
  children?: React.ReactNode
  open: boolean
  floating: boolean
  onClose: () => void
  width?: number
  targetWidth?: number
  maxWidth?: number
  onWidthChange?: (width: number) => void
  panelId?: string
}) {
  const emptyViews = useMemo(() => new ViewRegistry(), [])
  const rightViews = useViews(views ?? emptyViews, 'right')
  const activeView = rightViews[0] ?? null
  const [localWidth, setLocalWidth] = useState(readSavedRightWidth)
  const targetWidth = controlledTargetWidth ?? localWidth
  const paneWidth = floating ? targetWidth : (controlledWidth ?? localWidth)
  const fallbackMax =
    typeof window === 'undefined'
      ? RIGHT_MAX
      : Math.max(RIGHT_MIN, Math.min(RIGHT_MAX, Math.floor(window.innerWidth * RIGHT_MAX_RATIO)))
  const maxWidth = controlledMaxWidth ?? fallbackMax

  useLayoutEffect(() => {
    document.documentElement.style.setProperty('--research-width', `${paneWidth}px`)
  }, [paneWidth])

  if (views && rightViews.length === 0 && !children) return null

  const title = activeView?.title ?? 'Panel'
  const handleWidthChange = onWidthChange ?? setLocalWidth

  return (
    <aside
      id={panelId}
      className={`right-pane fy-rail${floating ? ' r-float' : ''}`}
      data-testid="right-pane"
      aria-label={title}
      hidden={!open}
      style={!floating ? { width: paneWidth, minWidth: paneWidth, maxWidth: paneWidth } : undefined}
    >
      {!floating && (
        <ResearchDivider
          paneWidth={paneWidth}
          targetWidth={targetWidth}
          maxWidth={maxWidth}
          panelTitle={title}
          onWidthChange={handleWidthChange}
        />
      )}
      {floating && (
        <button
          type="button"
          className="pill icon quiet rp-close"
          data-testid="right-pane-close"
          aria-label={`Close ${title}`}
          title={`Close ${title} (Esc)`}
          autoFocus
          onClick={onClose}
        >
          <i className="ph ph-x" aria-hidden="true" />
        </button>
      )}
      {activeView ? (
        <div className="right-view-host" data-view-id={activeView.id}>
          <ViewHost key={activeView.id} view={activeView} className="right-view" />
        </div>
      ) : (
        children
      )}
    </aside>
  )
}
