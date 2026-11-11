/**
 * Prototype question: how should the workspace chrome around the reading
 * paper feel — specifically document tabs, the document overflow menu, the
 * file tree, and the Model popover — without losing Portrait's quiet paper
 * language?
 *
 * Three structurally different answers:
 *   A · Paper   — the current three-column chrome, refined.
 *   B · Command — a wider command-palette navigation surface.
 *   C · Inline  — text tabs with an inline, list-first document menu.
 *
 * Throwaway: reads the workspace once, mixes local selection/query/model
 * state only, and never writes to the host. Switch with the bottom pill or
 * the left/right arrow keys (outside editable fields and open menus).
 */
import { useEffect, useRef, useState } from 'react'
import type { ReactElement } from 'react'
import { DropdownMenu } from 'radix-ui'
import type {
  DocumentInfo,
  HostAdapter,
  NavEntry,
  NavSnapshot,
  WorkspaceAdapter,
  WorkspaceSnapshot,
} from '../host/types'
import './workspace-chrome.css'

type VariantKey = 'A' | 'B' | 'C'

const VARIANTS = [
  { key: 'A', name: 'Paper' },
  { key: 'B', name: 'Command' },
  { key: 'C', name: 'Inline' },
] as const

/** The shipped agent defaults; a readable settings file replaces these. */
const FALLBACK_MODELS = ['gemini-3.8-flash', 'gemini-4-argon', 'claude-opus-5-5']

const EMPTY_COLLAPSED = new Set<string>()

interface VariantProps {
  snapshot: WorkspaceSnapshot | null
  nav: NavSnapshot | null
  docs: DocumentInfo[]
  activeDoc: DocumentInfo | null
  activeId: string
  onActivate: (id: string) => void
  model: string
  models: string[]
  onModel: (model: string) => void
  loading: boolean
  loadError: string
}

export interface WorkspaceChromePrototypeProps {
  workspace: WorkspaceAdapter
  initialVariant: string
}

export function WorkspaceChromePrototype({
  workspace,
  initialVariant,
}: WorkspaceChromePrototypeProps): ReactElement {
  const [variant, setVariant] = useState<VariantKey>(() => normalizeVariant(initialVariant))
  const [snapshot, setSnapshot] = useState<WorkspaceSnapshot | null>(null)
  const [nav, setNav] = useState<NavSnapshot | null>(null)
  const [activeId, setActiveId] = useState('')
  const [model, setModel] = useState('')
  const [models, setModels] = useState<string[]>(FALLBACK_MODELS)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')

  useEffect(() => {
    let disposed = false
    setLoading(true)
    void (async () => {
      const [workspaceResult, navResult] = await Promise.allSettled([
        workspace.snapshot(),
        workspace.navSnapshot(),
      ] as const)
      if (disposed) return
      if (workspaceResult.status === 'fulfilled') {
        setSnapshot(workspaceResult.value)
        setActiveId((current) =>
          current ||
          workspaceResult.value.activeId ||
          workspaceResult.value.documents[0]?.id ||
          '',
        )
      } else {
        setLoadError(messageOf(workspaceResult.reason))
      }
      if (navResult.status === 'fulfilled') setNav(navResult.value)
      else if (workspaceResult.status === 'fulfilled') setLoadError(messageOf(navResult.reason))
      setLoading(false)
    })()
    return () => {
      disposed = true
    }
  }, [workspace])

  const docs = snapshot?.documents ?? []
  const activeDoc =
    docs.find((doc) => doc.id === activeId) ??
    docs.find((doc) => doc.id === snapshot?.activeId) ??
    docs[0] ??
    null
  const effectiveId = activeDoc?.id ?? ''

  useEffect(() => {
    if (!effectiveId) return
    let disposed = false
    let host: HostAdapter
    try {
      host = workspace.document(effectiveId)
    } catch {
      return
    }
    void host
      .getSettings()
      .then((settings) => {
        if (disposed) return
        if (settings.models?.length) setModels(settings.models)
        if (settings.model) setModel((current) => current || settings.model)
      })
      .catch(() => {
        // Settings are optional in the prototype; the fallback list stays.
      })
    return () => {
      disposed = true
    }
  }, [workspace, effectiveId])

  useEffect(() => {
    if (typeof window === 'undefined') return
    const url = new URL(window.location.href)
    if (url.searchParams.get('variant') === variant) return
    url.searchParams.set('variant', variant)
    window.history.replaceState(null, '', url.toString())
  }, [variant])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
      if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey) return
      const target = event.target
      if (target instanceof Element) {
        if (target.closest('input, textarea, select, [contenteditable="true"]')) return
        if (target.closest('[role="menu"], [role="listbox"], [data-radix-popper-content-wrapper]')) {
          return
        }
      }
      event.preventDefault()
      setVariant((current) => stepVariant(current, event.key === 'ArrowRight' ? 1 : -1))
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  const data: VariantProps = {
    snapshot,
    nav,
    docs,
    activeDoc,
    activeId: effectiveId,
    onActivate: setActiveId,
    model,
    models,
    onModel: setModel,
    loading,
    loadError,
  }

  return (
    <div className={`wcp wcp-${variant.toLowerCase()}`}>
      <div className="wcp-shell">
        {variant === 'A' ? <PaperNavigation {...data} /> : null}
        {variant === 'B' ? <CommandNavigation {...data} /> : null}
        {variant === 'C' ? <InlineNavigation {...data} /> : null}
      </div>
      <VariantSwitcher variant={variant} onChange={setVariant} />
    </div>
  )
}

