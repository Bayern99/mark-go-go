import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { CommandRegistry } from '../core/CommandRegistry'
import { DocumentService } from '../core/DocumentService'
import { EditorAccess } from '../core/EditorAccess'
import { PluginRegistry } from '../core/PluginRegistry'
import { ViewRegistry } from '../core/ViewRegistry'
import type { AppPlugin, WorkspaceContext } from '../core/types'
import type { RecentWorkspace, ViewFileSource, ViewGenerator, WorkspaceAdapter, WorkspaceSnapshot } from '../host/types'
import { WorkspacePlugins } from '../plugins/DesktopPlugins'
import { createDocumentServices, DocumentPane, type DocumentServices } from './DocumentPane'
import { DocumentTabs } from './DocumentTabs'
import { RightPane } from './RightPane'
import { SidePane } from './SidePane'
import { useViews } from './ViewHost'
import { Welcome } from './Welcome'
import {
  readSavedRightWidth,
  resolvePaneWidths,
  SIDE_DOCKED_MIN_WIDTH,
  SIDEBAR_DEFAULT,
} from './paneLayout'

export const NARROW_WINDOW_WIDTH = 1180

const noopViewFile: ViewFileSource & ViewGenerator = {
  watchView: () => ({ dispose() {} }),
  undoView: async () => {},
  generate: async () => {},
  getSettings: async () => ({ model: '', agentCommand: '' }),
  setSettings: async (p) => ({ model: p.model ?? '', agentCommand: p.agentCommand ?? '' }),
  onSettings: () => ({ dispose() {} }),
}

function measureWindowWidth(): number {
  if (typeof window === 'undefined') return NARROW_WINDOW_WIDTH
  return document.documentElement.clientWidth || window.innerWidth
}

function measureIsNarrow(): boolean {
  return measureWindowWidth() < NARROW_WINDOW_WIDTH
}

