import { useEffect, useLayoutEffect, useMemo, useReducer, useRef, useState } from 'react'
import { flushSync } from 'react-dom'
import { SidePane } from './components/SidePane'
import { useViews, ViewHost } from './components/ViewHost'
import { CommandRegistry } from './core/CommandRegistry'
import { DocumentService } from './core/DocumentService'
import { EditorAccess } from './core/EditorAccess'
import { PluginRegistry } from './core/PluginRegistry'
import { ViewRegistry } from './core/ViewRegistry'
import type { AppPlugin } from './core/types'
import type { Disposable, DocState, HostAdapter } from './host/types'
import { createMarkdownEditor, type EditorMode, type MarkdownEditor } from './editor/createEditor'
import { createGenUIPlugin } from './plugins/genui'
import { createOutlinePlugin } from './plugins/outline'

const EDIT_DEBOUNCE_MS = 250

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

/**
 * Switches the center between the editor and a center view. The editor
 * stays mounted while hidden, so its state, selection and undo history
 * survive; its scroll position is put back when it shows again.
 */
class ViewModeController {
  private center: string | null = null
  private savedScroll = 0

  constructor(
    private readonly setCenter: (id: string | null) => void,
    private readonly editor: () => MarkdownEditor | null,
    private readonly scroller: () => HTMLElement | null,
  ) {}

  current(): string | null {
    return this.center
  }

  showCenter(id: string): void {
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
    if (this.center !== null) {
      this.center = null
      flushSync(() => this.setCenter(null))
      const scroller = this.scroller()
      if (scroller) scroller.scrollTop = this.savedScroll
    }
    if (mode) this.editor()?.setMode(mode)
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
}

export function App({
  host,
  plugins = [],
}: {
  host: HostAdapter
  /** Extra plugins registered after the built-ins (test seam). */
  plugins?: AppPlugin[]
}): React.ReactElement {
  const editorRoot = useRef<HTMLDivElement>(null)
  const editorScroll = useRef<HTMLDivElement>(null)
  const editorRef = useRef<MarkdownEditor | null>(null)
  const [docState, setDocState] = useState<DocState | null>(null)
  const [version, setVersion] = useState(0)
  const [editorMode, setEditorMode] = useState<EditorMode>('wysiwyg')
  const [centerViewId, setCenterViewId] = useState<string | null>(null)
  const [ready, setReady] = useState(false)
  const [, refreshPlugins] = useReducer((value: number) => value + 1, 0)
  const restored = useRef(false)

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
        () => editorRef.current,
        () => editorScroll.current,
      ),
  )

  const centerViews = useViews(services.views, 'center')
  const centerView = centerViews.find((view) => view.id === centerViewId) ?? null
  const viewMode: ViewMode = centerView ? centerView.id : editorMode === 'source' ? 'markdown' : 'richtext'

  useEffect(() => {
    if (centerViewId !== null && centerView === null) modes.dropCenter()
  }, [centerViewId, centerView, modes])

  useEffect(() => {
    let disposed = false
    const cleanups: Disposable[] = []
    let debounceTimer: number | undefined

    const handleModeShortcut = (event: KeyboardEvent) => {
      if (event.repeat) return
      if (!(event.metaKey || event.ctrlKey) || event.altKey || event.key !== '/') return
      if (!editorRef.current) return
      event.preventDefault()
      event.stopPropagation()
      modes.toggleEditorMode()
    }

    window.addEventListener('keydown', handleModeShortcut, true)
    cleanups.push({
      dispose() {
        window.removeEventListener('keydown', handleModeShortcut, true)
      },
    })

    const applyDocumentState = (state: DocState) => {
      if (disposed) return
      setDocState(state)
      editorRef.current?.setReadonly(state.readOnly)
    }

    void (async () => {
      const snapshot = await host.load()
      if (disposed) return

      services.documents.setSnapshot(snapshot.text, snapshot.version)
      setDocState(snapshot.state)
      setVersion(snapshot.version)

      const root = editorRoot.current
      if (!root) return
      const editor = await createMarkdownEditor(root, { initialText: snapshot.text })
      if (disposed) {
        await editor.destroy()
        return
      }
      editorRef.current = editor
      services.editor.attach(editor)
      editor.setReadonly(snapshot.state.readOnly)

      cleanups.push(
        services.commands.register('editor.reveal', (offset) => {
          const value = Number(offset)
          if (!Number.isFinite(value)) return false
          // Navigating to the source from a center view lands in rich text.
          if (modes.current() !== null) modes.showEditor('wysiwyg')
          return editor.reveal(value)
        }),
      )

      cleanups.push(
        services.commands.register('editor.toggleSourceMode', () => {
          modes.toggleEditorMode()
          return editor.getMode()
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

      cleanups.push(
        host.onState((state) => {
          applyDocumentState(state)
        }),
      )

      cleanups.push(
        host.onReload((reload) => {
          if (debounceTimer !== undefined) {
            window.clearTimeout(debounceTimer)
            debounceTimer = undefined
          }
          editor.setMarkdown(reload.text)
          editor.setReadonly(reload.state.readOnly)
          services.documents.setSnapshot(reload.text, reload.version)
          setVersion(reload.version)
          applyDocumentState(reload.state)
        }),
      )

      const appPlugins = [createGenUIPlugin(), createOutlinePlugin(), ...plugins]
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
    })()

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
    const saved = rememberedViewMode(docState.path)
    if (saved === 'markdown') editorRef.current?.setMode('source')
    else if (saved && services.views.getAll().some((v) => v.location === 'center' && v.id === saved)) {
      modes.showCenter(saved)
    }
  }, [ready, docState, services, modes])

  useEffect(() => {
    if (restored.current && docState) rememberViewMode(docState.path, viewMode)
  }, [viewMode, docState])

  const pluginEnabled = (id: string) =>
    services.plugins.list().find((p) => p.id === id)?.enabled ?? false

  const outlineEnabled = pluginEnabled('outline')

  const togglePlugin = (id: string) => {
    if (pluginEnabled(id)) services.plugins.disable(id)
    else void services.plugins.enable(id).then(refreshPlugins)
    refreshPlugins()
  }

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

  return (
    <div className="app">
      <header className="app-header">
        <div className="doc-title">
          <span className="doc-name">{docState?.name ?? 'Loading…'}</span>
          {docState?.edited ? <span className="edited-dot" title="Unsaved edits" /> : null}
        </div>
        <div className="header-actions">
          <span className="version-chip">v{version}</span>
          <div className="view-modes" role="radiogroup" aria-label="View mode">
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
            className="toggle-button"
            data-testid="outline-toggle"
            aria-pressed={outlineEnabled}
            onClick={() => togglePlugin('outline')}
            disabled={!ready}
          >
            Outline: {outlineEnabled ? 'On' : 'Off'}
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

      <div className="app-body">
        <SidePane views={services.views} />
        <main className="editor-pane" hidden={centerView !== null}>
          <div className="editor-scroll" ref={editorScroll}>
            <div className="crepe-root" ref={editorRoot} data-testid="editor-root" />
          </div>
        </main>
        {centerView ? (
          <section
            className="center-pane"
            data-testid="center-pane"
            data-view-id={centerView.id}
            aria-label={centerView.title}
          >
            <ViewHost key={centerView.id} view={centerView} className="center-view" />
          </section>
        ) : null}
      </div>

      <footer className="status-bar">
        <span className="status-name">{docState?.path ?? '—'}</span>
        <span className="status-state">
          {docState?.edited ? 'Edited' : 'Saved'}
          {docState?.conflict ? ' · conflict' : ''}
        </span>
      </footer>
    </div>
  )
}