export function PaperNavigation(props: VariantProps): ReactElement {
  const [query, setQuery] = useState('')
  const tree = useTreeState(props.nav, query)
  const visibleDocs = props.docs.slice(0, 3)

  return (
    <>
      <aside className="wcp-side wcp-side-a" aria-label="Workspace">
        <SideHeader {...props} />
        <SideFilter value={query} onChange={setQuery} />
        <div className="wcp-tree wcp-tree-a" role="tree" aria-label="Workspace files">
          <TreeEmpty {...props} query={query} rows={tree.rows} />
          {tree.rows.map((entry) =>
            entry.kind === 'folder' ? (
              <button
                key={entry.id}
                type="button"
                role="treeitem"
                aria-expanded={!tree.collapsed.has(entry.path)}
                className="wcp-nav-row wcp-nav-folder wcp-nav-row-a"
                style={{ paddingLeft: 6 + entry.depth * 14 }}
                title={entry.path}
                onClick={() => tree.toggle(entry.path)}
              >
                <i
                  className={`ph ${tree.collapsed.has(entry.path) ? 'ph-caret-right' : 'ph-caret-down'} wcp-caret`}
                  aria-hidden="true"
                />
                <span className="wcp-nav-name">{entry.name}</span>
                <span className="wcp-nav-meta">{entry.markdownCount ?? 0}</span>
              </button>
            ) : (
              <button
                key={entry.id}
                type="button"
                role="treeitem"
                aria-current={isActiveEntry(entry, props) ? 'true' : undefined}
                className={`wcp-nav-row wcp-nav-file wcp-nav-row-a${isActiveEntry(entry, props) ? ' is-active' : ''}`}
                style={{ paddingLeft: 6 + entry.depth * 14 }}
                title={entry.path || entry.name}
                onClick={() => openEntry(entry, props.onActivate)}
              >
                <span className="wcp-file-slot" aria-hidden="true">
                  {entry.edited ? <span className="wcp-mini-dot" /> : null}
                </span>
                <span className="wcp-nav-name">{entry.name}</span>
                {entry.notes ? (
                  <span className="wcp-nav-meta">{entry.notes}</span>
                ) : entry.modifiedAt ? (
                  <span className="wcp-nav-meta">{formatDate(entry.modifiedAt)}</span>
                ) : null}
              </button>
            ),
          )}
        </div>
        <SideFooter {...props} />
      </aside>

      <main className="wcp-main">
        <div className="wcp-tabs-row wcp-tabs-row-a">
          <div className="wcp-tabs wcp-tabs-a" role="tablist" aria-label="Documents">
            {visibleDocs.map((doc) => (
              <button
                key={doc.id}
                type="button"
                role="tab"
                aria-selected={doc.id === props.activeId}
                className="wcp-tab wcp-tab-a"
                title={doc.path || doc.name}
                onClick={() => props.onActivate(doc.id)}
              >
                <span className="wcp-tab-name">{tabLabel(props.docs, doc)}</span>
                <DocStatus doc={doc} />
              </button>
            ))}
          </div>
          <DocumentMenu
            variant="A"
            docs={props.docs}
            activeId={props.activeId}
            hiddenCount={Math.max(props.docs.length - visibleDocs.length, 0)}
            onActivate={props.onActivate}
          />
        </div>
        <div className="wcp-main-scroll">
          <ReadingPaper doc={props.activeDoc} snapshot={props.snapshot} />
        </div>
      </main>

      <RightRail {...props} variant="A" />
    </>
  )
}

