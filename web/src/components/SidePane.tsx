import { useEffect, useMemo, useState } from 'react'
import type { ViewRegistry } from '../core/ViewRegistry'
import type {
  DocumentInfo,
  NavEntry,
  NavSearchResult,
  NavSnapshot,
  WorkspaceAdapter,
  WorkspaceSnapshot,
} from '../host/types'
import { breadcrumbs } from '../workspace/nav'
import { useViews, ViewHost } from './ViewHost'
import { ScrollArea } from './ScrollArea'
import { AppTooltip } from './Tooltip'
import { DropdownMenu } from 'radix-ui'

const SIDEBAR_MIN = 190
const SIDEBAR_MAX = 420
const SIDEBAR_DEFAULT = 248
const CENTER_MIN = 310
const SIDEBAR_FLOOR = 140

const TREE_INDENT = 16
const TREE_INDENT_BASE = 8
const TREE_INDENT_MAX = 0.4
const ROW_TEXT_CHROME = 46
const META_DATE_MIN_TEXT = 160
const META_NOTES_MIN_TEXT = 96

function treeIndent(depth: number, contentWidth: number): number {
  return Math.min(TREE_INDENT_BASE + Math.max(0, depth) * TREE_INDENT, contentWidth * TREE_INDENT_MAX)
}

function indentStyle(depth: number): React.CSSProperties {
  return {
    paddingInlineStart: `min(${TREE_INDENT_BASE + Math.max(0, depth) * TREE_INDENT}px, ${TREE_INDENT_MAX * 100}%)`,
  }
}

function metaRoom(depth: number, paneWidth?: number): { date: boolean; notes: boolean } {
  if (paneWidth === undefined) return { date: true, notes: true }
  const contentWidth = Math.max(0, paneWidth - 16)
  const text = contentWidth - treeIndent(depth, contentWidth) - ROW_TEXT_CHROME
  return { date: text >= META_DATE_MIN_TEXT, notes: text >= META_NOTES_MIN_TEXT }
}

function AppLogo() {
  return (
    <svg className="ws-logo" width="16" height="16" viewBox="0 0 32 32" fill="none" aria-hidden="true">
      <rect x="1" y="1" width="30" height="30" rx="7" fill="#1e2330" stroke="#333b4e" strokeWidth="1" />
      <path d="M7 9.5C7 8.95 7.45 8.5 8 8.5H10.5C10.9 8.5 11.26 8.74 11.42 9.11L14 15.2L12 19L8.5 10.8V22.5C8.5 23.05 8.05 23.5 7.5 23.5H7.5C6.95 23.5 6.5 23.05 6.5 22.5V9.5C6.5 8.95 6.95 8.5 7.5 8.5Z" fill="#3b82f6" />
      <path d="M12 9.5C12 8.95 12.45 8.5 13 8.5H14.8C15.25 8.5 15.65 8.8 15.78 9.23L18.8 15.5C18.95 15.82 18.95 16.18 18.8 16.5L15.78 22.77C15.65 23.2 15.25 23.5 14.8 23.5H13C12.45 23.5 12 23.05 12 22.5V21L15.5 16L12 9.5Z" fill="#8b5cf6" />
      <path d="M18.5 9.5C18.5 8.95 18.95 8.5 19.5 8.5H21.3C21.75 8.5 22.15 8.8 22.28 9.23L25.3 15.5C25.45 15.82 25.45 16.18 25.3 16.5L22.28 22.77C22.15 23.2 21.75 23.5 21.3 23.5H19.5C18.95 23.5 18.5 23.05 18.5 22.5V21L22 16L18.5 9.5Z" fill="#06b6d4" />
    </svg>
  )
}

function formatModified(ms?: number): string {
  if (!ms) return ''
  const then = new Date(ms)
  if (Number.isNaN(then.getTime())) return ''
  const now = new Date()
  const sameDay = then.toDateString() === now.toDateString()
  if (sameDay) {
    return then.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })
  }
  return then.toLocaleDateString([], { month: 'short', day: 'numeric' })
}

function highlight(text: string, query: string): React.ReactNode {
  if (!query) return text
  const lower = text.toLowerCase()
  const needle = query.toLowerCase()
  const at = lower.indexOf(needle)
  if (at < 0) return text
  return (
    <>
      {text.slice(0, at)}
      <mark className="hit">{text.slice(at, at + query.length)}</mark>
      {text.slice(at + query.length)}
    </>
  )
}

