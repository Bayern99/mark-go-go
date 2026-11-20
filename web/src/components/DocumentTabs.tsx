import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type RefObject } from 'react'
import { DropdownMenu } from 'radix-ui'
import type { ViewRegistration } from '../core/types'
import type { DocumentInfo } from '../host/types'
import { PluginMenu } from '../plugins/DesktopPlugins'
import { MovingCapsule } from './MovingCapsule'
import { SearchClearButton } from './SearchClearButton'
import { AppTooltip } from './Tooltip'
import { ViewBadgeIndicator, ViewStatusIndicator } from './ViewHost'

export function tabLabel(
  docs: { id: string; name: string; path: string }[],
  doc: { id: string; name: string; path: string },
): string {
  const same = docs.filter((other) => other.name === doc.name)
  if (same.length < 2) return doc.name
  const dirs = doc.path.replace(/\\/g, '/').split('/').slice(0, -1).filter(Boolean)
  for (let count = 1; count <= dirs.length; count += 1) {
    const suffix = dirs.slice(-count).join('/')
    const candidate = `${doc.name} — ${suffix}`
    const unique = same.every((other) => {
      if (other.id === doc.id) return true
      const otherDirs = other.path.replace(/\\/g, '/').split('/').slice(0, -1).filter(Boolean)
      return otherDirs.slice(-count).join('/') !== suffix
    })
    if (unique) return candidate
  }
  return doc.path || doc.name
}

function StatusTooltip({ label, className, testId }: { label: string; className: string; testId?: string }) {
  return (
    <AppTooltip label={label}>
      <span className={className} data-testid={testId} tabIndex={0} role="img" aria-label={label} />
    </AppTooltip>
  )
}

function TabBadges({ doc }: { doc: DocumentInfo }): React.ReactElement {
  return (
    <>
      {doc.edited ? <StatusTooltip label="Unsaved edits" className="edited-dot" /> : null}
      {doc.generating ? (
        <StatusTooltip label="Generating" className="tab-generating" testId={`doc-tab-generating-${doc.id}`} />
      ) : null}
      {doc.conflict ? <StatusTooltip label="Conflict" className="tab-conflict" /> : null}
      {doc.deleted ? <StatusTooltip label="Deleted" className="tab-deleted" /> : null}
      {doc.readOnly ? <StatusTooltip label="Read only" className="tab-readonly" /> : null}
    </>
  )
}

