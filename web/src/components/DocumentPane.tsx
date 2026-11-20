import { useEffect, useLayoutEffect, useReducer, useRef, useState } from 'react'
import { flushSync } from 'react-dom'
import { CommandRegistry } from '../core/CommandRegistry'
import { DocumentService } from '../core/DocumentService'
import { EditorAccess } from '../core/EditorAccess'
import { PluginRegistry } from '../core/PluginRegistry'
import { ViewRegistry } from '../core/ViewRegistry'
import type { AppPlugin } from '../core/types'
import { createMarkdownEditor, type EditorMode, type MarkdownEditor } from '../editor/createEditor'
import { hasNativeFieldHistory, nativeEditCapabilities, registerNativeEditTarget } from '../editor/nativeEdit'
import { reportNativeEditState } from '../host/mygoHost'
import type { Disposable, DocumentInfo, HostAdapter, HostDocSnapshot } from '../host/types'
import { isRecord } from '../lib/guards'
import { DesktopPlugins } from '../plugins/DesktopPlugins'
import { createGenUIPlugin } from '../plugins/genui'
import { createOutlinePlugin } from '../plugins/outline'
import { MovingCapsule } from './MovingCapsule'
import { ScrollArea } from './ScrollArea'
import { PathCrumbs } from './SidePane'
import { useViews, ViewHost, ViewStatusIndicator } from './ViewHost'

const EDIT_DEBOUNCE_MS = 250
const VIEW_MODE_KEY = 'mgo.viewMode:'

type ViewMode = 'preview' | 'markdown' | (string & {})

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
  }
}

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

  showEditor(mode?: EditorMode): void {
    this.setReturnTo(null)
    this.leaveCenter()
    if (mode) this.editor()?.setMode(mode)
  }

  showSource(): void {
    const from = this.center
    if (from === null) return
    this.leaveCenter()
    this.editor()?.setMode('preview')
    this.setReturnTo(from)
  }

  toggleEditorMode(): void {
    const editor = this.editor()
    if (!editor) return
    this.showEditor(editor.getMode() === 'preview' ? 'source' : 'preview')
  }

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

export function createDocumentServices(host: HostAdapter) {
  const documents = new DocumentService()
  const commands = new CommandRegistry()
  const views = new ViewRegistry()
  const editor = new EditorAccess()
  const plugins = new PluginRegistry(documents, views, commands, editor, host)
  return { host, documents, commands, views, editor, plugins }
}

export type DocumentServices = ReturnType<typeof createDocumentServices>