function relativeFolder(rel: string): string {
  const slash = rel.replace(/\\/g, '/')
  const i = slash.lastIndexOf('/')
  return i <= 0 ? slash : slash.slice(i + 1)
}

function treeRel(root: string, path: string): string {
  const a = root.replace(/\\/g, '/').replace(/\/$/, '')
  const b = path.replace(/\\/g, '/')
  if (!a) return b
  if (b === a) return ''
  return b.startsWith(`${a}/`) ? b.slice(a.length + 1) : b
}

function WorkspaceSwitcher({
  workspace,
  snapshot,
  selectedId,
  name,
}: {
  workspace?: WorkspaceAdapter
  snapshot?: WorkspaceSnapshot | null
  selectedId: string
  name: string
}) {
  const roots = snapshot?.workspaces ?? []
  if (!workspace) {
    return <div className="ws-name"><AppLogo /><span>{name}</span></div>
  }
  return (
    <DropdownMenu.Root modal={false}>
      <DropdownMenu.Trigger asChild>
        <button
          type="button"
          className="ws-name ws-switcher"
          data-testid="workspace-switcher"
          aria-label={`Workspace: ${name}`}
        >
          <AppLogo />
          <span className="nm">{name}</span>
          <i className="ph ph-caret-down ws-caret" aria-hidden="true" />
        </button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          className="doc-tab-menu workspace-menu"
          data-testid="workspace-menu"
          aria-label="Workspaces"
          side="bottom"
          align="start"
          sideOffset={6}
          collisionPadding={8}
        >
          <div className="doc-tab-menu-head">
            <span>Workspaces</span>
            <span>{roots.length}</span>
          </div>
          <DropdownMenu.RadioGroup
            className="doc-tab-menu-list"
            value={selectedId}
            onValueChange={(id) => {
              if (id !== selectedId) void workspace.activateWorkspace(id)
            }}
          >
            {roots.map((root) => (
              <DropdownMenu.RadioItem
                key={root.id}
                value={root.id}
                className={`doc-tab-menu-item workspace-menu-item${root.id === selectedId ? ' sel' : ''}`}
                data-testid={`workspace-menu-item-${root.id}`}
              >
                <span className="doc-tab-menu-icon" aria-hidden="true">
                  <i className={`ph ${root.kind === 'folder' ? 'ph-folder' : 'ph-file-text'}`} />
                </span>
                <AppTooltip label={root.root || root.name}>
                  <span className="doc-tab-menu-copy">
                    <span className="nm">{root.name}</span>
                    <span className="doc-tab-menu-path">{root.root || 'Untitled'}</span>
                  </span>
                </AppTooltip>
                <DropdownMenu.ItemIndicator>
                  <i className="ph ph-check" aria-label="Current workspace" />
                </DropdownMenu.ItemIndicator>
              </DropdownMenu.RadioItem>
            ))}
          </DropdownMenu.RadioGroup>
          <DropdownMenu.Separator className="workspace-menu-separator" />
          <DropdownMenu.Item
            className="doc-tab-menu-item"
            data-testid="workspace-open-folder"
            onSelect={() => {
              void workspace.pickFolder()
            }}
          >
            <span className="doc-tab-menu-icon" aria-hidden="true"><i className="ph ph-folder-plus" /></span>
            Open Folder…
          </DropdownMenu.Item>
          {snapshot?.kind === 'folder' ? (
            <DropdownMenu.Item
              className="doc-tab-menu-item"
              data-testid="workspace-close"
              onSelect={() => {
                void workspace.closeWorkspaceRoot(selectedId)
              }}
            >
              <span className="doc-tab-menu-icon" aria-hidden="true"><i className="ph ph-x" /></span>
              Close Workspace
            </DropdownMenu.Item>
          ) : null}
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  )
}

