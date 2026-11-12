import { useCallback, useEffect, useLayoutEffect, useMemo, useReducer, useRef, useState } from 'react'
import { flushSync } from 'react-dom'
import { RightPane } from './components/RightPane'
import { PathCrumbs, SidePane } from './components/SidePane'
import { useViews, ViewHost } from './components/ViewHost'
import { CommandRegistry } from './core/CommandRegistry'
import { DocumentService } from './core/DocumentService'
import { EditorAccess } from './core/EditorAccess'
import { PluginRegistry } from './core/PluginRegistry'
import { ViewRegistry } from './core/ViewRegistry'
import type { AppPlugin } from './core/types'
import type { Disposable, DocumentInfo, HostAdapter, HostDocSnapshot, RecentWorkspace, WorkspaceAdapter, WorkspaceSnapshot } from './host/types'
import { wrapHostAsWorkspace } from './host/browserHost'
import { reportNativeEditState } from './host/mygoHost'
import { createMarkdownEditor, type EditorMode, type MarkdownEditor } from './editor/createEditor'
import {
  nativeEditCapabilities,
  registerNativeEditTarget,
} from './editor/nativeEdit'
import { createGenUIPlugin } from './plugins/genui'
import { createOutlinePlugin } from './plugins/outline'
import { ScrollArea } from './components/ScrollArea'
import { AppTooltip } from './components/Tooltip'
import './components/ui.css'
import { DropdownMenu, Tooltip } from 'radix-ui'

const EDIT_DEBOUNCE_MS = 250
export const NARROW_WINDOW_WIDTH = 1180

/**
 * What the center of the window shows: the editor as rich text or as
 * Markdown, or the id of a center view a plugin registered.
 */
type ViewMode = 'richtext' | 'markdown' | (string & {})

/** The view mode is remembered per document path in the page's storage. */
const VIEW_MODE_KEY = 'mgo.viewMode:'

function rememberedViewMode(path: string): string | null {
  if (path === '') return null
  try {
    return window.localStorage.getItem(VIEW_MODE_KEY + path)
  } catch {
    return null
  }
}

function rememberViewMode(path: string, mode: ViewMode): void {
  if (path === '') return
  try {
    window.localStorage.setItem(VIEW_MODE_KEY + path, mode)
  } catch {
    // No storage: the mode is a convenience, the next window starts in rich text.
  }
}

function measureIsNarrow(): boolean {
  return typeof window !== 'undefined' && window.innerWidth < NARROW_WINDOW_WIDTH
}

/**
 * Switches the center between the editor and a center view. The editor
 * stays mounted while hidden, so its state, selection and undo history
 * survive; its scroll position is put back when it shows again. When a
 * center view sends the reader to the source, it stays the way back until
 * the reader picks a mode.
 */
class ViewModeController {
  private center: string | null = null
  private savedScroll = 0

  constructor(
    private readonly setCenter: (id: string | null) => void,
    private readonly setReturnTo: (id: string | null) => void,
    private readonly editor: () => MarkdownEditor | null,
    private readonly scroller: () => HTMLElement | null,
  ) {}

  showCenter(id: string): void {
    this.setReturnTo(null)
    if (this.center === id) return
    if (this.center === null) this.savedScroll = this.scroller()?.scrollTop ?? 0
    this.center = id
    this.setCenter(id)
  }

  /**
   * Shows the editor, in `mode` when given. The editor is visible when this
   * returns, so a reveal right after it can scroll and focus.
   */
  showEditor(mode?: EditorMode): void {
    this.setReturnTo(null)
    this.leaveCenter()
    if (mode) this.editor()?.setMode(mode)
  }

  /**
   * Navigation to a place in the source: from a center view it lands in
   * rich text and keeps that view as the way back.
   */
  showSource(): void {
    const from = this.center
    if (from === null) return
    this.leaveCenter()
    this.editor()?.setMode('wysiwyg')
    this.setReturnTo(from)
  }