export function DocumentTabs({
  documents,
  activeId,
  recentOrder,
  libraryOpen,
  libraryToggleRef,
  onToggleLibrary,
  onActivate,
  onCloseTab,
  rightView,
  rightShown,
  rightToggleRef,
  onToggleRight,
}: {
  documents: DocumentInfo[]
  activeId: string
  recentOrder: readonly string[]
  libraryOpen: boolean
  libraryToggleRef: RefObject<HTMLButtonElement | null>
  onToggleLibrary: () => void
  onActivate: (id: string, focusTab?: boolean) => void
  onCloseTab: (id: string) => void
  rightView: ViewRegistration | null
  rightShown: boolean
  rightToggleRef: RefObject<HTMLButtonElement | null>
  onToggleRight: () => void
}): React.ReactElement {
  const tabsRef = useRef<HTMLDivElement>(null)
  const tabRefs = useRef(new Map<string, HTMLButtonElement>())
  const tabFocusTarget = useRef<string | null>(null)
  const tabMenuButtonRef = useRef<HTMLButtonElement>(null)
  const tabFilterRef = useRef<HTMLInputElement>(null)
  const tabMenuSelectedId = useRef<string | null>(null)
  const [visibleRange, setVisibleRange] = useState({ start: 0, end: 0 })
  const [tabMenuOpen, setTabMenuOpen] = useState(false)
  const [tabFilter, setTabFilter] = useState('')

  const hiddenDocuments = useMemo(() => {
    const hidden = [...documents.slice(0, visibleRange.start), ...documents.slice(visibleRange.end)]
    const rank = new Map(recentOrder.map((id, index) => [id, index]))
    return hidden.sort(
      (a, b) => (rank.get(a.id) ?? Number.MAX_SAFE_INTEGER) - (rank.get(b.id) ?? Number.MAX_SAFE_INTEGER),
    )
  }, [documents, visibleRange, recentOrder])

  const filteredHiddenDocuments = useMemo(() => {
    const query = tabFilter.trim().toLowerCase()
    if (query === '') return hiddenDocuments
    return hiddenDocuments.filter((doc) => {
      const label = tabLabel(documents, doc)
      return `${label}\n${doc.name}\n${doc.path}`.toLowerCase().includes(query)
    })
  }, [documents, hiddenDocuments, tabFilter])

  const measureTabs = useCallback(() => {
    const bar = tabsRef.current
    if (!bar || documents.length === 0) {
      setVisibleRange((current) => (current.start === 0 && current.end === 0 ? current : { start: 0, end: 0 }))
      return
    }
    const widths = documents.map((doc) => tabRefs.current.get(doc.id)?.offsetWidth ?? 0)
    const gap = 2
    const overflowButton = tabMenuButtonRef.current
    const overflowWidth = overflowButton?.offsetWidth || 32
    const fullBudget = bar.clientWidth + (overflowButton && !overflowButton.hidden ? overflowWidth + gap : 0)
    const allTabsWidth = widths.reduce((used, width, index) => used + width + (index === 0 ? 0 : gap), 0)
    if (allTabsWidth <= fullBudget) {
      setVisibleRange((current) =>
        current.start === 0 && current.end === documents.length ? current : { start: 0, end: documents.length },
      )
      return
    }
    const budget = fullBudget - overflowWidth - gap
    const activeIndex = documents.findIndex((doc) => doc.id === activeId)
    let start = 0
    let end = documents.length
    if (activeIndex >= 0) {
      const throughActive = widths
        .slice(0, activeIndex + 1)
        .reduce((sum, width, index) => sum + width + (index === 0 ? 0 : gap), 0)
      end = activeIndex + 1
      if (throughActive <= budget) {
        let used = throughActive
        while (end < documents.length && used + gap + (widths[end] ?? 0) <= budget) {
          used += gap + (widths[end] ?? 0)
          end += 1
        }
      } else {
        start = activeIndex
        let used = widths[activeIndex] ?? 0
        while (start > 0 && used + gap + (widths[start - 1] ?? 0) <= budget) {
          start -= 1
          used += gap + (widths[start] ?? 0)
        }
      }
    } else {
      let used = 0
      end = 0
      while (end < documents.length && used + (end === 0 ? 0 : gap) + (widths[end] ?? 0) <= budget) {
        used += (end === 0 ? 0 : gap) + (widths[end] ?? 0)
        end += 1
      }
    }
    setVisibleRange((current) => (current.start === start && current.end === end ? current : { start, end }))
  }, [documents, activeId])

  useLayoutEffect(() => {
    const bar = tabsRef.current
    if (!bar) return
    measureTabs()
    const observer = new ResizeObserver(measureTabs)
    observer.observe(bar)
    return () => {
      observer.disconnect()
    }
  }, [measureTabs, visibleRange])

  useLayoutEffect(() => {
    const target = tabFocusTarget.current
    if (!target) return
    if (activeId !== target) {
      tabFocusTarget.current = null
      return
    }
    const tab = tabRefs.current.get(target)
    if (!tab || tab.classList.contains('tab-hidden')) return
    tabFocusTarget.current = null
    tab.focus()
  }, [activeId, visibleRange])

  useEffect(() => {
    let cancelled = false
    void document.fonts?.ready.then(() => {
      if (!cancelled) measureTabs()
    })
    return () => {
      cancelled = true
    }
  }, [measureTabs])

  useEffect(() => {
    if (hiddenDocuments.length === 0) setTabMenuOpen(false)
  }, [hiddenDocuments.length])

  const activate = (id: string, focusTab = false) => {
    tabFocusTarget.current = focusTab ? id : null
    onActivate(id, focusTab)
  }

  const onTabsKey = (event: React.KeyboardEvent) => {
    if (!documents.length) return
    const index = documents.findIndex((doc) => doc.id === activeId)
    if (event.key === 'ArrowRight') {
      event.preventDefault()
      activate(documents[(index + 1) % documents.length].id, true)
    } else if (event.key === 'ArrowLeft') {
      event.preventDefault()
      activate(documents[(index - 1 + documents.length) % documents.length].id, true)
    } else if (event.key === 'Delete' || (event.key === 'w' && (event.metaKey || event.ctrlKey))) {
      event.preventDefault()
      onCloseTab(activeId)
    }
  }

  return (
    <div className="doc-tabs-row">
      <button
        type="button"
        className="pill icon quiet library-toggle"
        aria-label={libraryOpen ? 'Hide navigation' : 'Show navigation'}
        aria-expanded={libraryOpen}
        title={libraryOpen ? 'Hide navigation' : 'Show navigation'}
        ref={libraryToggleRef}
        onClick={onToggleLibrary}
      >
        <i className="ph ph-sidebar-simple" aria-hidden="true" />
      </button>
      <div
        className="doc-tabs"
        role="tablist"
        aria-label="Documents"
        onKeyDown={onTabsKey}
        data-testid="doc-tabs"
        ref={tabsRef}
      >
        <MovingCapsule
          trackRef={tabsRef}
          selector=".doc-tab.sel:not(.tab-hidden)"
          watch={[activeId, visibleRange, documents.length]}
        />
        {documents.map((doc, index) => {
          const hidden = index < visibleRange.start || index >= visibleRange.end
          return (
            <button
              key={doc.id}
              type="button"
              role="tab"
              tabIndex={doc.id === activeId ? 0 : -1}
              data-testid={`doc-tab-${doc.id}`}
              className={`doc-tab${doc.id === activeId ? ' sel' : ''}${hidden ? ' tab-hidden' : ''}`}
              aria-selected={doc.id === activeId}
              aria-hidden={hidden ? true : undefined}
              title={doc.path || doc.name}
              ref={(element) => {
                if (element) tabRefs.current.set(doc.id, element)
                else tabRefs.current.delete(doc.id)
              }}
              onClick={() => activate(doc.id)}
            >
              <i className="ph ph-file-text doc-tab-icon" aria-hidden="true" />
              <AppTooltip label={doc.path || doc.name} disabled={hidden}>
                <span className="nm">{tabLabel(documents, doc)}</span>
              </AppTooltip>
              <TabBadges doc={doc} />
              <span
                className="tab-close"
                role="button"
                aria-label={`Close ${doc.name}`}
                data-testid={`doc-tab-close-${doc.id}`}
                onClick={(event) => {
                  event.stopPropagation()
                  onCloseTab(doc.id)
                }}
              >
                <i className="ph ph-x" aria-hidden="true" />
              </span>
            </button>
          )
        })}
      </div>
      <DropdownMenu.Root
        modal={false}
        open={tabMenuOpen && hiddenDocuments.length > 0}
        onOpenChange={(open) => {
          setTabFilter('')
          setTabMenuOpen(open)
        }}
      >
        <AppTooltip label={`${hiddenDocuments.length} more documents`}>
          <DropdownMenu.Trigger asChild>
            <button
              type="button"
              className="tab-overflow"
              data-testid="doc-tab-overflow"
              aria-label={`${hiddenDocuments.length} more documents`}
              hidden={hiddenDocuments.length === 0}
              ref={tabMenuButtonRef}
            >
              +{hiddenDocuments.length}
            </button>
          </DropdownMenu.Trigger>
        </AppTooltip>
        <DropdownMenu.Portal>
          <DropdownMenu.Content
            className="doc-tab-menu"
            id="doc-tab-menu"
            data-testid="doc-tab-menu"
            aria-label="More documents"
            side="bottom"
            align="end"
            sideOffset={4}
            collisionPadding={8}
            onCloseAutoFocus={(event) => {
              const selectedId = tabMenuSelectedId.current
              if (!selectedId) return
              event.preventDefault()
              tabMenuSelectedId.current = null
              requestAnimationFrame(() => {
                const tab = tabRefs.current.get(selectedId)
                if (tab && !tab.classList.contains('tab-hidden')) tab.focus()
              })
            }}
          >
            <div className="doc-tab-menu-head">
              <span>Open documents</span>
              <span>{hiddenDocuments.length}</span>
            </div>
            <div className="doc-tab-menu-search">
              <i className="ph ph-magnifying-glass" aria-hidden="true" />
              <input
                type="search"
                className="doc-tab-menu-filter"
                data-testid="doc-tab-filter"
                aria-label="Filter open tabs"
                placeholder="Filter open tabs…"
                ref={tabFilterRef}
                value={tabFilter}
                onChange={(event) => setTabFilter(event.target.value)}
                onKeyDownCapture={(event) => {
                  if (['ArrowUp', 'Home', 'End'].includes(event.key)) event.stopPropagation()
                  if (event.key === 'ArrowDown') {
                    event.preventDefault()
                    event.stopPropagation()
                    event.currentTarget
                      .closest('[data-testid="doc-tab-menu"]')
                      ?.querySelector<HTMLElement>('[role="menuitemradio"]')
                      ?.focus()
                  }
                }}
                autoFocus
              />
              {tabFilter ? (
                <SearchClearButton
                  onClear={() => {
                    setTabFilter('')
                    tabFilterRef.current?.focus()
                  }}
                />
              ) : null}
            </div>
            <DropdownMenu.RadioGroup
              className="doc-tab-menu-list"
              value={activeId}
              onValueChange={(id) => {
                tabMenuSelectedId.current = id
                activate(id, true)
              }}
            >
              {filteredHiddenDocuments.length === 0 ? (
                <p className="doc-tab-menu-empty" role="status">
                  No matching tabs
                </p>
              ) : (
                filteredHiddenDocuments.map((doc) => (
                  <DropdownMenu.RadioItem
                    key={doc.id}
                    value={doc.id}
                    className={`doc-tab-menu-item${doc.id === activeId ? ' sel' : ''}`}
                    data-testid={`doc-tab-menu-item-${doc.id}`}
                  >
                    <span className="doc-tab-menu-icon" aria-hidden="true">
                      <i className="ph ph-file-text" />
                    </span>
                    <AppTooltip label={doc.path || doc.name}>
                      <span className="doc-tab-menu-copy">
                        <span className="nm">{tabLabel(documents, doc)}</span>
                        <span className="doc-tab-menu-path">{doc.path || 'Untitled'}</span>
                      </span>
                    </AppTooltip>
                    <TabBadges doc={doc} />
                  </DropdownMenu.RadioItem>
                ))
              )}
            </DropdownMenu.RadioGroup>
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
      {rightView && (
        <button
          type="button"
          className="pill icon quiet rp-tg"
          data-testid="right-pane-toggle"
          data-rp-toggle
          ref={rightToggleRef}
          aria-expanded={rightShown}
          aria-controls="right-pane-panel"
          aria-label={rightShown ? `Hide ${rightView.title}` : `Show ${rightView.title}`}
          title={`${rightShown ? 'Hide' : 'Show'} ${rightView.title} (⌥⌘0)`}
          onClick={onToggleRight}
        >
          <i className="ph ph-sidebar-simple rp-tg-icon" aria-hidden="true" />
          {rightView.badge && <ViewBadgeIndicator badge={rightView.badge} testId="right-pane-badge" />}
          {rightView.status && <ViewStatusIndicator status={rightView.status} testId="right-pane-status" />}
        </button>
      )}
      <PluginMenu />
    </div>
  )
}