export function SidePane({
  views,
  doc,
  workspace,
  workspaceSnap,
  onJumpHeading,
}: {
  views: ViewRegistry
  doc?: DocumentInfo | null
  workspace?: WorkspaceAdapter
  workspaceSnap?: WorkspaceSnapshot | null
  onJumpHeading?: (documentId: string | undefined, path: string, offset: number) => void
}): React.ReactElement | null {
  const entries = useViews(views, 'left')
  const [filters, setFilters] = useState<Record<string, string>>({})
  const [nav, setNav] = useState<NavSnapshot | null>(null)
  const [search, setSearch] = useState<NavSearchResult | null>(null)
  const [width, setWidth] = useState(SIDEBAR_DEFAULT)
  const [winWidth, setWinWidth] = useState(() =>
    typeof window === 'undefined' ? SIDEBAR_DEFAULT : document.documentElement.clientWidth || window.innerWidth,
  )

  useEffect(() => {
    const onResize = () => setWinWidth(document.documentElement.clientWidth || window.innerWidth)
    window.addEventListener('resize', onResize)
    const observer = new ResizeObserver(onResize)
    observer.observe(document.documentElement)
    return () => {
      window.removeEventListener('resize', onResize)
      observer.disconnect()
    }
  }, [])

  const paneWidth = Math.min(width, Math.max(SIDEBAR_FLOOR, winWidth - CENTER_MIN))

  const selectedWorkspaceId = workspaceSnap?.workspaces.find(
    (item) => item.root === workspaceSnap.root && item.kind === workspaceSnap.kind,
  )?.id ?? ''
  const filter = filters[selectedWorkspaceId] ?? ''

  useEffect(() => {
    if (!workspace) return
    let disposed = false
    setNav(null)
    setSearch(null)
    void workspace.navSnapshot().then((snap) => {
      if (!disposed) setNav(snap)
    })
    const sub = workspace.subscribeNav((snap) => setNav(snap))
    const searchSub = workspace.subscribeSearch((result) => setSearch(result))
    return () => {
      disposed = true
      sub.dispose()
      searchSub.dispose()
    }
  }, [workspace, selectedWorkspaceId])

  useEffect(() => {
    if (!workspace) return
    const handle = window.setTimeout(() => {
      void workspace.setFilter(filter).then(setSearch)
    }, 80)
    return () => window.clearTimeout(handle)
  }, [filter, workspace, selectedWorkspaceId])

  if (!workspace && doc === undefined && entries.length === 0) return null

  const root = workspaceSnap?.root ?? nav?.root ?? ''
  const name = workspaceSnap?.name ?? (root ? relativeFolder(root) : 'Workspace')
  const kind = workspaceSnap?.kind ?? (root ? 'folder' : 'untitled')
  const activePath = workspaceSnap?.documents.find((d) => d.id === workspaceSnap.activeId)?.path ?? doc?.path ?? ''
  const total = nav?.total ?? 0
  const query = filter.trim()
  const filtering = query !== ''

  const onOpenFile = (path: string) => {
    if (!workspace) return
    void workspace.open(path)
  }

  const onToggle = (entry: NavEntry) => {
    if (!workspace) return
    void workspace.toggleFolder(treeRel(root, entry.path) || entry.path)
  }

  const tree = nav?.entries ?? []
  const fileCountLabel =
    nav?.status === 'scanning' || nav?.status === 'indexing'
      ? `Indexing ${nav.indexed} of ${nav.total} files`
      : nav?.status === 'error'
        ? nav.error || 'Could not index this folder'
        : `${total} ${total === 1 ? 'file' : 'files'}`

  return (
    <>
      <aside
        className="side-pane left-pane side"
        data-testid="left-pane"
        aria-label="Workspace"
        style={{ width: paneWidth, maxWidth: paneWidth, minWidth: paneWidth }}
      >
        <div className="ws">
          <WorkspaceSwitcher
            workspace={workspace}
            snapshot={workspaceSnap}
            selectedId={selectedWorkspaceId}
            name={kind === 'untitled' ? 'Untitled' : name}
          />
          <div className="ws-meta meta">
            <span className="p">{kind === 'folder' ? root || name : 'Untitled window'}</span>
            <span>·</span>
            <span data-testid="workspace-file-count">{fileCountLabel}</span>
          </div>
        </div>
        {kind === 'folder' ? (
          <label className={`filter${filter ? ' has' : ''}`}>
            <input
              type="text"
              placeholder="Filter files and headings"
              aria-label="Filter files and headings"
              autoComplete="off"
              value={filter}
              onChange={(event) => {
                const value = event.target.value
                setFilters((current) => ({ ...current, [selectedWorkspaceId]: value }))
              }}
            />
            {filter ? (
              <button
                type="button"
                className="clear"
                aria-label="Clear filter"
                data-testid="clear-filter"
                onClick={() => setFilters((current) => ({ ...current, [selectedWorkspaceId]: '' }))}
              >
                ×
              </button>
            ) : null}
          </label>
        ) : null}
        {kind === 'folder' && filtering ? (
          <FilterResults
            query={query}
            search={search}
            nav={nav}
            paneWidth={paneWidth}
            onOpenFile={onOpenFile}
            onJumpHeading={onJumpHeading}
          />
        ) : kind === 'folder' ? (
          <FileTree
            entries={tree}
            activePath={activePath}
            collapsedCount={nav?.collapsedCount ?? 0}
            status={nav?.status ?? 'idle'}
            paneWidth={paneWidth}
            onOpenFile={onOpenFile}
            onToggle={onToggle}
          />
        ) : (
          <div className="list" role="tree" aria-label="Files">
            {doc ? (
              <FileRow entry={{
                id: `open:${doc.id ?? 'current'}`,
                kind: 'file',
                name: doc.name,
                path: doc.path,
                parent: '',
                depth: 0,
                edited: doc.edited,
                conflict: doc.conflict,
                deleted: doc.deleted,
              }} selected query="" paneWidth={paneWidth} onOpen={() => {}} />
            ) : (
              <p className="nav-empty" data-testid="untitled-empty">Write a new document, or open a folder.</p>
            )}
          </div>
        )}
        {entries.map((view) => (
          <section key={view.id} className="side-view" data-view-id={view.id} aria-label={view.title}>
            <header className="side-view-header">{view.title}</header>
            <ViewHost view={view} className="side-view-body" />
          </section>
        ))}
      </aside>
      <div
        className="sidebar-splitter"
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize workspace sidebar"
        aria-valuemin={SIDEBAR_FLOOR}
        aria-valuemax={SIDEBAR_MAX}
        aria-valuenow={paneWidth}
        tabIndex={0}
        data-testid="sidebar-splitter"
        onPointerDown={(event) => {
          event.preventDefault()
          const startX = event.clientX
          const startW = width
          const move = (ev: PointerEvent) => {
            const next = Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, startW + (ev.clientX - startX)))
            setWidth(next)
          }
          const up = () => {
            window.removeEventListener('pointermove', move)
            window.removeEventListener('pointerup', up)
          }
          window.addEventListener('pointermove', move)
          window.addEventListener('pointerup', up)
        }}
        onKeyDown={(event) => {
          if (event.key === 'ArrowLeft') {
            event.preventDefault()
            setWidth((w) => Math.max(SIDEBAR_MIN, w - 16))
          } else if (event.key === 'ArrowRight') {
            event.preventDefault()
            setWidth((w) => Math.min(SIDEBAR_MAX, w + 16))
          } else if (event.key === 'Home') {
            event.preventDefault()
            setWidth(SIDEBAR_MIN)
          } else if (event.key === 'End') {
            event.preventDefault()
            setWidth(SIDEBAR_MAX)
          }
        }}
      />
    </>
  )
}