export function CommandNavigation(props: VariantProps): ReactElement {
  const [query, setQuery] = useState('')
  const tree = useTreeState(props.nav, query)
  const visibleDocs = props.docs.slice(0, 4)
  const folders = tree.rows.filter((entry) => entry.kind === 'folder')
  const files = tree.rows.filter((entry) => entry.kind === 'file')

  return (
    <>
      <aside className="wcp-side wcp-side-b" aria-label="Workspace">
        <SideHeader {...props} />
        <SideFilter value={query} onChange={setQuery} />
        <div className="wcp-tree wcp-tree-b" role="tree" aria-label="Workspace files">
          <TreeEmpty {...props} query={query} rows={tree.rows} />
          {folders.length > 0 ? (
            <div role="group" aria-label="Folders">
              <div className="wcp-section-label">
                <span>Folders</span>
                <span>{folders.length}</span>
              </div>
              {folders.map((entry) => {
                const open = !tree.collapsed.has(entry.path)
                return (
                  <button
                    key={entry.id}
                    type="button"
                    role="treeitem"
                    aria-expanded={open}
                    className="wcp-nav-row wcp-nav-folder wcp-nav-row-b"
                    style={{ paddingLeft: 6 + entry.depth * 14 }}
                    title={entry.path}
                    onClick={() => tree.toggle(entry.path)}
                  >
                    <i
                      className={`ph ${open ? 'ph-folder-open' : 'ph-folder'} wcp-folder-icon`}
                      aria-hidden="true"
                    />
                    <span className="wcp-nav-name">{entry.name}</span>
                    <span className="wcp-nav-meta">{entry.markdownCount ?? 0}</span>
                  </button>
                )
              })}
            </div>
          ) : null}
          {files.length > 0 ? (
            <div role="group" aria-label="Documents">
              <div className="wcp-section-label">
                <span>Documents</span>
                <span>{files.length}</span>
              </div>
              {files.map((entry) => (
                <button
                  key={entry.id}
                  type="button"
                  role="treeitem"
                  aria-current={isActiveEntry(entry, props) ? 'true' : undefined}
                  className={`wcp-nav-row wcp-nav-file wcp-nav-row-b${isActiveEntry(entry, props) ? ' is-active' : ''}`}
                  style={{ paddingLeft: 6 + entry.depth * 14 }}
                  title={entry.path || entry.name}
                  onClick={() => openEntry(entry, props.onActivate)}
                >
                  <i className="ph ph-file-text wcp-file-icon" aria-hidden="true" />
                  <span className="wcp-nav-copy">
                    <span className="wcp-nav-name">{entry.name}</span>
                    <span className="wcp-nav-sub">{parentFolder(entry.path)}</span>
                  </span>
                  <DocEntryStatus entry={entry} />
                </button>
              ))}
            </div>
          ) : null}
        </div>
        <SideFooter {...props} />
      </aside>

      <main className="wcp-main">
        <div className="wcp-tabs-row wcp-tabs-row-b">
          <div className="wcp-tabs wcp-tabs-b" role="tablist" aria-label="Documents">
            {visibleDocs.map((doc) => (
              <button
                key={doc.id}
                type="button"
                role="tab"
                aria-selected={doc.id === props.activeId}
                className="wcp-tab wcp-tab-b"
                title={doc.path || doc.name}
                onClick={() => props.onActivate(doc.id)}
              >
                <i className="ph ph-file-text wcp-tab-icon" aria-hidden="true" />
                <span className="wcp-tab-name">{tabLabel(props.docs, doc)}</span>
                <DocStatus doc={doc} />
              </button>
            ))}
          </div>
          <DocumentMenu
            variant="B"
            docs={props.docs}
            activeId={props.activeId}
            hiddenCount={Math.max(props.docs.length - visibleDocs.length, 0)}
            onActivate={props.onActivate}
          />
        </div>
        <div className="wcp-main-scroll">
          <ReadingPaper doc={props.activeDoc} snapshot={props.snapshot} />
        </div>
      </main>

      <RightRail {...props} variant="B" />
    </>
  )
}

