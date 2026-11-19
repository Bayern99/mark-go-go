import { useRef, useState } from 'react'
import type { ViewRegistry } from '../core/ViewRegistry'
import { MovingCapsule } from './MovingCapsule'
import type { ViewRegistration } from '../core/types'
import { useViews, ViewHost, ViewStatusIndicator } from './ViewHost'

function tabPosition(view: ViewRegistration): number {
  if (view.id === 'outline.panel') return -1
  if (view.id === 'feynman.panel') return 1
  return 0
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

  const handleSelect = (id: string) => {
    setInternalId(id)
    onSelect?.(id)
  }

  return (
    <aside
      className={`right-pane rp${floating ? ' r-float' : ''}`}
      data-testid="right-pane"
      aria-label="Document panel"
      hidden={!open}
    >
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
          scrollable
          edgeFade
          contentClassName="right-view-content"
        />
      </div>
    </aside>
  )
}