function FileTree({
  entries,
  activePath,
  collapsedCount,
  status,
  paneWidth,
  onOpenFile,
  onToggle,
}: {
  entries: NavEntry[]
  activePath: string
  collapsedCount: number
  status: string
  paneWidth: number
  onOpenFile: (path: string) => void
  onToggle: (entry: NavEntry) => void
}) {
  if (status === 'error') {
    return (
      <ScrollArea className="list" contentClassName="list-content" role="tree" aria-label="Files" data-testid="workspace-tree">
        <p className="nav-empty" role="alert" data-testid="nav-error">Could not read this folder.</p>
      </ScrollArea>
    )
  }
  if (entries.length === 0 && (status === 'scanning' || status === 'indexing')) {
    return (
      <ScrollArea className="list" contentClassName="list-content" role="tree" aria-label="Files" data-testid="workspace-tree">
        <p className="nav-empty" data-testid="nav-loading" role="status">Indexing files…</p>
      </ScrollArea>
    )
  }
  if (entries.length === 0) {
    return (
      <ScrollArea className="list" contentClassName="list-content" role="tree" aria-label="Files" data-testid="workspace-tree">
        <p className="nav-empty" data-testid="nav-empty">No Markdown files in this folder.</p>
      </ScrollArea>
    )
  }
  return (
    <ScrollArea
      className="list"
      contentClassName="list-content"
      role="tree"
      aria-label="Files"
      data-testid="workspace-tree"
      onKeyDown={(event) => {
        const row = (event.target as HTMLElement | null)?.closest<HTMLElement>('[role="treeitem"]')
        if (!row) return
        const rows = Array.from(event.currentTarget.querySelectorAll<HTMLElement>('[role="treeitem"]'))
        const index = rows.indexOf(row)
        const entry = entries.find((item) => item.id === row.dataset.navId)
        if (index < 0 || !entry) return
        const focusRow = (next: number) => rows[next]?.focus()
        switch (event.key) {
          case 'ArrowDown':
            event.preventDefault()
            focusRow(index + 1)
            return
          case 'ArrowUp':
            event.preventDefault()
            focusRow(index - 1)
            return
          case 'Home':
            event.preventDefault()
            focusRow(0)
            return
          case 'End':
            event.preventDefault()
            focusRow(rows.length - 1)
            return
          case 'ArrowRight':
            if (entry.kind !== 'folder') return
            event.preventDefault()
            if (entry.folderState === 'collapsed') onToggle(entry)
            else if (Number(rows[index + 1]?.dataset.depth ?? -1) === entry.depth + 1) focusRow(index + 1)
            return
          case 'ArrowLeft': {
            if (entry.kind === 'folder' && entry.folderState !== 'collapsed') {
              event.preventDefault()
              onToggle(entry)
              return
            }
            event.preventDefault()
            for (let above = index - 1; above >= 0; above -= 1) {
              if (Number(rows[above].dataset.depth ?? -1) < entry.depth) {
                focusRow(above)
                return
              }
            }
            return
          }
          default:
            return
        }
      }}
    >
      <div className="tree-section" aria-hidden="true">
        <span>Workspace</span>
        <span>{entries.filter((entry) => entry.kind === 'file').length}</span>
      </div>
      {collapsedCount > 0 ? (
        <p className="nav-collapsed meta" data-testid="collapsed-count">{collapsedCount} files hidden</p>
      ) : null}
      {entries.map((entry) =>
        entry.kind === 'folder' ? (
          <FolderRow key={entry.id} entry={entry} onToggle={() => onToggle(entry)} />
        ) : (
          <FileRow
            key={entry.id}
            entry={entry}
            selected={entry.path === activePath}
            query=""
            paneWidth={paneWidth}
            onOpen={() => onOpenFile(entry.path)}
          />
        ),
      )}
    </ScrollArea>
  )
}