export function InlineNavigation(props: VariantProps): ReactElement {
  const [query, setQuery] = useState('')
  const tree = useTreeState(props.nav, query)
  const visibleDocs = props.docs.slice(0, 4)
  const hiddenCount = Math.max(props.docs.length - visibleDocs.length, 0)

  return (
    <>
      <aside className="wcp-side wcp-side-c" aria-label="Workspace">
        <SideHeader {...props} />
        <SideFilter value={query} onChange={setQuery} />
        <div className="wcp-tree wcp-tree-c" role="tree" aria-label="Workspace files">
          <TreeEmpty {...props} query={query} rows={tree.rows} />
          {tree.rows.map((entry) => {
            const open = !tree.collapsed.has(entry.path)
            return entry.kind === 'folder' ? (
              <button
                key={entry.id}
                type="button"
                role="treeitem"
                aria-expanded={open}
                className="wcp-nav-row wcp-nav-folder wcp-nav-row-c"
                style={{ paddingLeft: 4 + entry.depth * 12 }}
                title={entry.path}
                onClick={() => tree.toggle(entry.path)}
              >
                <i
                  className={`ph ph-caret-right wcp-caret wcp-caret-c${open ? ' is-open' : ''}`}
                  aria-hidden="true"
                />
                <span className="wcp-nav-name">{entry.name}</span>
                <span className="wcp-nav-meta">{entry.markdownCount ?? 0}</span>
              </button>
            ) : (
              <button
                key={entry.id}
                type="button"
                role="treeitem"
                aria-current={isActiveEntry(entry, props) ? 'true' : undefined}
                className={`wcp-nav-row wcp-nav-file wcp-nav-row-c${isActiveEntry(entry, props) ? ' is-active' : ''}`}
                style={{ paddingLeft: 18 + entry.depth * 12 }}
                title={entry.path || entry.name}
                onClick={() => openEntry(entry, props.onActivate)}
              >
                <span className="wcp-nav-name">{entry.name}</span>
                <span className="wcp-nav-meta">{entry.notes ? `${entry.notes}` : parentFolder(entry.path)}</span>
              </button>
            )
          })}
        </div>
        <SideFooter {...props} />
      </aside>

      <main className="wcp-main">
        <div className="wcp-tabs-row wcp-tabs-row-c">
          <div className="wcp-tabs wcp-tabs-c" role="tablist" aria-label="Documents">
            {visibleDocs.map((doc) => (
              <button
                key={doc.id}
                type="button"
                role="tab"
                aria-selected={doc.id === props.activeId}
                className="wcp-tab wcp-tab-c"
                title={doc.path || doc.name}
                onClick={() => props.onActivate(doc.id)}
              >
                <span className="wcp-tab-name">{tabLabel(props.docs, doc)}</span>
                <DocStatus doc={doc} />
              </button>
            ))}
          </div>
          <DocumentMenu
            variant="C"
            docs={props.docs}
            activeId={props.activeId}
            hiddenCount={hiddenCount}
            onActivate={props.onActivate}
          />
        </div>
        <div className="wcp-main-scroll">
          <ReadingPaper doc={props.activeDoc} snapshot={props.snapshot} />
        </div>
      </main>

      <RightRail {...props} variant="C" />
    </>
  )
}

function SideHeader({ snapshot, nav, loading }: VariantProps): ReactElement {
  const name = workspaceLabel(snapshot)
  const root = snapshot?.root?.trim() || 'Untitled window'
  const count =
    nav?.status === 'scanning' || nav?.status === 'indexing'
      ? `Indexing ${nav.indexed} of ${nav.total}`
      : `${nav?.total ?? 0} ${nav?.total === 1 ? 'file' : 'files'}`
  return (
    <div className="wcp-side-head">
      <div className="wcp-workspace-name">
        <span className="wcp-workspace-mark" aria-hidden="true">
          <i className="ph ph-file-text" />
        </span>
        <span className="wcp-workspace-label">{name}</span>
      </div>
      <div className="wcp-workspace-meta" title={root}>
        <span className="wcp-workspace-path">{root}</span>
        <span className="wcp-meta-sep">·</span>
        <span>{loading && !nav ? 'reading' : count}</span>
      </div>
    </div>
  )
}

function SideFilter({
  value,
  onChange,
}: {
  value: string
  onChange: (value: string) => void
}): ReactElement {
  return (
    <label className="wcp-filter">
      <i className="ph ph-magnifying-glass" aria-hidden="true" />
      <input
        type="search"
        value={value}
        placeholder="Filter files"
        aria-label="Filter files"
        autoComplete="off"
        onChange={(event) => onChange(event.target.value)}
      />
      {value ? (
        <button
          type="button"
          className="wcp-filter-clear"
          aria-label="Clear filter"
          onClick={() => onChange('')}
        >
          <i className="ph ph-x" aria-hidden="true" />
        </button>
      ) : null}
    </label>
  )
}

function SideFooter({ nav, loading, loadError }: VariantProps): ReactElement {
  return (
    <div className="wcp-side-foot">
      <span>{loadError || (loading && !nav ? 'Reading workspace…' : `${nav?.total ?? 0} files`)}</span>
      {nav && nav.collapsedCount > 0 ? <span>{nav.collapsedCount} hidden</span> : null}
    </div>
  )
}

function TreeEmpty({
  nav,
  loading,
  loadError,
  query,
  rows,
}: VariantProps & { query: string; rows: NavEntry[] }): ReactElement | null {
  if (loading && !nav) return <p className="wcp-tree-note">Reading workspace…</p>
  if (loadError && !nav) return <p className="wcp-tree-note">{loadError}</p>
  if (nav?.error) return <p className="wcp-tree-note">{nav.error}</p>
  if (rows.length > 0) return null
  if (query.trim()) {
    return (
      <div className="wcp-tree-empty">
        <strong>No files match “{query.trim()}”</strong>
        <span>Try a shorter name, or clear the filter.</span>
      </div>
    )
  }
  return (
    <div className="wcp-tree-empty">
      <i className="ph ph-folder-open" aria-hidden="true" />
      <strong>No Markdown files</strong>
      <span>This workspace is ready for its first document.</span>
    </div>
  )
}