  /** ⌘/: between rich text and Markdown; from a center view, the one the editor is not in. */
  toggleEditorMode(): void {
    const editor = this.editor()
    if (!editor) return
    this.showEditor(editor.getMode() === 'wysiwyg' ? 'source' : 'wysiwyg')
  }

  /** The shown center view went away (its plugin was disabled). */
  dropCenter(): void {
    this.center = null
    this.setCenter(null)
  }

  private leaveCenter(): void {
    if (this.center === null) return
    this.center = null
    flushSync(() => this.setCenter(null))
    const scroller = this.scroller()
    if (scroller) scroller.scrollTop = this.savedScroll
  }
}

export function DocumentPane({
  host,
  plugins = [],
  active = true,
  workspaceRoot = '',
  onOpenPath,
  onReadyReveal,
}: {
  host: HostAdapter
  plugins?: AppPlugin[]
  active?: boolean
  workspaceRoot?: string
  onOpenPath?: (path: string, isFile: boolean) => void
  onReadyReveal?: (id: string, reveal: (offset: number) => boolean) => () => void
}): React.ReactElement {
  const editorRoot = useRef<HTMLDivElement>(null)
  const editorScroll = useRef<HTMLElement | null>(null)
  const editorRef = useRef<MarkdownEditor | null>(null)
  const [docState, setDocState] = useState<DocumentInfo | null>(null)
  const [version, setVersion] = useState(0)
  const [editorMode, setEditorMode] = useState<EditorMode>('wysiwyg')
  const [centerViewId, setCenterViewId] = useState<string | null>(null)
  const [returnToId, setReturnToId] = useState<string | null>(null)
  const [rightViewId, setRightViewId] = useState<string | null>('outline.panel')
  const [isNarrow, setIsNarrow] = useState<boolean>(() => measureIsNarrow())
  const [rightWantOpen, setRightWantOpen] = useState<boolean>(true)
  const [rightFloatOpen, setRightFloatOpen] = useState<boolean>(false)
  const [ready, setReady] = useState(false)
  const [bootstrapError, setBootstrapError] = useState('')
  const [, refreshPlugins] = useReducer((value: number) => value + 1, 0)
  const restored = useRef(false)
  const reportedMode = useRef(false)
  const activeRef = useRef(active)
  const revealBinder = useRef(onReadyReveal)
  activeRef.current = active
  revealBinder.current = onReadyReveal

  const services = useMemo(() => {
    const documents = new DocumentService()
    const commands = new CommandRegistry()
    const views = new ViewRegistry()
    const editor = new EditorAccess()
    const plugins = new PluginRegistry(documents, views, commands, editor, host)
    return { documents, commands, views, editor, plugins }
  }, [host])

  const [modes] = useState(
    () =>
      new ViewModeController(
        setCenterViewId,
        setReturnToId,
        () => editorRef.current,
        () => editorScroll.current,
      ),
  )

  const centerViews = useViews(services.views, 'center')
  const rightViews = useViews(services.views, 'right')
  const centerView = centerViews.find((view) => view.id === centerViewId) ?? null
  const returnTo = centerView ? null : (centerViews.find((view) => view.id === returnToId) ?? null)
  const viewMode: ViewMode = centerView ? centerView.id : editorMode === 'source' ? 'markdown' : 'richtext'

  const rightShown = rightViews.length > 0 && (isNarrow ? rightFloatOpen : rightWantOpen)
  const rightFloating = isNarrow && rightFloatOpen

  const toggleRightPane = () => {
    if (measureIsNarrow()) {
      setIsNarrow(true)
      setRightFloatOpen((open) => !open)
    } else {
      setIsNarrow(false)
      setRightWantOpen((open) => !open)
    }
  }

  useEffect(() => {
    const syncViewport = () => {
      const narrow = measureIsNarrow()
      setIsNarrow((prev) => {
        if (narrow && !prev) setRightFloatOpen(false)
        return narrow
      })
    }
    syncViewport()
    window.addEventListener('resize', syncViewport)
    return () => window.removeEventListener('resize', syncViewport)
  }, [])

  useEffect(() => {
    if (centerViewId !== null && centerView === null) modes.dropCenter()
  }, [centerViewId, centerView, modes])

  useEffect(() => {
    let disposed = false
    const cleanups: Disposable[] = []
    let debounceTimer: number | undefined
    const startup: {
      reload: { snapshot: HostDocSnapshot; stateEvents: number } | null
      state: DocumentInfo | null
      stateEvents: number
    } = {
      reload: null,
      state: null,
      stateEvents: 0,
    }
    let appliedVersion = 0

    const handleShortcuts = (event: KeyboardEvent) => {
      if (!activeRef.current) return
      if (event.repeat) return
      if ((event.metaKey || event.ctrlKey) && event.altKey && event.code === 'Digit0') {
        event.preventDefault()
        event.stopPropagation()
        toggleRightPane()
        return
      }
      if (!(event.metaKey || event.ctrlKey) || event.altKey || event.key !== '/') return
      if (!editorRef.current) return
      event.preventDefault()
      event.stopPropagation()
      modes.toggleEditorMode()
    }

    window.addEventListener('keydown', handleShortcuts, true)
    cleanups.push({
      dispose() {
        window.removeEventListener('keydown', handleShortcuts, true)
      },
    })

    const applyDocumentState = (state: DocumentInfo) => {
      if (disposed) return
      setDocState(state)
      services.documents.setPath(state.path)
      editorRef.current?.setReadonly(state.readOnly)
    }

    const applyReload = (reload: HostDocSnapshot) => {
      if (!editorRef.current || reload.version <= appliedVersion) return
      if (debounceTimer !== undefined) {
        window.clearTimeout(debounceTimer)
        debounceTimer = undefined
      }
      editorRef.current.setMarkdown(reload.text)
      services.documents.setSnapshot(reload.text, reload.version)
      setVersion(reload.version)
      appliedVersion = reload.version
      applyDocumentState(reload.state)
    }

    const onReload = (reload: HostDocSnapshot) => {
      if (disposed || reload.version <= appliedVersion) return
      if (editorRef.current) {
        applyReload(reload)
      } else if (!startup.reload || reload.version > startup.reload.snapshot.version) {
        startup.reload = { snapshot: reload, stateEvents: startup.stateEvents }
      }
    }

    cleanups.push(host.onState((state) => {
      startup.stateEvents += 1
      startup.state = state
      applyDocumentState(state)
    }))
    cleanups.push(host.onReload(onReload))
    setBootstrapError('')

    void (async () => {
      const stateEventsAtLoad = startup.stateEvents
      const snapshot = await host.load()
      if (disposed) return
      const reloadAtLoad = startup.reload && startup.reload.snapshot.version > snapshot.version ? startup.reload : null
      const initial = reloadAtLoad?.snapshot ?? snapshot
      const stateEventsAtInitial = reloadAtLoad?.stateEvents ?? stateEventsAtLoad
      const initialState = startup.stateEvents > stateEventsAtInitial ? startup.state ?? initial.state : initial.state

      appliedVersion = initial.version
      services.documents.setSnapshot(initial.text, initial.version)
      applyDocumentState(initialState)
      setVersion(initial.version)

      const root = editorRoot.current
      if (!root) return
      const editor = await createMarkdownEditor(root, { initialText: initial.text })
      if (disposed) {
        await editor.destroy()
        return
      }
      editorRef.current = editor
      services.editor.attach(editor)
      editor.setReadonly(initialState.readOnly)
      const reloadAtReady = startup.reload && startup.reload.snapshot.version > appliedVersion ? startup.reload : null
      if (reloadAtReady) applyReload(reloadAtReady.snapshot)
      startup.reload = null
      const stateEventsAtSnapshot = reloadAtReady?.stateEvents ?? stateEventsAtInitial
      if (startup.stateEvents > stateEventsAtSnapshot && startup.state) applyDocumentState(startup.state)

      if (revealBinder.current) {
        cleanups.push({
          dispose: revealBinder.current(host.id, (offset) => {
            modes.showSource()
            return editor.reveal(offset)
          }),
        })
      }
      const reportHistory = () => {
        if (!activeRef.current) return
        const caps = nativeEditCapabilities(host.id)
        reportNativeEditState(host.id, caps.canUndo, caps.canRedo)
      }
      cleanups.push(
        registerNativeEditTarget(
          host.id,
          {
            capabilities: () => ({ canUndo: editor.canUndo(), canRedo: editor.canRedo() }),
            editorVisible: () => {
              const pane = editorRoot.current?.closest('.editor-pane')
              return pane instanceof HTMLElement && !pane.hidden
            },
            perform(command) {
              const acted = command === 'undo' ? editor.undo() : editor.redo()
              if (acted && activeRef.current) reportHistory()
              return acted
            },
          },
          editorRoot.current?.closest('.app') ?? editorRoot.current,
        ),
      )
      cleanups.push(editor.onHistoryChange(reportHistory))
      reportHistory()
      const onFocus = () => {
        if (activeRef.current) reportHistory()
      }
      document.addEventListener('focusin', onFocus, true)
      document.addEventListener('focusout', onFocus, true)
      cleanups.push({
        dispose() {
          document.removeEventListener('focusin', onFocus, true)
          document.removeEventListener('focusout', onFocus, true)
        },
      })

      cleanups.push(
        services.commands.register('editor.reveal', (offset) => {
          const value = Number(offset)
          if (!Number.isFinite(value)) return false
          modes.showSource()
          return editor.reveal(value)
        }),
      )

      cleanups.push(
        services.commands.register('editor.select', (from, to) => {
          const start = Number(from)
          const end = Number(to)
          if (!Number.isFinite(start) || !Number.isFinite(end)) return false
          modes.showSource()
          return editor.select(start, end)
        }),
      )

      // Adds to the document without leaving the view that asked for it.
      cleanups.push(
        services.commands.register('editor.addNote', (offset, text) => {
          const at = Number(offset)
          if (!Number.isFinite(at) || typeof text !== 'string') return false
          return editor.addNote(at, text)
        }),
      )

      cleanups.push(
        services.commands.register('editor.toggleSourceMode', () => {
          modes.toggleEditorMode()
          return editor.getMode()
        }),
      )

      cleanups.push(
        services.commands.register('rightPane.show', (targetViewId) => {
          if (typeof targetViewId === 'string' && targetViewId !== '') {
            setRightViewId(targetViewId)
          }
          if (measureIsNarrow()) {
            setIsNarrow(true)
            setRightFloatOpen(true)
          } else {
            setIsNarrow(false)
            setRightWantOpen(true)
          }
          return true
        }),
      )

      cleanups.push(
        services.commands.register('centerPane.show', (targetViewId) => {
          if (typeof targetViewId === 'string' && targetViewId !== '') {
            modes.showCenter(targetViewId)
          }
          return true
        }),
      )

      cleanups.push(editor.onModeChange(setEditorMode))

      cleanups.push(
        editor.onChange((markdown) => {
          services.documents.setText(markdown)
          if (debounceTimer !== undefined) window.clearTimeout(debounceTimer)
          debounceTimer = window.setTimeout(() => {
            debounceTimer = undefined
            void host.update(markdown)
          }, EDIT_DEBOUNCE_MS)
        }),
      )
      // What this pane really holds: the text on screen now, else the
      // core document state the host loaded. Never a buffer that only
      // changes were written to, so a pane that was remounted or has not
      // mounted an editor cannot deliver an invented empty buffer. The
      // host's flush is the one layer that writes it back, reading this.
      const currentText = () => editorRef.current?.getMarkdown() ?? services.documents.snapshot().text
      if (host.bindFlush) {
        cleanups.push(
          host.bindFlush(() => {
            // The delivery this reader feeds supersedes the debounce.
            if (debounceTimer !== undefined) {
              window.clearTimeout(debounceTimer)
              debounceTimer = undefined
            }
            return currentText()
          }),
        )
      }

      const appPlugins = [createOutlinePlugin(), createGenUIPlugin(), ...plugins]
      for (const plugin of appPlugins) {
        if (disposed) return
        services.plugins.register(plugin)
      }
      for (const plugin of appPlugins) {
        if (disposed) return
        try {
          await services.plugins.enable(plugin.id)
        } catch (error) {
          // Unmounting detaches the editor; a pending activation that resumes
          // afterwards is expected to fail and must not become unhandled.
          if (disposed) return
          throw error
        }
        if (disposed) return
      }
      refreshPlugins()
      setReady(true)
    })().catch((error: unknown) => {
      if (!disposed) setBootstrapError(error instanceof Error ? error.message : String(error))
    })

    return () => {
      disposed = true
      if (debounceTimer !== undefined) window.clearTimeout(debounceTimer)
      for (const cleanup of cleanups) cleanup.dispose()
      for (const plugin of services.plugins.list()) services.plugins.disable(plugin.id)
      services.editor.detach()
      void editorRef.current?.destroy()
      editorRef.current = null
    }
  }, [host, services, modes])

  // Reopen in the mode the document was last shown in, once its views exist.
  useLayoutEffect(() => {
    if (!ready || restored.current || !docState) return
    restored.current = true
    const saved = docState.viewMode || rememberedViewMode(docState.path)
    if (saved === 'markdown') editorRef.current?.setMode('source')
    else if (saved && services.views.getAll().some((v) => v.location === 'center' && v.id === saved)) {
      modes.showCenter(saved)
    }
  }, [ready, docState, services, modes])

  useEffect(() => {
    if (!restored.current || !docState) return
    rememberViewMode(docState.path, viewMode)
    // The first run is the mode being restored, which the session already
    // holds. Writing it back would race the restore with the default.
    if (!reportedMode.current) {
      reportedMode.current = true
      return
    }
    if (docState.viewMode === viewMode) return
    void host.setViewMode(viewMode)
  }, [viewMode, docState, host])

  useEffect(() => {
    if (!active) return
    const caps = nativeEditCapabilities(host.id)
    reportNativeEditState(host.id, caps.canUndo, caps.canRedo)
  }, [active, host.id, centerViewId, editorMode])

  const modeChoices = [
    { id: 'richtext', label: 'Rich text', title: 'Rich text (⌘/)' },
    { id: 'markdown', label: 'Markdown', title: 'Markdown source (⌘/)' },
    ...centerViews.map((view) => ({ id: view.id, label: view.title, title: view.title })),
  ]

  const chooseMode = (id: string) => {
    if (id === 'richtext') modes.showEditor('wysiwyg')
    else if (id === 'markdown') modes.showEditor('source')
    else modes.showCenter(id)
  }

  const handleCenterClick = () => {
    if (isNarrow && rightFloatOpen) {
      setRightFloatOpen(false)
    }
  }

  return (
    <div
      className={`app mgg win${rightShown && !rightFloating ? ' rp-dock' : ''}${rightFloating ? ' rp-float' : ''}`}
      hidden={!active}
      data-document-id={host.id}
    >
      <div className="app-body">
        <div className="main center-column">
          <section className="sheet">
            <header className="app-header docbar">
              <div className="doc-title crumbs">
                {docState?.path ? (
                  <PathCrumbs
                    root={workspaceRoot}
                    path={docState.path}
                    onOpen={(path, isFile) => onOpenPath?.(path, isFile)}
                  />
                ) : (
                  <span className="doc-name last">{docState?.name ?? 'Loading…'}</span>
                )}
                {docState?.edited ? <span className="edited-dot" title="Unsaved edits" /> : null}
              </div>
              <div className="header-actions">
                <span className="status meta">
                  {docState?.edited ? 'Edited' : 'Saved'}
                  {docState?.conflict ? ' · conflict' : ''}
                  {docState?.generating
                    ? ' · generating — wait or cancel in the GenUI panel'
                    : ''}
                </span>
                <span className="version-chip meta">v{version}</span>
                <div className="view-modes seg" role="radiogroup" aria-label="View mode">
                  {modeChoices.map((choice) => (
                    <button
                      key={choice.id}
                      type="button"
                      role="radio"
                      className="view-mode"
                      data-testid={`view-mode-${choice.id}`}
                      aria-checked={viewMode === choice.id}
                      title={choice.title}
                      onClick={() => chooseMode(choice.id)}
                      disabled={!ready}
                    >
                      {choice.label}
                    </button>
                  ))}
                </div>
                <button
                  type="button"
                  className="toggle-button pill icon quiet rp-tg"
                  data-testid="right-pane-toggle"
                  data-rp-toggle
                  aria-expanded={rightShown}
                  aria-label={rightShown ? 'Hide right panel' : 'Show right panel'}
                  title={`${rightShown ? 'Hide' : 'Show'} right panel (⌥⌘0)`}
                  onClick={toggleRightPane}
                  disabled={!ready}
                >
                  <span aria-hidden="true">◨</span>
                </button>
              </div>
            </header>

            {docState?.conflict ? (
              <div className="banner banner-conflict" role="alert">
                The file changed on disk. Saving will ask which version to keep.
              </div>
            ) : null}
            {docState?.deleted ? (
              <div className="banner banner-deleted" role="alert">
                The file was deleted on disk. Edits are kept in the window until you save.
              </div>
            ) : null}
            {docState?.readOnly ? <div className="banner banner-readonly">Read only</div> : null}
            {docState?.actionError ? (
              <div className="banner" role="alert" data-testid="doc-action-error">
                {docState.actionError}
              </div>
            ) : null}
            {docState?.restoreError ? (
              <div className="banner" role="alert" data-testid="doc-restore-error">
                {docState.restoreError}
              </div>
            ) : null}
            {bootstrapError ? (
              <div className="banner" role="alert" data-testid="document-bootstrap-error">
                Could not open this document: {bootstrapError}
              </div>
            ) : null}

            <div
              className="center-stage"
              data-testid="center-stage"
              onClick={handleCenterClick}
            >
              <main className="editor-pane" hidden={centerView !== null}>
                {returnTo ? (
                  <button
                    type="button"
                    className="return-to-view"
                    data-testid="return-to-view"
                    onClick={() => modes.showCenter(returnTo.id)}
                  >
                    ← Back to {returnTo.title}
                  </button>
                ) : null}
                <ScrollArea
                  className="editor-scroll"
                  contentClassName="editor-scroll-content"
                  onViewport={(viewport) => {
                    editorScroll.current = viewport
                  }}
                >
                  <div className="crepe-root" ref={editorRoot} data-testid="editor-root" />
                </ScrollArea>
              </main>
              {centerView ? (
                <section
                  className="center-pane"
                  data-testid="center-pane"
                  data-view-id={centerView.id}
                  aria-label={centerView.title}
                >
                  <ViewHost
                    key={centerView.id}
                    view={centerView}
                    className="center-view"
                    contentClassName="center-view-content"
                    scrollable
                  />
                </section>
              ) : null}
            </div>
          </section>
        </div>

        <RightPane
          views={services.views}
          activeId={rightViewId}
          onSelect={setRightViewId}
          open={rightShown}
          floating={rightFloating}
        />
      </div>

      <footer className="status-bar">
        <span className="status-name">{docState?.path ?? '—'}</span>
        <span className="status-state">
          {docState?.edited ? 'Edited' : 'Saved'}
          {docState?.conflict ? ' · conflict' : ''}
          {docState?.generating ? ' · generating' : ''}
        </span>
      </footer>
    </div>
  )
}