export function WorkspaceShell({
  workspace,
  plugins = [],
}: {
  workspace: WorkspaceAdapter
  plugins?: AppPlugin[]
}): React.ReactElement {
  const [snap, setSnap] = useState<WorkspaceSnapshot | null>(null)
  const panes = useRef(new Map<string, DocumentServices>())
  const [recents, setRecents] = useState<RecentWorkspace[]>([])
  const reveals = useRef(new Map<string, (offset: number) => boolean>())
  const emptyViews = useMemo(() => new ViewRegistry(), [])
  const workspaceListeners = useRef(new Set<() => void>())

  const documents = snap?.documents ?? []
  const activeId = snap?.activeId ?? documents[0]?.id ?? ''
  const snapRef = useRef(snap)
  const activeIdRef = useRef(activeId)
  snapRef.current = snap
  activeIdRef.current = activeId

  const openDocument = useCallback(
    async (path: string, existingID?: string) => {
      let id: string
      if (existingID && (await workspace.snapshot()).documents.some((doc) => doc.id === existingID)) {
        await workspace.activate(existingID)
        id = existingID
      } else {
        id = (await workspace.open(path)).id
      }
      return {
        id,
        async snapshot() {
          const buffer = panes.current.get(id)?.documents.snapshot()
          if (buffer?.version) return buffer
          const { state, text, version } = await workspace.document(id).load()
          return { path: state.path, text, version }
        },
      }
    },
    [workspace],
  )

  const workspaceBundle = useMemo(() => {
    const wsViews = new ViewRegistry()
    const wsCommands = new CommandRegistry()
    const wsDocs = new DocumentService()
    const wsEditor = new EditorAccess()
    const ctx: WorkspaceContext = {
      root: () => snapRef.current?.root ?? '',
      workspaces: () => snapRef.current?.workspaces ?? [],
      activeDocumentId: () => activeIdRef.current,
      activeDocument: () => {
        const pane = panes.current.get(activeIdRef.current)
        if (!pane) return null
        return {
          snapshot: () => pane.documents.snapshot(),
          onDidChange: (fn) => pane.documents.onDidChange(fn),
          applyMarkdown: async (base, next) => {
            await pane.commands.execute('editor.applyMarkdown', base, next)
          },
        }
      },
      openDocument,
      openFolder: () => {
        void workspace.pickFolder()
      },
      onDidChange: (fn) => {
        workspaceListeners.current.add(fn)
        return {
          dispose() {
            workspaceListeners.current.delete(fn)
          },
        }
      },
    }
    const wsPlugins = new PluginRegistry(wsDocs, wsViews, wsCommands, wsEditor, noopViewFile, {
      scope: 'workspace',
      workspace: ctx,
    })
    return { views: wsViews, plugins: wsPlugins }
  }, [workspace, openDocument])

  const documentPlugins = useMemo(
    () => plugins.filter((plugin) => (plugin.scope ?? 'document') === 'document'),
    [plugins],
  )
  const externalWorkspacePlugins = useMemo(
    () => plugins.filter((plugin) => plugin.scope === 'workspace'),
    [plugins],
  )

  useEffect(() => {
    const disposables = externalWorkspacePlugins.map((plugin) => {
      const reg = workspaceBundle.plugins.register(plugin)
      void workspaceBundle.plugins.enable(plugin.id)
      return reg
    })
    return () => {
      for (const d of disposables) d.dispose()
    }
  }, [workspaceBundle, externalWorkspacePlugins])

  useLayoutEffect(() => {
    for (const fn of workspaceListeners.current) fn()
  }, [snap, activeId])

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

  const rightViews = useViews(workspaceBundle.views, 'right')
  const primaryRightView = rightViews[0] ?? null
  const rightAvailable = primaryRightView !== null

  const [libraryOpen, setLibraryOpen] = useState(false)
  const libraryToggleRef = useRef<HTMLButtonElement>(null)
  const rightToggleRef = useRef<HTMLButtonElement>(null)
  const [winWidth, setWinWidth] = useState(measureWindowWidth)
  const isNarrow = winWidth < NARROW_WINDOW_WIDTH
  const [targetLeftWidth, setTargetLeftWidth] = useState(SIDEBAR_DEFAULT)
  const [targetRightWidth, setTargetRightWidth] = useState(readSavedRightWidth)
  const [rightWantOpen, setRightWantOpen] = useState(true)
  const [rightFloatOpen, setRightFloatOpen] = useState(false)
  const rightShown = rightAvailable && (isNarrow ? rightFloatOpen : rightWantOpen)
  const rightFloating = rightShown && isNarrow
  const leftDocked = winWidth >= SIDE_DOCKED_MIN_WIDTH
  const rightDocked = Boolean(rightAvailable && snap && rightShown && !rightFloating)
  const paneLayout = resolvePaneWidths({
    winWidth,
    targetLeft: targetLeftWidth,
    targetRight: targetRightWidth,
    leftDocked,
    rightDocked,
  })

  const closeDrawers = useCallback(() => {
    setLibraryOpen(false)
    setRightFloatOpen(false)
  }, [])

  const toggleRightPane = useCallback(() => {
    if (measureIsNarrow()) {
      setLibraryOpen(false)
      rightToggleRef.current?.focus({ preventScroll: true })
      setRightFloatOpen((value) => !value)
    } else {
      setRightWantOpen((value) => !value)
    }
  }, [])

  useEffect(() => {
    const resize = () => {
      const nextWidth = measureWindowWidth()
      setWinWidth(nextWidth)
      if (nextWidth >= NARROW_WINDOW_WIDTH) setRightFloatOpen(false)
    }
    const observer = new ResizeObserver(resize)
    observer.observe(document.documentElement)
    window.addEventListener('resize', resize)
    return () => {
      observer.disconnect()
      window.removeEventListener('resize', resize)
    }
  }, [])

  useEffect(() => {
    if (!rightAvailable) {
      setRightFloatOpen(false)
      setRightWantOpen(true)
      return
    }
    const keydown = (event: KeyboardEvent) => {
      if (event.repeat || event.defaultPrevented) return
      if ((event.metaKey || event.ctrlKey) && event.altKey && event.code === 'Digit0') {
        event.preventDefault()
        toggleRightPane()
      } else if (
        rightFloating &&
        event.key === 'Escape' &&
        !document.querySelector('[data-radix-popper-content-wrapper], [role="dialog"][data-state="open"]') &&
        !(
          event.target instanceof Element &&
          event.target.closest('input, textarea, select, [contenteditable="true"], dialog')
        )
      ) {
        event.preventDefault()
        setRightFloatOpen(false)
        rightToggleRef.current?.focus()
      }
    }
    window.addEventListener('keydown', keydown)
    return () => window.removeEventListener('keydown', keydown)
  }, [rightAvailable, rightFloating, toggleRightPane])

  const recentDocuments = useRef<string[]>([])
  useEffect(() => {
    const live = new Set(documents.map((doc) => doc.id))
    recentDocuments.current = [
      ...(activeId ? [activeId] : []),
      ...recentDocuments.current.filter((id) => id !== activeId && live.has(id)),
    ]
  }, [activeId, documents])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (!event.ctrlKey || event.metaKey || event.altKey || event.key !== 'Tab') return
      const previous = recentDocuments.current.find(
        (id) => id !== activeId && documents.some((doc) => doc.id === id),
      )
      if (!previous) return
      event.preventDefault()
      event.stopPropagation()
      void workspace.activate(previous)
    }
    window.addEventListener('keydown', onKeyDown, true)
    return () => window.removeEventListener('keydown', onKeyDown, true)
  }, [activeId, documents, workspace])

  useEffect(() => {
    setLibraryOpen(false)
  }, [activeId])

  useEffect(() => {
    if (!libraryOpen) return
    const toggle = libraryToggleRef.current
    if (!toggle) return
    const observer = new ResizeObserver(() => {
      if (!toggle.getClientRects().length) setLibraryOpen(false)
    })
    observer.observe(toggle)
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || event.defaultPrevented || event.repeat) return
      event.preventDefault()
      setLibraryOpen(false)
      libraryToggleRef.current?.focus()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => {
      observer.disconnect()
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [libraryOpen])

  for (const doc of documents) {
    if (!panes.current.has(doc.id)) panes.current.set(doc.id, createDocumentServices(workspace.document(doc.id)))
  }
  for (const id of [...panes.current.keys()]) {
    if (!documents.some((doc) => doc.id === id)) panes.current.delete(id)
  }

  const activePane = panes.current.get(activeId)

  const openPath = (path: string, isFile: boolean) => {
    if (isFile) {
      setLibraryOpen(false)
      void workspace.open(path)
    } else {
      void workspace.revealPath(path)
    }
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

  return (
    <div className="workspace-shell">
      <WorkspacePlugins registry={workspaceBundle.plugins} />
      {snap?.restoreError ? (
        <div className="banner" role="alert" data-testid="restore-error">
          {snap.restoreError}
        </div>
      ) : null}
      <div className={`workspace-body${libraryOpen ? ' library-open' : ''}${rightFloating ? ' right-floating' : ''}`}>
        {libraryOpen ? (
          <div
            className="drawer-scrim library-scrim"
            aria-hidden="true"
            onClick={() => {
              setLibraryOpen(false)
              libraryToggleRef.current?.focus()
            }}
          />
        ) : null}
        {rightFloating ? (
          <div
            className="drawer-scrim right-scrim"
            data-testid="right-scrim"
            aria-hidden="true"
            onClick={() => {
              setRightFloatOpen(false)
              rightToggleRef.current?.focus()
            }}
          />
        ) : null}
        <SidePane
          views={activePane?.views ?? emptyViews}
          workspace={workspace}
          workspaceSnap={snap}
          doc={documents.find((d) => d.id === activeId) ?? null}
          onJumpHeading={(documentId, path, offset) => {
            setLibraryOpen(false)
            jumpHeading(documentId, path, offset)
          }}
          floatingOpen={libraryOpen}
          onCloseFloating={() => {
            setLibraryOpen(false)
            libraryToggleRef.current?.focus()
          }}
          width={leftDocked ? paneLayout.left : SIDEBAR_DEFAULT}
          maxWidth={paneLayout.maxLeft}
          onWidthChange={setTargetLeftWidth}
        />
        <div
          className="workspace-main"
          onClick={() => {
            if (libraryOpen) setLibraryOpen(false)
          }}
        >
          <DocumentTabs
            documents={documents}
            activeId={activeId}
            recentOrder={recentDocuments.current}
            libraryOpen={libraryOpen}
            libraryToggleRef={libraryToggleRef}
            onToggleLibrary={() => {
              setRightFloatOpen(false)
              setLibraryOpen((open) => !open)
            }}
            onActivate={(id) => void workspace.activate(id)}
            onCloseTab={(id) => void workspace.closeDocument(id)}
            rightView={primaryRightView}
            rightShown={rightShown}
            rightToggleRef={rightToggleRef}
            onToggleRight={toggleRightPane}
          />
          {documents.map((doc) => (
            <DocumentPane
              key={doc.id}
              services={panes.current.get(doc.id)!}
              plugins={documentPlugins}
              active={doc.id === activeId}
              workspaceRoot={doc.workspaceRoot ?? (doc.id === activeId ? snap?.root : '') ?? ''}
              drawerOpen={libraryOpen || rightFloating}
              closeDrawers={closeDrawers}
              onOpenPath={openPath}
              onReadyReveal={(id, reveal) => {
                reveals.current.set(id, reveal)
                return () => {
                  if (reveals.current.get(id) === reveal) reveals.current.delete(id)
                }
              }}
            />
          ))}
          {activeId === '' ? (
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
        {rightAvailable && snap && (
          <RightPane
            views={workspaceBundle.views}
            open={rightShown}
            floating={rightFloating}
            width={paneLayout.right}
            targetWidth={targetRightWidth}
            maxWidth={paneLayout.maxRight}
            onWidthChange={setTargetRightWidth}
            onClose={() => {
              setRightFloatOpen(false)
              rightToggleRef.current?.focus()
            }}
          />
        )}
      </div>
    </div>
  )
}