function ReadingPaper({
  doc,
  snapshot,
}: {
  doc: DocumentInfo | null
  snapshot: WorkspaceSnapshot | null
}): ReactElement {
  if (!doc) {
    return (
      <article className="wcp-paper wcp-paper-empty">
        <span className="wcp-empty-mark" aria-hidden="true">
          <i className="ph ph-file-text" />
        </span>
        <h1>No document open</h1>
        <p>Open a document from the workspace to read it here.</p>
      </article>
    )
  }
  return (
    <article className="wcp-paper" aria-label={`${doc.name} reading view`}>
      <div className="wcp-paper-meta">
        <span className="wcp-paper-crumb">{workspaceLabel(snapshot)}</span>
        <span className="wcp-meta-sep">/</span>
        <span className="wcp-paper-path" title={doc.path || doc.name}>
          {doc.path || doc.name}
        </span>
        {doc.edited ? <span className="wcp-paper-state">Unsaved</span> : null}
        {doc.readOnly ? <span className="wcp-paper-state">Read-only</span> : null}
      </div>
      <h1>{displayTitle(doc.name)}</h1>
      <p className="wcp-lede">
        A working note on how the reading surface holds its calm while the chrome around it does
        the talking.
      </p>
      <div className="wcp-paper-rule" />
      <h2>Reading surface</h2>
      <p>
        The page keeps its original measure: one column, generous leading, and no controls
        competing with the text. Navigation stays at the edges, and the model choice rests at the
        foot of the right rail.
      </p>
      <blockquote>The interface should feel like a desk, not a dashboard.</blockquote>
      <p>
        A document can show its state without turning the tab into a control panel. A small dot
        marks unsaved work; conflict and deletion keep their own words in the menu where there is
        room to read them.
      </p>
      <h2>Navigation</h2>
      <p>
        Open documents stay close, but they do not become buttons. Folders keep their hierarchy.
        Menus appear only when called, on paper that belongs to the same desk.
      </p>
      <h2>Open questions</h2>
      <p>
        Where should a document’s state live when the tab is quiet? How much of the path belongs
        in the menu, and how much in the tooltip? Which affordance earns the accent?
      </p>
      <div className="wcp-paper-end" aria-hidden="true" />
    </article>
  )
}

function RightRail({
  variant,
  activeDoc,
  model,
  models,
  onModel,
}: VariantProps & { variant: VariantKey }): ReactElement {
  const [panel, setPanel] = useState<'outline' | 'genui'>('outline')
  return (
    <aside className={`wcp-rail wcp-rail-${variant.toLowerCase()}`} aria-label="Document panels">
      <div className="wcp-rail-top">
        <div className="wcp-seg" role="tablist" aria-label="Document panel">
          <button
            type="button"
            role="tab"
            aria-selected={panel === 'outline'}
            onClick={() => setPanel('outline')}
          >
            Outline
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={panel === 'genui'}
            onClick={() => setPanel('genui')}
          >
            GenUI
          </button>
        </div>
        {activeDoc ? (
          <div className="wcp-rail-context" title={activeDoc.path || activeDoc.name}>
            {activeDoc.name}
          </div>
        ) : null}
      </div>
      <div className="wcp-rail-body">
        {panel === 'outline' ? <OutlinePanel doc={activeDoc} /> : <GenUIPanel />}
      </div>
      <div className="wcp-rail-foot">
        <div className="wcp-agent-line">
          <span className="wcp-agent-key">Agent</span>
          <span className="wcp-agent-value">Antigravity (agy)</span>
        </div>
        <ModelMenu variant={variant} value={model} models={models} onChange={onModel} />
      </div>
    </aside>
  )
}

function OutlinePanel({ doc }: { doc: DocumentInfo | null }): ReactElement {
  if (!doc) {
    return <p className="wcp-rail-empty">No document to outline.</p>
  }
  const rows = [
    { level: 1, label: displayTitle(doc.name), active: false },
    { level: 2, label: 'Reading surface', active: true },
    { level: 2, label: 'Navigation', active: false },
    { level: 2, label: 'Open questions', active: false },
  ]
  return (
    <div className="wcp-outline">
      <div className="wcp-rail-label">{rows.length} headings</div>
      {rows.map((row) => (
        <div
          key={row.label}
          className={`wcp-outline-row level-${row.level}${row.active ? ' is-active' : ''}`}
          aria-current={row.active ? 'true' : undefined}
          style={{ paddingLeft: row.level === 1 ? 8 : 20 }}
        >
          {row.label}
        </div>
      ))}
    </div>
  )
}

function GenUIPanel(): ReactElement {
  return (
    <div className="wcp-genui">
      <div className="wcp-genui-card">
        <div className="wcp-kicker">KEY POINTS</div>
        <ul>
          <li>Navigation should be quieter than the content.</li>
          <li>Menus carry state; tabs carry names.</li>
          <li>One accent mark is enough.</li>
        </ul>
        <div className="wcp-genui-source">
          <i className="ph ph-file-text" aria-hidden="true" />
          <span>From Reading surface</span>
        </div>
      </div>
      <p className="wcp-rail-empty">Read-only GenUI view</p>
    </div>
  )
}