export function App({
  host,
  workspace,
  plugins = [],
}: {
  host?: HostAdapter
  workspace?: WorkspaceAdapter
  plugins?: AppPlugin[]
}): React.ReactElement {
  const resolved =
    workspace ??
    (host ? wrapHostAsWorkspace(host) : null)
  if (!resolved) throw new Error('App needs a workspace or a document host')
  return (
    <Tooltip.Provider delayDuration={400} skipDelayDuration={250}>
      <WorkspaceApp workspace={resolved} plugins={plugins} />
    </Tooltip.Provider>
  )
}

function tabLabel(docs: { id: string; name: string; path: string }[], doc: { id: string; name: string; path: string }): string {
  const same = docs.filter((other) => other.name === doc.name)
  if (same.length < 2) return doc.name
  const parent = doc.path.replace(/\\/g, '/').split('/').slice(-2, -1)[0]
  return parent ? `${doc.name} — ${parent}` : doc.path || doc.name
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
      {doc.generating ? <StatusTooltip label="Generating" className="tab-generating" testId={`doc-tab-generating-${doc.id}`} /> : null}
      {doc.conflict ? <StatusTooltip label="Conflict" className="tab-conflict" /> : null}
      {doc.deleted ? <StatusTooltip label="Deleted" className="tab-deleted" /> : null}
      {doc.readOnly ? <StatusTooltip label="Read only" className="tab-readonly" /> : null}
    </>
  )
}

