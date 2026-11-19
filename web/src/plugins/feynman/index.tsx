import { lazy, Suspense, useEffect, useState } from 'react'
import type { DocumentSnapshot, PluginContext } from '../../core'
import { ResearchSession } from './session'

const ResearchPanel = lazy(() => import('./ResearchPanel'))

export type OpenResearchDocument = (path: string, existingID?: string) => Promise<{
  id: string
  snapshot: () => Promise<DocumentSnapshot>
}>

export function WorkspaceResearch({ root, ctx, documentID, open, openFolder }: {
  root: string
  ctx?: Pick<PluginContext, 'documents' | 'commands'>
  documentID: string
  open: OpenResearchDocument
  openFolder: () => void
}) {
  const [session, setSession] = useState<ResearchSession>()
  useEffect(() => {
    if (!root) return
    const current = ResearchSession.acquire(root)
    setSession(current)
    return () => current.release()
  }, [root])
  if (!root) return <div className="research-empty"><h3>Feynman</h3><p>Open a folder to start research.</p>
    <button type="button" className="pill" onClick={openFolder}>Open folder…</button></div>
  const loading = <p className="nav-empty" role="status">Opening research…</p>
  return session ? <Suspense fallback={loading}><ResearchPanel session={session} ctx={ctx} documentID={documentID} open={open} /></Suspense> : loading
}