function ModelMenu({
  variant,
  value,
  models,
  onChange,
}: {
  variant: VariantKey
  value: string
  models: string[]
  onChange: (model: string) => void
}): ReactElement {
  const options = ['', ...Array.from(new Set([value, ...models].map((item) => item.trim()).filter(Boolean)))]
  return (
    <DropdownMenu.Root modal={false}>
      <DropdownMenu.Trigger asChild>
        <button
          type="button"
          className="wcp-model-trigger"
          aria-label={`Model: ${value || 'Agent’s choice'}`}
        >
          <span className="wcp-model-key">Model</span>
          <span className="wcp-model-value">{value || 'Agent’s choice'}</span>
          <i className="ph ph-caret-down wcp-model-caret" aria-hidden="true" />
        </button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          className={`wcp-model-menu wcp-model-menu-${variant.toLowerCase()}`}
          side="top"
          align="end"
          sideOffset={6}
          collisionPadding={12}
          aria-label="Model"
        >
          {variant === 'B' ? <div className="wcp-model-head">MODEL</div> : null}
          {variant === 'C' ? <div className="wcp-model-head">Model</div> : null}
          <DropdownMenu.RadioGroup
            className="wcp-model-list"
            value={value}
            onValueChange={(next) => onChange(next)}
          >
            {options.map((option) => (
              <DropdownMenu.RadioItem
                key={option || '__agent_choice__'}
                value={option}
                className="wcp-model-row"
              >
                <span className="wcp-model-name">{option || 'Agent’s choice'}</span>
                <DropdownMenu.ItemIndicator className="wcp-model-check">
                  <i className="ph ph-check" aria-hidden="true" />
                </DropdownMenu.ItemIndicator>
              </DropdownMenu.RadioItem>
            ))}
          </DropdownMenu.RadioGroup>
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  )
}

function DocumentMenu({
  variant,
  docs,
  activeId,
  hiddenCount,
  onActivate,
}: {
  variant: VariantKey
  docs: DocumentInfo[]
  activeId: string
  hiddenCount: number
  onActivate: (id: string) => void
}): ReactElement {
  const [open, setOpen] = useState(true)
  const [query, setQuery] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const filtered = docs.filter((doc) => matchesDoc(doc, query))
  const empty = docs.length === 0

  useEffect(() => {
    if (!open || empty) return
    const frame = requestAnimationFrame(() => inputRef.current?.focus())
    return () => cancelAnimationFrame(frame)
  }, [open, empty])

  const trigger =
    variant === 'A' ? (
      <button type="button" className="wcp-overflow wcp-overflow-a" aria-label="More documents" title="More documents">
        <i className="ph ph-caret-down" aria-hidden="true" />
      </button>
    ) : variant === 'B' ? (
      <button type="button" className="wcp-overflow wcp-overflow-b" aria-label="More documents" title="More documents">
        <i className="ph ph-dots-three" aria-hidden="true" />
      </button>
    ) : (
      <button
        type="button"
        className="wcp-overflow wcp-overflow-c"
        aria-label={hiddenCount > 0 ? `${hiddenCount} more open documents` : 'Open documents'}
        title={hiddenCount > 0 ? `${hiddenCount} more open documents` : 'Open documents'}
      >
        <span aria-hidden="true">{hiddenCount > 0 ? `+${hiddenCount}` : '+'}</span>
      </button>
    )

  return (
    <DropdownMenu.Root
      modal={false}
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (!next) setQuery('')
      }}
    >
      <DropdownMenu.Trigger asChild>{trigger}</DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          className={`wcp-doc-menu wcp-doc-menu-${variant.toLowerCase()}`}
          side="bottom"
          align="end"
          sideOffset={6}
          collisionPadding={12}
          aria-label="Open documents"
        >
          {variant === 'B' ? (
            <div className="wcp-doc-head">
              <span>OPEN DOCUMENTS</span>
              <span>{docs.length}</span>
            </div>
          ) : null}
          {variant === 'C' ? (
            <div className="wcp-doc-title">
              <span>Open documents</span>
              <span className="wcp-doc-count">{docs.length}</span>
            </div>
          ) : null}
          <div className={`wcp-doc-search wcp-doc-search-${variant.toLowerCase()}`}>
            <i className="ph ph-magnifying-glass" aria-hidden="true" />
            <input
              ref={inputRef}
              type="search"
              value={query}
              placeholder={
                variant === 'A'
                  ? 'Search open documents'
                  : variant === 'B'
                    ? 'Search documents'
                    : 'Filter open documents'
              }
              aria-label="Filter open documents"
              autoComplete="off"
              disabled={empty}
              onChange={(event) => setQuery(event.target.value)}
              onKeyDown={(event) => {
                if (event.key !== 'ArrowDown') return
                const first = event.currentTarget
                  .closest('[role="menu"]')
                  ?.querySelector<HTMLElement>('[role="menuitem"]')
                if (!first) return
                event.preventDefault()
                first.focus()
              }}
            />
          </div>
          <div className="wcp-doc-list" role="group" aria-label="Open documents">
            {filtered.length === 0 ? (
              <div className="wcp-menu-empty" role="status">
                <strong>{empty ? 'No open documents' : 'No matching documents'}</strong>
                <span>
                  {empty
                    ? 'Open a file from the workspace.'
                    : `Nothing matches “${query.trim()}”.`}
                </span>
              </div>
            ) : (
              filtered.map((doc) => (
                <DocMenuItem
                  key={doc.id}
                  variant={variant}
                  doc={doc}
                  active={doc.id === activeId}
                  onSelect={() => {
                    onActivate(doc.id)
                    setOpen(false)
                  }}
                />
              ))
            )}
          </div>
          {variant === 'A' ? (
            <div className="wcp-doc-foot">
              {filtered.length} of {docs.length} open
            </div>
          ) : null}
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  )
}