function WorkspaceApp({
  workspace,
  plugins = [],
}: {
  workspace: WorkspaceAdapter
  plugins?: AppPlugin[]
}): React.ReactElement {
  const [snap, setSnap] = useState<WorkspaceSnapshot | null>(null)
  const [recents, setRecents] = useState<RecentWorkspace[]>([])
  const panes = useRef(new Map<string, HostAdapter>())
  const reveals = useRef(new Map<string, (offset: number) => boolean>())
  const emptyViews = useMemo(() => new ViewRegistry(), [])

  useEffect(() => {
    let disposed = false
    void workspace.snapshot().then((next) => {
      if (!disposed) setSnap(next)
    })
    void workspace.recentWorkspaces().then((list) => {
      if (!disposed) setRecents(list)
    })
    const sub = workspace.subscribe((next) => setSnap(next))
    return () => {
      disposed = true
      sub.dispose()
    }
  }, [workspace])

  const documents = snap?.documents ?? []
  const activeId = snap?.activeId ?? documents[0]?.id ?? ''
  const tabsRef = useRef<HTMLDivElement>(null)
  const tabRefs = useRef(new Map<string, HTMLButtonElement>())
  const tabFocusTarget = useRef<string | null>(null)
  const tabMenuButtonRef = useRef<HTMLButtonElement>(null)
  const tabMenuSelectedId = useRef<string | null>(null)
  const [visibleRange, setVisibleRange] = useState({ start: 0, end: 0 })
  const [tabMenuOpen, setTabMenuOpen] = useState(false)
  const [tabFilter, setTabFilter] = useState('')
  const hiddenDocuments = useMemo(
    () => [...documents.slice(0, visibleRange.start), ...documents.slice(visibleRange.end)],
    [documents, visibleRange],
  )
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
      setVisibleRange((current) => (current.start === 0 && current.end === documents.length
        ? current
        : { start: 0, end: documents.length }))
      return
    }
    const budget = fullBudget - overflowWidth - gap
    const activeIndex = documents.findIndex((doc) => doc.id === activeId)
    let start = 0
    let end = documents.length
    if (activeIndex >= 0) {
      const throughActive = widths.slice(0, activeIndex + 1).reduce((sum, width, index) => sum + width + (index === 0 ? 0 : gap), 0)
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

  for (const doc of documents) {
    if (!panes.current.has(doc.id)) panes.current.set(doc.id, workspace.document(doc.id))
  }
  for (const id of [...panes.current.keys()]) {
    if (!documents.some((doc) => doc.id === id)) panes.current.delete(id)
  }

  const activate = (id: string, focusTab = false) => {
    tabFocusTarget.current = focusTab ? id : null
    void workspace.activate(id)
  }

  const closeTab = (id: string) => {
    void workspace.closeDocument(id)
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
      closeTab(activeId)
    }
  }

  const openPath = (path: string, isFile: boolean) => {
    if (isFile) void workspace.open(path)
    else void workspace.revealPath(path)
  }

  const jumpHeading = (documentId: string | undefined, path: string, offset: number) => {
    void (async () => {
      const info = documentId ? { id: documentId } : await workspace.open(path)
      const id = documentId ?? info.id
      await workspace.activate(id)
      const tryReveal = () => reveals.current.get(id)?.(offset)
      if (tryReveal()) return
      window.setTimeout(() => {
        tryReveal()
      }, 80)
    })()
  }

  const showWelcome = !snap || (snap.kind !== 'folder' && documents.length === 0)
  const documentTabs = (
    <div className="doc-tabs-row">
      <div
        className="doc-tabs"
        role="tablist"
        aria-label="Documents"
        onKeyDown={onTabsKey}
        data-testid="doc-tabs"
        ref={tabsRef}
      >
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
                  closeTab(doc.id)
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
              value={tabFilter}
              onChange={(event) => setTabFilter(event.target.value)}
              onKeyDownCapture={(event) => {
                if (['ArrowUp', 'Home', 'End'].includes(event.key)) event.stopPropagation()
                if (event.key === 'ArrowDown') {
                  event.preventDefault()
                  event.stopPropagation()
                  event.currentTarget.closest('[data-testid="doc-tab-menu"]')?.querySelector<HTMLElement>('[role="menuitemradio"]')?.focus()
                }
              }}
              autoFocus
            />
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
                <p className="doc-tab-menu-empty" role="status">No matching tabs</p>
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
    </div>
  )

  return (
    <div className="workspace-shell">
      {snap?.restoreError ? (
        <div className="banner" role="alert" data-testid="restore-error">
          {snap.restoreError}
        </div>
      ) : null}
      <div className="workspace-body">
        <SidePane
          views={emptyViews}
          workspace={workspace}
          workspaceSnap={snap}
          doc={documents.find((d) => d.id === activeId) ?? null}
          onJumpHeading={jumpHeading}
        />
        <div className="workspace-main">
          {documentTabs}
          {documents.map((doc) => (
            <DocumentPane
              key={doc.id}
              host={panes.current.get(doc.id)!}
              plugins={plugins}
              active={doc.id === activeId}
              workspaceRoot={snap?.root ?? ''}
              onOpenPath={openPath}
              onReadyReveal={(id, reveal) => {
                reveals.current.set(id, reveal)
                return () => {
                  if (reveals.current.get(id) === reveal) reveals.current.delete(id)
                }
              }}
            />
          ))}
          {documents.length === 0 ? (
            showWelcome ? (
              <Welcome
                recents={recents}
                onOpenFolder={() => {
                  void workspace.pickFolder().then(() => workspace.recentWorkspaces().then(setRecents))
                }}
                onNew={() => {
                  void workspace.newDocument()
                }}
                onOpenRecent={(root) => {
                  void workspace.openWorkspace(root)
                }}
              />
            ) : (
              <div className="workspace-empty" data-testid="workspace-empty">
                Open a Markdown file from this folder.
              </div>
            )
          ) : null}
        </div>
      </div>
    </div>
  )
}

