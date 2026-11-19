import { useEffect, useLayoutEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react'
import { createRoot } from 'react-dom/client'
import type { AppPlugin, PluginContext } from '../../core'
import { ResearchHost, type ResearchArtifact } from '../../host/generated/mygo'
import { ScrollArea } from '../../components/ScrollArea'
import { RightPane } from '../../components/RightPane'
import { ViewRegistry } from '../../core/ViewRegistry'
import { DocumentDiff } from '../DocumentDiff'
import { ResearchSession } from './session'

function ResearchPanel({ session, ctx, documentID, open }: {
  session: ResearchSession
  ctx?: PluginContext
  documentID: string
  open: (path: string) => Promise<unknown>
}) {
  useSyncExternalStore(session.subscribe, session.snapshot)
  const [edit, setEdit] = useState(false)
  useEffect(() => { void session.connect() }, [session])
  const [doc, setDoc] = useState(() => ctx?.documents.snapshot())
  useEffect(() => {
    setDoc(ctx?.documents.snapshot())
    const sub = ctx?.documents.onDidChange(setDoc)
    return () => sub?.dispose()
  }, [ctx])
  const [transcript, setTranscript] = useState<HTMLElement | null>(null)
  const follow = useRef(true)
  const lastMessage = session.messages.at(-1)?.text
  useLayoutEffect(() => {
    if (transcript && follow.current) transcript.scrollTop = transcript.scrollHeight
  }, [transcript, session.messages.length, lastMessage])
  const { status, permission, proposal } = session
  const sameTarget = proposal && doc?.path === proposal.base.path
  const stale = sameTarget && (proposal.documentID !== documentID || doc.version !== proposal.base.version || doc.text !== proposal.base.text)
  const busy = status === 'working'
  const setDraft = (value: string) => { session.draft = value; session.changed() }
  const report = (error: unknown) => { session.error = String(error); session.changed() }

  async function review(file: ResearchArtifact) {
    if (!file.proposal || !ctx || doc?.path !== file.target) {
      await open(file.target)
      return
    }
    const base = ctx.documents.snapshot()
    const text = await ResearchHost.readArtifact(session.root, file.path)
    session.proposal = { documentID, base, text }
    session.changed()
  }
  async function apply() {
    if (!ctx || !proposal || proposal.documentID !== documentID) return
    await ctx.commands.execute('editor.applyMarkdown', proposal.base, proposal.text)
    session.proposal = undefined
    session.changed()
  }
  if (!session.root) return <p className="nav-empty">Open a folder to start research.</p>
  return <section className="plugin-panel" aria-label="Feynman research">
    {session.messages.length > 0 && <ScrollArea className="research-transcript" contentClassName="research-conversation"
      aria-label="Conversation" edgeFade onViewport={setTranscript} onViewportScroll={() => {
        if (transcript) follow.current = transcript.scrollHeight - transcript.clientHeight - transcript.scrollTop < 12
      }}>
      {session.messages.map((message, i) => <div className="research-message" key={i}>
        <strong>{message.role === 'user' ? 'You' : message.role === 'notice' ? 'Session' : 'Feynman'}</strong>
        <p>{message.text}</p>
      </div>)}
    </ScrollArea>}
    {session.error && <p role="alert">{session.error}</p>}
    {permission && <div role="group" aria-label="Feynman confirmation">
      <p>{permission.text || 'Feynman requests confirmation.'}</p>
      <div className="plugin-actions">{permission.options?.map(option =>
        <button key={option.id} className="pill" onClick={() => void ResearchHost.answer(session.root, permission.requestId!, option.id).catch(report)}>{option.name}</button>
      )}</div>
    </div>}
    <form onSubmit={event => { event.preventDefault(); void session.send(documentID, edit ? ctx?.documents.snapshot() : undefined) }}>
      {session.commands.length > 0 && <select aria-label="Research command" value="" disabled={busy} onChange={e => {
        setDraft('/' + e.target.value.replace(/^\//, '') + ' ')
        e.target.form?.querySelector('textarea')?.focus()
      }}>
        <option value="" disabled>Commands…</option>
        {session.commands.map(command => <option key={command} value={command}>/{command.replace(/^\//, '')}</option>)}
      </select>}
      <textarea rows={3} aria-label="Research prompt" placeholder="Ask Feynman…" value={session.draft}
        onChange={e => setDraft(e.target.value)} onKeyDown={e => {
          if ((e.metaKey || e.ctrlKey) && e.key === 'Enter' && !e.nativeEvent.isComposing) e.currentTarget.form?.requestSubmit()
        }} />
      {doc?.path && <label><input type="checkbox" checked={edit} disabled={busy || !!proposal}
        onChange={e => setEdit(e.target.checked)} /> Edit this document</label>}
      <div className="plugin-actions">
        {status === 'offline' ? <button type="button" className="pill primary" onClick={() => void session.connect()}>Reconnect</button>
          : busy ? <button type="button" className="pill" onClick={() => void ResearchHost.cancel(session.root).catch(report)}>Cancel</button>
          : <button className="pill primary" disabled={status !== 'ready' || !session.draft.trim() || (edit && !!proposal)}>Send</button>}
        <span role="status">{busy ? session.progress || 'Researching…' : status === 'connecting' ? 'Connecting…' : ''}</span>
      </div>
    </form>
    {proposal && <section aria-label="Proposed document edit">
      {sameTarget ? <>
        <DocumentDiff before={proposal.base.text} after={proposal.text} />
        {stale ? <>
          <p>The document changed since this comparison.</p>
          <button className="pill" onClick={() => {
            if (ctx) session.proposal = { ...proposal, documentID, base: ctx.documents.snapshot() }
            session.changed()
          }}>Compare current text</button>
        </> : <button className="pill" onClick={() => void apply().catch(report)}>Apply in Markdown</button>}
      </> : <button className="pill" onClick={() => void open(proposal.base.path).catch(report)}>Review {proposal.base.path.split('/').pop()}</button>}
      <button className="pill quiet" onClick={() => { session.proposal = undefined; setEdit(false); session.changed() }}>Dismiss</button>
    </section>}
    {session.files.length > 0 && <section aria-label="Research files"><h3>Files</h3>
      {session.files.map(file => <div key={file.path}>
        {file.error ? <p role="alert">{file.path} · {file.error}</p> :
          <button className="research-file" disabled={busy && file.proposal} onClick={() => void review(file).catch(report)}>
            {file.proposal ? 'Review ' : ''}{file.path}
          </button>}
      </div>)}
    </section>}
  </section>
}

function researchView(session: ResearchSession, documentID: string, open: (path: string) => Promise<unknown>, ctx?: PluginContext) {
  return {
    id: 'feynman.panel', title: 'Feynman', location: 'right' as const, workspaceScoped: true,
    mount(el: HTMLElement) {
      const view = createRoot(el)
      view.render(<ResearchPanel ctx={ctx} session={session} documentID={documentID} open={open} />)
      return { dispose() { queueMicrotask(() => view.unmount()) } }
    },
  }
}

export function EmptyResearchPane({ root, open }: { root: string; open: (path: string) => Promise<unknown> }) {
  const views = useMemo(() => new ViewRegistry(), [])
  useEffect(() => {
    const session = ResearchSession.acquire(root)
    const view = views.register(researchView(session, '', open))
    return () => { view.dispose(); session.release() }
  }, [views, root, open])
  return <RightPane views={views} />
}

export function createFeynmanPlugin(root: string, documentID: string, open: (path: string) => Promise<unknown>): AppPlugin {
  return {
    id: 'feynman', title: 'Feynman',
    activate(ctx) {
      const session = ResearchSession.acquire(root)
      ctx.subscriptions.push({ dispose: () => session.release() })
      ctx.views.register(researchView(session, documentID, open, ctx))
    },
  }
}