export function DocumentPane({
  services,
  plugins,
  active,
  workspaceRoot,
  drawerOpen,
  closeDrawers,
  onOpenPath,
  onReadyReveal,
}: {
  services: DocumentServices
  plugins: AppPlugin[]
  active: boolean
  workspaceRoot: string
  drawerOpen: boolean
  closeDrawers: () => void
  onOpenPath: (path: string, isFile: boolean) => void
  onReadyReveal: (id: string, reveal: (offset: number) => boolean) => () => void
}): React.ReactElement {
  const { host } = services
  const editorRoot = useRef<HTMLDivElement>(null)
  const editorScroll = useRef<HTMLElement | null>(null)
  const editorRef = useRef<MarkdownEditor | null>(null)
  const viewModesRef = useRef<HTMLDivElement>(null)
  const [docState, setDocState] = useState<DocumentInfo | null>(null)
  const [version, setVersion] = useState(0)
  const [editorMode, setEditorMode] = useState<EditorMode>('preview')
  const [centerViewId, setCenterViewId] = useState<string | null>(null)
  const [returnToId, setReturnToId] = useState<string | null>(null)
  const [ready, setReady] = useState(false)
  const [bootstrapError, setBootstrapError] = useState('')
  const [, refreshPlugins] = useReducer((value: number) => value + 1, 0)
  const restored = useRef(false)
  const reportedMode = useRef(false)
  const activeRef = useRef(active)
  const revealBinder = useRef(onReadyReveal)
  activeRef.current = active
  revealBinder.current = onReadyReveal

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
  const documentActions = useViews(services.views, 'document')
  const centerView = centerViews.find((view) => view.id === centerViewId) ?? null
  const returnTo = centerView ? null : (centerViews.find((view) => view.id === returnToId) ?? null)
  const viewMode: ViewMode = centerView ? centerView.id : editorMode === 'source' ? 'markdown' : 'preview'

  useLayoutEffect(() => {
    if (!drawerOpen || !active) return
    const selection = window.getSelection()
    if (selection && editorRoot.current?.contains(selection.anchorNode)) selection.removeAllRanges()
  }, [drawerOpen, active])

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

    let latestState: DocumentInfo | undefined
    const applyDocumentState = (state: DocumentInfo) => {
      if (disposed) return
      latestState = state
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

    cleanups.push(
      host.onState((state) => {
        startup.stateEvents += 1
        startup.state = state
        applyDocumentState(state)
      }),
    )
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
      const editor = await createMarkdownEditor(root, {
        initialText: initial.text,
        docKey: initialState.path || host.id,
      })
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
      const onTextInput = (event: Event) => {
        if (hasNativeFieldHistory(event.target)) reportHistory()
      }
      document.addEventListener('focusin', onFocus, true)
      document.addEventListener('focusout', onFocus, true)
      document.addEventListener('input', onTextInput)
      cleanups.push({
        dispose() {
          document.removeEventListener('focusin', onFocus, true)
          document.removeEventListener('focusout', onFocus, true)
          document.removeEventListener('input', onTextInput)
        },
      })

      cleanups.push(
        services.commands.register('editor.reveal', (offset) => {
          const value = Number(offset)
          if (!Number.isFinite(value)) return false
          closeDrawers()
          modes.showSource()
          return editor.reveal(value)
        }),
      )

      cleanups.push(
        services.commands.register('editor.select', (from, to) => {
          const start = Number(from)
          const end = Number(to)
          if (!Number.isFinite(start) || !Number.isFinite(end)) return false
          closeDrawers()
          modes.showSource()
          return editor.select(start, end)
        }),
      )

      cleanups.push(
        services.commands.register('editor.applyMarkdown', (base, next) => {
          const state = latestState
          if (
            !activeRef.current ||
            !state ||
            state.readOnly ||
            state.conflict ||
            state.deleted ||
            !isRecord(base) ||
            typeof base.text !== 'string' ||
            typeof next !== 'string' ||
            base.path !== state.path ||
            base.version !== appliedVersion ||
            !editor.applyMarkdown(base.text, next)
          ) {
            throw new Error('The document changed. Review the current text before applying this edit.')
          }
          modes.showEditor()
          return true
        }),
      )

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
      const currentText = () => editorRef.current?.getMarkdown() ?? services.documents.snapshot().text
      if (host.bindFlush) {
        cleanups.push(
          host.bindFlush(() => {
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
  }, [host, services, modes, closeDrawers])

  useLayoutEffect(() => {
    if (!ready || restored.current || !docState) return
    restored.current = true
    const saved = docState.viewMode || rememberedViewMode(docState.path)
    if (saved === 'preview' || saved === 'richtext') return
    if (saved === 'markdown') editorRef.current?.setMode('source')
    else if (saved && services.views.getAll().some((v) => v.location === 'center' && v.id === saved)) {
      modes.showCenter(saved)
    }
  }, [ready, docState, services, modes])

  useEffect(() => {
    if (!restored.current || !docState) return
    rememberViewMode(docState.path, viewMode)
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
    { id: 'preview', label: 'Preview', title: 'Markdown Preview (⌘/)', status: undefined },
    { id: 'markdown', label: 'Markdown', title: 'Markdown source (⌘/)', status: undefined },
    ...centerViews.map((view) => ({ id: view.id, label: view.title, title: view.title, status: view.status })),
  ]

  const chooseMode = (id: string) => {
    if (id === 'preview') modes.showEditor('preview')
    else if (id === 'markdown') modes.showEditor('source')
    else modes.showCenter(id)
  }

  return (
    <div className="app mgg win" hidden={!active} data-document-id={host.id}>
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
                {docState?.edited || docState?.conflict || docState?.generating ? (
                  <span className="status meta">
                    {docState.edited ? 'Edited' : ''}
                    {docState.conflict ? `${docState.edited ? ' · ' : ''}conflict` : ''}
                    {docState.generating
                      ? `${docState.edited || docState.conflict ? ' · ' : ''}generating — wait or cancel in GenUI settings`
                      : ''}
                  </span>
                ) : null}
                <span className="version-chip meta">v{version}</span>
                <div className="view-modes seg" role="radiogroup" aria-label="View mode" ref={viewModesRef}>
                  <MovingCapsule
                    trackRef={viewModesRef}
                    selector="[aria-checked='true']"
                    watch={[active, ready, viewMode, centerViews.length]}
                  />
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
                      {choice.status && (
                        <ViewStatusIndicator status={choice.status} testId={`view-mode-dot-${choice.id}`} />
                      )}
                    </button>
                  ))}
                </div>
                {ready && <DesktopPlugins registry={services.plugins} documentID={host.id} />}
                {active && documentActions.map((view) => <ViewHost key={view.id} view={view} />)}
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

            <div className="center-stage" data-testid="center-stage" onClick={closeDrawers}>
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
      </div>

      <footer className="status-bar">
        <span className="status-name" title={docState?.path ?? '—'}>
          {docState?.path ?? '—'}
        </span>
        <span className="status-state">
          {!docState ? 'Loading…' : docState.edited ? 'Edited' : 'Saved'}
          {docState?.conflict ? ' · conflict' : ''}
          {docState?.generating ? ' · generating' : ''}
        </span>
      </footer>
    </div>
  )
}