function Welcome({
  recents,
  onOpenFolder,
  onNew,
  onOpenRecent,
}: {
  recents: RecentWorkspace[]
  onOpenFolder: () => void
  onNew: () => void
  onOpenRecent: (root: string) => void
}): React.ReactElement {
  return (
    <div className="welcome" data-testid="welcome">
      <h1>Welcome</h1>
      <p>Open a folder to browse Markdown files, or start an untitled document.</p>
      <div className="welcome-actions">
        <button type="button" className="pill primary" data-testid="open-folder" onClick={onOpenFolder}>
          Open Folder
        </button>
        <button type="button" className="pill" data-testid="new-document" onClick={onNew}>
          New Document
        </button>
      </div>
      <section className="recent-workspaces" aria-label="Recent workspaces">
        <h2>Recent Workspaces</h2>
        {recents.length === 0 ? (
          <p className="nav-empty" data-testid="recent-empty">No recent workspaces yet.</p>
        ) : (
          <ul data-testid="recent-workspaces">
            {recents.map((item) => (
              <li key={item.root}>
                <button type="button" className="row file" onClick={() => onOpenRecent(item.root)}>
                  <span className="body">
                    <span className="l1"><span className="nm">{item.name}</span></span>
                    <span className="l2"><span className="s">{item.root}</span></span>
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}
