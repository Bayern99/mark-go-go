import { useEffect, useMemo, useReducer, useRef, useState } from 'react'
import { ViewsPanel } from './components/ViewsPanel'
import { CommandRegistry } from './core/CommandRegistry'
import { DocumentService } from './core/DocumentService'
import { PluginRegistry } from './core/PluginRegistry'
import { ViewRegistry } from './core/ViewRegistry'
import type { Disposable, DocState, HostAdapter } from './host/types'
import { createMarkdownEditor, type MarkdownEditor } from './editor/createEditor'
import { createGenUIPlugin } from './plugins/genui'

const EDIT_DEBOUNCE_MS = 250

export function App({ host }: { host: HostAdapter }): React.ReactElement {
  const editorRoot = useRef<HTMLDivElement>(null)
  const editorRef = useRef<MarkdownEditor | null>(null)
  const [docState, setDocState] = useState<DocState | null>(null)
  const [version, setVersion] = useState(0)
  const [ready, setReady] = useState(false)
  const [, refreshPlugins] = useReducer((value: number) => value + 1, 0)

  const services = useMemo(() => {
    const documents = new DocumentService()
    const commands = new CommandRegistry()
    const views = new ViewRegistry()
    const plugins = new PluginRegistry(documents, views, commands)
    return { documents, commands, views, plugins }
  }, [])

  useEffect(() => {
    let disposed = false
    const cleanups: Disposable[] = []
    let debounceTimer: number | undefined

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
      editor.setReadonly(snapshot.state.readOnly)

      cleanups.push(
        services.commands.register('editor.revealHeading', (heading) =>
          editor.revealHeading(String(heading ?? '')),
        ),
      )

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

      services.plugins.register(createGenUIPlugin())
      await services.plugins.enable('genui')
      if (disposed) return
      refreshPlugins()
      setReady(true)
    })()

    return () => {
      disposed = true
      if (debounceTimer !== undefined) window.clearTimeout(debounceTimer)
      for (const cleanup of cleanups) cleanup.dispose()
      services.plugins.disable('genui')
      void editorRef.current?.destroy()
      editorRef.current = null
    }
  }, [host, services])

  const genuiEnabled = services.plugins.list().find((p) => p.id === 'genui')?.enabled ?? false

  const toggleGenUI = () => {
    if (genuiEnabled) services.plugins.disable('genui')
    else void services.plugins.enable('genui').then(refreshPlugins)
    refreshPlugins()
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
          <button
            type="button"
            className="toggle-button"
            data-testid="genui-toggle"
            aria-pressed={genuiEnabled}
            onClick={toggleGenUI}
            disabled={!ready}
          >
            GenUI: {genuiEnabled ? 'On' : 'Off'}
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
        <main className="editor-pane">
          <div className="editor-scroll">
            <div className="crepe-root" ref={editorRoot} data-testid="editor-root" />
          </div>
        </main>
        <aside className="side-pane" data-testid="side-pane">
          <ViewsPanel views={services.views} />
        </aside>
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