function DocMenuItem({
  variant,
  doc,
  active,
  onSelect,
}: {
  variant: VariantKey
  doc: DocumentInfo
  active: boolean
  onSelect: () => void
}): ReactElement {
  const parent = parentFolder(doc.path)
  if (variant === 'A') {
    return (
      <DropdownMenu.Item
        className={`wcp-doc-row wcp-doc-row-a${active ? ' is-active' : ''}`}
        aria-current={active ? 'true' : undefined}
        title={doc.path || doc.name}
        onSelect={onSelect}
      >
        <i className="ph ph-file-text wcp-doc-file" aria-hidden="true" />
        <span className="wcp-doc-name">{doc.name}</span>
        {parent ? <span className="wcp-doc-parent">{parent}</span> : null}
        <DocStatus doc={doc} />
        {active ? <i className="ph ph-check wcp-doc-check" aria-hidden="true" /> : null}
      </DropdownMenu.Item>
    )
  }
  if (variant === 'B') {
    return (
      <DropdownMenu.Item
        className={`wcp-doc-row wcp-doc-row-b${active ? ' is-active' : ''}`}
        aria-current={active ? 'true' : undefined}
        title={doc.path || doc.name}
        onSelect={onSelect}
      >
        <span className="wcp-doc-tile" aria-hidden="true">
          <i className="ph ph-file-text" />
        </span>
        <span className="wcp-doc-copy">
          <span className="wcp-doc-name">{doc.name}</span>
          <span className="wcp-doc-path">{doc.path || 'Untitled'}</span>
        </span>
        <DocStatus doc={doc} />
        {active ? <i className="ph ph-check wcp-doc-check" aria-hidden="true" /> : null}
      </DropdownMenu.Item>
    )
  }
  return (
    <DropdownMenu.Item
      className={`wcp-doc-row wcp-doc-row-c${active ? ' is-active' : ''}`}
      aria-current={active ? 'true' : undefined}
      title={doc.path || doc.name}
      onSelect={onSelect}
    >
      <span className="wcp-doc-name">{doc.name}</span>
      <span className="wcp-doc-parent">{parent || 'Untitled'}</span>
      <DocStatus doc={doc} />
    </DropdownMenu.Item>
  )
}

function DocStatus({ doc }: { doc: DocumentInfo }): ReactElement | null {
  if (doc.conflict) {
    return (
      <span className="wcp-state-dot wcp-state-conflict" title="Conflict">
        <span className="wcp-sr-only">Conflict</span>
      </span>
    )
  }
  if (doc.deleted) {
    return (
      <span className="wcp-state-dot wcp-state-deleted" title="Deleted">
        <span className="wcp-sr-only">Deleted</span>
      </span>
    )
  }
  if (doc.edited) {
    return (
      <span className="wcp-state-dot wcp-state-edited" title="Unsaved edits">
        <span className="wcp-sr-only">Unsaved edits</span>
      </span>
    )
  }
  if (doc.generating) {
    return (
      <span className="wcp-state-dot wcp-state-generating" title="Generating">
        <span className="wcp-sr-only">Generating</span>
      </span>
    )
  }
  return null
}

function DocEntryStatus({ entry }: { entry: NavEntry }): ReactElement | null {
  const label = entry.conflict
    ? 'Conflict'
    : entry.deleted
      ? 'Deleted'
      : entry.edited
        ? 'Unsaved edits'
        : ''
  if (!label) return null
  return (
    <span
      className={`wcp-state-dot ${entry.conflict ? 'wcp-state-conflict' : entry.deleted ? 'wcp-state-deleted' : 'wcp-state-edited'}`}
      title={label}
    >
      <span className="wcp-sr-only">{label}</span>
    </span>
  )
}

