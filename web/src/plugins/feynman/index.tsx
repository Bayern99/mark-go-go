import { lazy, Suspense, useEffect, useMemo } from 'react'
import { createRoot } from 'react-dom/client'
import type { AppPlugin, PluginContext } from '../../core'
import { RightPane } from '../../components/RightPane'
import { ViewRegistry } from '../../core/ViewRegistry'
import { ResearchSession } from './session'

const ResearchPanel = lazy(() => import('./ResearchPanel'))

function researchView(session: ResearchSession, documentID: string, open: (path: string, existingID?: string) => Promise<unknown>, ctx?: PluginContext) {
  return {
    id: 'feynman.panel', title: 'Feynman', location: 'right' as const, workspaceScoped: true,
    mount(el: HTMLElement) {
      const view = createRoot(el)
      view.render(<Suspense fallback={<p role="status">Opening research…</p>}><ResearchPanel ctx={ctx} session={session} documentID={documentID} open={open} /></Suspense>)
      return { dispose() { queueMicrotask(() => view.unmount()) } }
    },
  }
}

export function EmptyResearchPane({ root, open }: { root: string; open: (path: string, existingID?: string) => Promise<unknown> }) {
  const views = useMemo(() => new ViewRegistry(), [])
  useEffect(() => {
    const session = ResearchSession.acquire(root)
    const view = views.register(researchView(session, '', open))
    return () => { view.dispose(); session.release() }
  }, [views, root, open])
  return <RightPane views={views} />
}

export function createFeynmanPlugin(root: string, documentID: string, open: (path: string, existingID?: string) => Promise<unknown>): AppPlugin {
  return {
    id: 'feynman', title: 'Feynman',
    activate(ctx) {
      const session = ResearchSession.acquire(root)
      ctx.subscriptions.push({ dispose: () => session.release() })
      ctx.views.register(researchView(session, documentID, open, ctx))
    },
  }
}
