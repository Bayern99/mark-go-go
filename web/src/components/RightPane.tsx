import { useRef, useState, useSyncExternalStore } from 'react'
import type { ViewRegistry } from '../core/ViewRegistry'
import { MovingCapsule } from './MovingCapsule'
import type { ViewRegistration, ViewStatusSource } from '../core/types'
import { useViews, ViewHost } from './ViewHost'

const emptySubscribe = () => () => {}
const emptySnapshot = () => null

function RightTabDot({
  viewId,
  status,
}: {
  viewId: string
  status?: ViewStatusSource
}): React.ReactElement | null {
  const dot = useSyncExternalStore(
    status ? status.subscribe : emptySubscribe,
    status ? status.getSnapshot : emptySnapshot,
  )
  if (!dot) return null
  return (
    <span
      className={`rdot${dot.tone === 'danger' ? ' bad' : ''}`}
      data-testid={`right-tab-dot-${viewId}`}
      data-tone={dot.tone}
      title={dot.title}
    />
  )
}

function orderRightViews(entries: ViewRegistration[]): ViewRegistration[] {
  const outlineIndex = entries.findIndex((view) => view.id === 'outline.panel')
  if (outlineIndex <= 0) return entries
  const outline = entries[outlineIndex] as ViewRegistration
  return [outline, ...entries.slice(0, outlineIndex), ...entries.slice(outlineIndex + 1)]
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
  const entries = orderRightViews(rawEntries)
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
                <RightTabDot viewId={view.id} status={view.status} />
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
          scrollable={active.id !== 'genui.panel'}
          edgeFade={active.id !== 'genui.panel'}
          contentClassName="right-view-content"
        />
      </div>
    </aside>
  )
}
