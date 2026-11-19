import { lazy, Suspense, useEffect, useReducer, useState } from 'react'
import { createRoot } from 'react-dom/client'
import type { AppPlugin, DocumentSnapshot, PluginContext, WorkspaceContext } from '../../core'
import { ResearchSession } from './session'

const ResearchPanel = lazy(() => import('./ResearchPanel'))

export type OpenResearchDocument = (path: string, existingID?: string) => Promise<{
  id: string
  snapshot: () => Promise<DocumentSnapshot>
}>

export interface FeynmanSettingsSlotProps {
  root: string
  onClose: () => void
}

export interface FeynmanPluginOptions {
  renderSettings?: (props: FeynmanSettingsSlotProps) => React.ReactNode
}

export function WorkspaceResearch({
  root,
  ctx,
  documentID,
  open,
  openFolder,
  renderSettings,
}: {
  root: string
  ctx?: {
    documents: Pick<PluginContext['documents'], 'snapshot' | 'onDidChange'>
    commands: Pick<PluginContext['commands'], 'execute'>
  }
  documentID: string
  open: OpenResearchDocument
  openFolder: () => void
  renderSettings?: (props: FeynmanSettingsSlotProps) => React.ReactNode
}) {
  const [session, setSession] = useState<ResearchSession>()
  useEffect(() => {
    if (!root) {
      setSession(undefined)
      return
    }
    const current = ResearchSession.acquire(root)
    setSession(current)
    return () => current.release()
  }, [root])
  if (!root) {
    return (
      <div className="research-empty">
        <h2>Feynman</h2>
        <p>Open a folder to start research.</p>
        <button type="button" className="pill" onClick={openFolder}>
          Open folder…
        </button>
      </div>
    )
  }
  const loading = (
    <p className="nav-empty" role="status">
      Opening research…
    </p>
  )
  return session ? (
    <Suspense fallback={loading}>
      <ResearchPanel
        session={session}
        ctx={ctx}
        documentID={documentID}
        open={open}
        renderSettings={renderSettings}
      />
    </Suspense>
  ) : (
    loading
  )
}

function FeynmanWorkspaceView({
  workspace,
  renderSettings,
}: {
  workspace: WorkspaceContext
  renderSettings?: (props: FeynmanSettingsSlotProps) => React.ReactNode
}) {
  const [, refresh] = useReducer((n: number) => n + 1, 0)
  useEffect(() => {
    const sub = workspace.onDidChange(refresh)
    return () => sub.dispose()
  }, [workspace])

  const root = workspace.root()
  const documentID = workspace.activeDocumentId()
  const activeDoc = workspace.activeDocument()
  const docCtx = activeDoc
    ? {
        documents: {
          snapshot: () => activeDoc.snapshot(),
          onDidChange: (fn: (s: DocumentSnapshot) => void) => activeDoc.onDidChange(fn),
        },
        commands: {
          execute: async (id: string, ...args: unknown[]) => {
            if (id === 'editor.applyMarkdown') {
              await activeDoc.applyMarkdown(args[0] as DocumentSnapshot, args[1] as string)
              return true
            }
            return false
          },
        },
      }
    : undefined

  return (
    <WorkspaceResearch
      key={root || '__empty__'}
      root={root}
      ctx={docCtx}
      documentID={documentID}
      open={workspace.openDocument}
      openFolder={() => void workspace.openFolder()}
      renderSettings={renderSettings}
    />
  )
}

const ACTION_REQUIRED_DOT = { tone: 'warn', title: 'Action required' } as const

export function createFeynmanPlugin(options?: FeynmanPluginOptions): AppPlugin {
  return {
    id: 'feynman',
    title: 'Feynman',
    scope: 'workspace',
    activate(ctx) {
      const workspace = ctx.workspace
      if (!workspace) {
        throw new Error('Feynman plugin requires workspace scope')
      }

      const held = new Map<string, ResearchSession>()
      const syncWorkspaceSessions = () => {
        const wanted = new Set(workspace.workspaces().map((w) => w.root).filter(Boolean))
        for (const root of wanted) {
          if (!held.has(root)) held.set(root, ResearchSession.acquire(root))
        }
        for (const [root, session] of held) {
          if (!wanted.has(root)) {
            session.release()
            held.delete(root)
          }
        }
      }
      syncWorkspaceSessions()
      ctx.workspace?.onDidChange(syncWorkspaceSessions)
      ctx.subscriptions.push({
        dispose() {
          for (const session of held.values()) session.release()
          held.clear()
        },
      })

      const subscribeActiveSession = (fn: () => void) => {
        let currentRoot = workspace.root()
        let currentSession = currentRoot ? held.get(currentRoot) : undefined
        let offSession = currentSession?.subscribe(fn)
        const offWs = workspace.onDidChange(() => {
          const nextRoot = workspace.root()
          if (nextRoot !== currentRoot) {
            offSession?.()
            currentRoot = nextRoot
            currentSession = currentRoot ? held.get(currentRoot) : undefined
            offSession = currentSession?.subscribe(fn)
          }
          fn()
        })
        return () => {
          offSession?.()
          offWs.dispose()
        }
      }

      ctx.views.register({
        id: 'feynman.panel',
        title: 'Feynman',
        location: 'right',
        badge: {
          subscribe: subscribeActiveSession,
          getSnapshot: () => {
            const root = workspace.root()
            return root ? (held.get(root)?.proposals.length ?? 0) : 0
          },
        },
        status: {
          subscribe: subscribeActiveSession,
          getSnapshot: () => {
            const root = workspace.root()
            const session = root ? held.get(root) : undefined
            return session?.permission ? ACTION_REQUIRED_DOT : null
          },
        },
        mount(el) {
          const reactRoot = createRoot(el)
          reactRoot.render(
            <FeynmanWorkspaceView workspace={workspace} renderSettings={options?.renderSettings} />,
          )
          return {
            dispose() {
              queueMicrotask(() => reactRoot.unmount())
            },
          }
        },
      })
    },
  }
}