function FolderRow({ entry, onToggle }: { entry: NavEntry; onToggle: () => void }) {
  const collapsed = entry.folderState === 'collapsed'
  const count = entry.markdownCount ?? 0
  return (
    <button
      type="button"
      className="row folder"
      role="treeitem"
      aria-expanded={!collapsed}
      aria-label={`${entry.name}, ${count} ${count === 1 ? 'file' : 'files'}`}
      data-testid={`folder-${entry.name}`}
      data-nav-id={entry.id}
      data-depth={entry.depth}
      style={indentStyle(entry.depth)}
      onClick={onToggle}
    >
      <span className="ed-slot tree-folder-icon" aria-hidden="true">
        <i className={`ph ${collapsed ? 'ph-folder' : 'ph-folder-open'}`} />
      </span>
      <span className="body">
        <span className="l1">
          <span className="nm">{entry.name}</span>
          <span className="count meta">{count}</span>
        </span>
      </span>
    </button>
  )
}

function FileRow({
  entry,
  selected,
  query,
  onOpen,
  paneWidth,
}: {
  entry: NavEntry
  selected: boolean
  query: string
  onOpen: () => void
  paneWidth?: number
}) {
  const subtitle = query
    ? entry.title && entry.title !== entry.name ? entry.title : entry.excerpt || entry.path || 'Untitled'
    : relativeFolder(entry.parent) || entry.path || 'Untitled'
  const room = metaRoom(entry.depth, paneWidth)
  const fullModified = entry.modifiedAt ? new Date(entry.modifiedAt).toLocaleString() : ''
  const rowTitle =
    !room.date && fullModified ? `${entry.path || entry.name} · ${fullModified}` : entry.path || entry.name
  return (
    <AppTooltip label={<><span className="mgo-tooltip-path">{rowTitle}</span>{subtitle === rowTitle ? null : <span>{subtitle}</span>}</>}>
    <button
      type="button"
      className={`row file${selected ? ' sel' : ''}${entry.deleted ? ' deleted' : ''}${entry.conflict ? ' conflict' : ''}`}
      role="treeitem"
      aria-current={selected ? 'true' : undefined}
      data-testid={selected ? 'workspace-current-file' : `file-${entry.name}`}
      data-nav-id={entry.id}
      data-depth={entry.depth}
      style={indentStyle(entry.depth)}
      onClick={onOpen}
    >
      <span className="ed-slot">
        {entry.edited ? <span className="edited" title="Unsaved edits" /> : null}
      </span>
      <i className="ph ph-file-text tree-file-icon" aria-hidden="true" />
      <span className="body">
        <span className="l1">
          <span className="nm">{highlight(entry.name, query)}</span>
          {entry.conflict ? <span className="badge conflict" title="Conflict">conflict</span> : null}
          {entry.deleted ? <span className="badge deleted" title="Deleted">deleted</span> : null}
        </span>
        <span className="l2">
          <span className="s">{highlight(subtitle, query)}</span>
          {entry.notes && room.notes ? <span className="notes meta" title="Notes">{entry.notes}</span> : null}
          {entry.modifiedAt && room.date ? (
            <span className="mtime meta" title={fullModified}>{formatModified(entry.modifiedAt)}</span>
          ) : null}
        </span>
      </span>
    </button>
    </AppTooltip>
  )
}