function VariantSwitcher({
  variant,
  onChange,
}: {
  variant: VariantKey
  onChange: (variant: VariantKey) => void
}): ReactElement {
  const index = VARIANTS.findIndex((item) => item.key === variant)
  const current = VARIANTS[index] ?? VARIANTS[0]
  const step = (direction: number) => {
    const next = VARIANTS[(index + direction + VARIANTS.length) % VARIANTS.length]
    onChange(next.key)
  }
  return (
    <div className="wcp-switch" role="group" aria-label="Prototype variant">
      <span className="wcp-switch-kicker">Prototype</span>
      <button type="button" className="wcp-switch-arrow" aria-label="Previous variant" onClick={() => step(-1)}>
        <i className="ph ph-caret-right wcp-flip-x" aria-hidden="true" />
      </button>
      <span className="wcp-switch-label" aria-live="polite">
        {current.key} · {current.name}
      </span>
      <button type="button" className="wcp-switch-arrow" aria-label="Next variant" onClick={() => step(1)}>
        <i className="ph ph-caret-right" aria-hidden="true" />
      </button>
    </div>
  )
}

function useTreeState(nav: NavSnapshot | null, query: string) {
  const [collapsed, setCollapsed] = useState<Set<string> | null>(null)

  useEffect(() => {
    if (!nav) return
    setCollapsed((current) => {
      if (current) return current
      const next = new Set<string>()
      for (const entry of nav.entries) {
        if (entry.kind === 'folder' && entry.folderState === 'collapsed') next.add(entry.path)
      }
      return next
    })
  }, [nav])

  const collapsedSet = collapsed ?? EMPTY_COLLAPSED
  return {
    rows: visibleRows(nav?.entries ?? [], query, collapsedSet),
    collapsed: collapsedSet,
    toggle: (path: string) => {
      setCollapsed((current) => {
        const next = new Set(current ?? [])
        if (next.has(path)) next.delete(path)
        else next.add(path)
        return next
      })
    },
  }
}

function visibleRows(entries: NavEntry[], query: string, collapsed: Set<string>): NavEntry[] {
  const needle = query.trim().toLowerCase()
  if (needle) {
    return entries.filter(
      (entry) =>
        entry.name.toLowerCase().includes(needle) ||
        entry.path.toLowerCase().includes(needle),
    )
  }
  const rows: NavEntry[] = []
  let hideBelow: number | null = null
  for (const entry of entries) {
    if (hideBelow !== null && entry.depth <= hideBelow) hideBelow = null
    if (hideBelow !== null) continue
    rows.push(entry)
    if (entry.kind === 'folder' && collapsed.has(entry.path)) hideBelow = entry.depth
  }
  return rows
}

function isActiveEntry(entry: NavEntry, props: VariantProps): boolean {
  if (entry.documentId) return entry.documentId === props.activeId
  return Boolean(props.activeDoc?.path && entry.path === props.activeDoc.path)
}

function openEntry(entry: NavEntry, onActivate: (id: string) => void): void {
  if (entry.documentId) onActivate(entry.documentId)
}

function matchesDoc(doc: DocumentInfo, query: string): boolean {
  const needle = query.trim().toLowerCase()
  if (!needle) return true
  return doc.name.toLowerCase().includes(needle) || doc.path.toLowerCase().includes(needle)
}

function tabLabel(docs: DocumentInfo[], doc: DocumentInfo): string {
  const duplicates = docs.filter((item) => item.name === doc.name).length
  if (duplicates < 2) return doc.name
  return parentFolder(doc.path) ? `${parentFolder(doc.path)}/${doc.name}` : doc.name
}

function parentFolder(path: string): string {
  if (!path) return ''
  const parts = path.replace(/\\/g, '/').split('/').filter(Boolean)
  return parts.length > 1 ? parts[parts.length - 2] : ''
}

function displayTitle(name: string): string {
  return name.replace(/\.(md|markdown)$/i, '')
}

function formatDate(timestamp: number): string {
  try {
    return new Date(timestamp).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
  } catch {
    return ''
  }
}

function workspaceLabel(snapshot: WorkspaceSnapshot | null): string {
  const name = snapshot?.name?.trim()
  if (name) return name
  const root = snapshot?.root?.trim() ?? ''
  if (!root) return 'Untitled'
  const parts = root.replace(/\\/g, '/').split('/').filter(Boolean)
  return parts[parts.length - 1] ?? 'Untitled'
}

function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : 'Could not read this workspace.'
}

function normalizeVariant(value: string): VariantKey {
  const normalized = String(value ?? '').trim().toLowerCase()
  if (normalized.includes('b') || normalized.includes('command')) return 'B'
  if (normalized.includes('c') || normalized.includes('inline')) return 'C'
  return 'A'
}

function stepVariant(current: VariantKey, direction: number): VariantKey {
  const index = VARIANTS.findIndex((item) => item.key === current)
  return VARIANTS[(index + direction + VARIANTS.length) % VARIANTS.length].key
}
