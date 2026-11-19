import { useLayoutEffect, useRef, useState } from 'react'
import type { ViewRegistry } from '../core/ViewRegistry'
import { MovingCapsule } from './MovingCapsule'
import type { ViewRegistration } from '../core/types'
import { useViews, ViewHost, ViewStatusIndicator } from './ViewHost'

function tabPosition(view: ViewRegistration): number {
  if (view.id === 'outline.panel') return -1
  if (view.id === 'feynman.panel') return 1
  return 0
}

function ResearchDivider() {
  const drag = useRef<{ x: number; width: number } | null>(null)
  const [width, setWidth] = useState(() => {
    const saved = Number(localStorage.getItem('mgo.researchWidth'))
    return saved >= 310 && saved <= 800 ? saved : 376
  })
  useLayoutEffect(() => {
    document.documentElement.style.setProperty('--research-width', width + 'px')
  }, [width])
  const resize = (size: number) => {
    const value = Math.min(800, Math.max(310, Math.min(window.innerWidth * 0.56, size)))
    setWidth(value)
    return value
  }
  return <div role="separator" aria-label="Resize research panel" aria-orientation="vertical"
    aria-valuemin={310} aria-valuemax={Math.min(800, Math.floor(window.innerWidth * 0.56))} aria-valuenow={Math.round(width)}
    tabIndex={0} className="fy-resize" onPointerDown={e => {
      if (e.button !== 0) return
      drag.current = { x: e.clientX, width: e.currentTarget.parentElement!.getBoundingClientRect().width }
      e.currentTarget.setPointerCapture(e.pointerId); e.preventDefault()
    }} onPointerMove={e => {
      if (drag.current) resize(drag.current.width + drag.current.x - e.clientX)
    }} onLostPointerCapture={() => { drag.current = null; localStorage.setItem('mgo.researchWidth', String(width)) }}
    onDoubleClick={() => localStorage.setItem('mgo.researchWidth', String(resize(376)))}
    onKeyDown={e => {
      if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
      e.preventDefault()
      localStorage.setItem('mgo.researchWidth', String(resize(e.currentTarget.parentElement!.getBoundingClientRect().width + (e.key === 'ArrowLeft' ? 24 : -24))))
    }} />
}

export function RightPane({
  views,
  activeId,
  onSelect,
  open = true,
  floating = false,
  onClose,
}: {
  views: ViewRegistry
  activeId?: string | null
  onSelect?: (id: string) => void
  open?: boolean
  floating?: boolean
  onClose?: () => void
}): React.ReactElement | null {
  const rawEntries = useViews(views, 'right')
  const entries = [...rawEntries].sort((a, b) => tabPosition(a) - tabPosition(b))
  const [internalId, setInternalId] = useState<string | null>(null)
  const tabsRef = useRef<HTMLDivElement>(null)

  if (entries.length === 0) return null

  const selectedId = activeId !== undefined ? activeId : internalId
  const active = entries.find((view) => view.id === selectedId) ?? (entries[0] as ViewRegistration)

  const research = active.id === 'feynman.panel'

  const handleSelect = (id: string) => {
    setInternalId(id)
    onSelect?.(id)
  }

  return (
    <aside
      className={`right-pane rp${floating ? ' r-float' : ''}${research ? ' fy-rail' : ''}`}
      data-testid="right-pane"
      aria-label="Document panel"
      hidden={!open}
    >
      {research && !floating && <ResearchDivider />}
      <div className="rp-bar">
        <div className="seg" role="tablist" aria-label="Panel" ref={tabsRef}>
          <MovingCapsule trackRef={tabsRef} watch={[selectedId, open, floating, entries.length]} />
          {entries.map((view) => {
            const isSelected = view.id === active.id
            return (
              <button
                key={view.id}
                type="button"
                role="tab"
                data-testid={`right-tab-${view.id}`}
                aria-selected={isSelected}
                onClick={() => handleSelect(view.id)}
              >
                <span>{view.title}</span>
                {view.status && <ViewStatusIndicator status={view.status} testId={`right-tab-dot-${view.id}`} />}
              </button>
            )
          })}
        </div>
        {floating && onClose ? (
          <button
            type="button"
            className="pill icon quiet rp-close"
            data-testid="right-pane-close"
            aria-label="Close document panel"
            title="Close panel (Esc)"
            autoFocus
            onClick={onClose}
          >
            <i className="ph ph-x" aria-hidden="true" />
          </button>
        ) : null}
      </div>
      <div
        className="rp-body right-view"
        data-view-id={active.id}
        role="tabpanel"
        aria-label={active.title}
      >
        <ViewHost
          key={active.id}
          view={active}
          className="right-view-body"
          scrollable={!research}
          edgeFade={!research}
          contentClassName="right-view-content"
        />
      </div>
    </aside>
  )
}