function FilterResults({
  query,
  search,
  nav,
  paneWidth,
  onOpenFile,
  onJumpHeading,
}: {
  query: string
  search: NavSearchResult | null
  nav: NavSnapshot | null
  paneWidth: number
  onOpenFile: (path: string) => void
  onJumpHeading?: (documentId: string | undefined, path: string, offset: number) => void
}) {
  const incomplete = search?.incomplete || nav?.status === 'scanning' || nav?.status === 'indexing'
  const matched = search?.matched ?? 0
  const total = search?.total ?? nav?.total ?? 0
  return (
    <div className="list" role="tree" aria-label="Filter results" data-testid="filter-results">
      <p className="nav-status meta" data-testid="filter-status" role="status">
        {incomplete
          ? `Indexing ${search?.indexed ?? nav?.indexed ?? 0} of ${total} files — ${matched} so far`
          : `${matched} of ${total} files`}
      </p>
      {incomplete && matched === 0 ? (
        <p className="nav-empty" data-testid="filter-indexing">Still indexing — not an empty result.</p>
      ) : null}
      {!incomplete && matched === 0 ? (
        <p className="nav-empty" data-testid="filter-empty">No matching files or headings.</p>
      ) : null}
      {search?.hits.map((hit) => (
        <div key={hit.path} className="hit-group">
          <FileRow
            entry={{
              id: `hit:${hit.path}`,
              kind: 'file',
              name: hit.name,
              path: hit.path,
              parent: '',
              depth: 0,
              title: hit.title,
              excerpt: hit.excerpt,
              documentId: hit.documentId,
            }}
            selected={false}
            query={query}
            paneWidth={paneWidth}
            onOpen={() => onOpenFile(hit.path)}
          />
          {hit.headings.map((heading) => (
            <button
              key={`${hit.path}:${heading.offset}`}
              type="button"
              className="row heading-hit"
              data-testid={`heading-hit-${heading.offset}`}
              style={{ paddingLeft: 32 }}
              onClick={() => onJumpHeading?.(hit.documentId, hit.path, heading.offset)}
            >
              <span className="ed-slot" />
              <span className="body">
                <span className="l1">
                  <span className="nm">{highlight(heading.text, query)}</span>
                </span>
              </span>
            </button>
          ))}
        </div>
      ))}
    </div>
  )
}

export function PathCrumbs({
  root,
  path,
  onOpen,
}: {
  root: string
  path: string
  onOpen: (path: string, isFile: boolean) => void
}): React.ReactElement | null {
  const crumbs = useMemo(() => breadcrumbs(root, path), [root, path])
  if (!path || crumbs.length === 0) return null
  return (
    <nav className="crumbs" aria-label="Path" data-testid="path-crumbs">
      {crumbs.map((crumb, index) => (
        <span key={crumb.path} className="crumb">
          {index > 0 ? <span className="sep" aria-hidden="true">/</span> : null}
          <button
            type="button"
            className={`crumb-btn${crumb.isFile ? ' last' : ''}`}
            onClick={() => onOpen(crumb.path, crumb.isFile)}
          >
            {crumb.name}
          </button>
        </span>
      ))}
    </nav>
  )
}
