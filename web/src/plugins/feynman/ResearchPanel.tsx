import { lazy, Suspense, useEffect, useLayoutEffect, useReducer, useRef, useState, useSyncExternalStore } from 'react'
import type { PluginContext } from '../../core'
import { ResearchHost, type ResearchArtifact } from '../../host/generated/mygo'
import { ScrollArea } from '../../components/ScrollArea'
import { DocumentDiff } from '../DocumentDiff'
import { Conversation, ResearchIcon } from './Conversation'
import { Composer, useCommand } from './Composer'
import { SessionMenu } from './SessionMenu'
import { ResearchQuestion } from './ResearchQuestion'
import type { OpenResearchDocument } from './index'
import type { ResearchSession } from './session'
import './research.css'

const CLIConsole = lazy(() => import('./CLIConsole').then(m => ({ default: m.CLIConsole })))

export default function ResearchPanel({ session, ctx, documentID, open }: {
  session: ResearchSession
  ctx?: Pick<PluginContext, 'documents' | 'commands'>
  documentID: string
  open: OpenResearchDocument
}) {
  useSyncExternalStore(session.subscribe, session.snapshot)
  const [editTarget, setEditTarget] = useState('')
  const edit = !!documentID && editTarget === documentID
  const setEdit = (value: boolean) => setEditTarget(value ? documentID : '')
  const input = useRef<HTMLTextAreaElement>(null)
  const choose = useCommand(session, input)
  const { tab, cliRunning } = session
  useEffect(() => { if (tab === 'chat' && !cliRunning) void session.connect() }, [session, tab, cliRunning])
  const [, refreshDoc] = useReducer(n => n + 1, 0)
  useLayoutEffect(() => {
    const sub = ctx?.documents.onDidChange(refreshDoc)
    return () => sub?.dispose()
  }, [ctx])
  const doc = ctx?.documents.snapshot()
  const { status, permission, proposal } = session
  const sameTarget = proposal && doc && proposal.documentID === documentID
  const stale = sameTarget && (doc.path !== proposal.base.path ||
    doc.version !== proposal.base.version || doc.text !== proposal.base.text)
  const busy = status === 'working'
  const report = (error: unknown) => { session.error = String(error); session.changed() }
  const comparison = useRef<HTMLElement>(null)
  function showComparison() {
    session.tab = 'chat'
    session.changed()
    requestAnimationFrame(() => comparison.current?.scrollIntoView({ block: 'nearest' }))
  }
  async function showPending() {
    const pending = session.proposal
    if (!pending) return
    const target = await open(pending.base.path, pending.documentID)
    if (session.proposal === pending && target.id !== pending.documentID) {
      session.proposal = { ...pending, documentID: target.id }
    }
    showComparison()
  }
  async function review(file: ResearchArtifact) {
    if (file.missing) {
      const recovered = await ResearchHost.recoverArtifact(session.root, file.path)
      await session.refresh()
      await open(recovered)
      return
    }
    if (file.error || !file.proposal) {
      await open(file.target)
      return
    }
    if (session.proposal) return showPending()
    const target = await open(file.target)
    const [base, text] = await Promise.all([target.snapshot(), ResearchHost.readArtifact(session.root, file.path)])
    if (session.proposal) return showPending()
    if (session.status === 'working') throw new Error('Finish the current research action before reviewing this file.')
    if (base.path !== file.target) throw new Error('The document moved. Refresh research files before reviewing.')
    if (text === base.text) return
    session.proposal = { documentID: target.id, base, text }
    showComparison()
  }
  async function apply() {
    if (!ctx || !proposal || proposal.documentID !== documentID) return
    await ctx.commands.execute('editor.applyMarkdown', proposal.base, proposal.text)
    session.proposal = undefined
    session.changed()
    await session.refresh()
  }

  const label = cliRunning ? 'CLI running' : status === 'ready' ? 'Connected' : status === 'working' ? 'Working' : status === 'connecting' ? 'Connecting…' : 'Offline'
  const pane = (tab: ResearchSession['tab']) => { session.tab = tab; session.changed() }
  function fileButton(file: ResearchArtifact) {
    return <div className="fy-file-wrap" key={file.path}>
      {file.error && <p role="alert">{file.path} · {file.error}</p>}
      {(file.target || (file.missing && !file.error)) && <button type="button" className="research-file"
        title={file.proposal && proposal ? 'Finish the pending edit before reviewing this file.' : file.path}
        disabled={(busy && (file.proposal || file.missing)) || (file.proposal && !!proposal)} onClick={() => void review(file).catch(report)}>
        <ResearchIcon name={file.path.endsWith('.provenance.md') ? 'link' : file.proposal ? 'git-diff' : 'file-text'} />
        <span><b>{file.missing ? 'Restore ' : file.proposal ? 'Review ' : ''}{file.path.split('/').at(-1)}</b>
        {file.path.includes('/') && <small>{file.path.slice(0,file.path.lastIndexOf('/'))}</small>}</span>
        <ResearchIcon name="arrow-up-right" />
      </button>}
    </div>
  }
  const reviewSlot = <>
    {proposal && <section ref={comparison} aria-label="Proposed document edit">
      {sameTarget ? <>
        <DocumentDiff before={proposal.base.text} after={proposal.text} />
        {stale ? <>
          <p>The document changed since this comparison.</p>
          <button className="pill" onClick={() => {
            if (ctx) session.proposal = { ...proposal, documentID, base: ctx.documents.snapshot() }
            session.changed()
          }}>Compare current text</button>
        </> : <button className="pill" onClick={() => void apply().catch(report)}>Apply in Markdown</button>}
      </> : <button className="pill" onClick={() => void showPending().catch(report)}>Open document {proposal.base.path.split('/').pop()}</button>}
      {!sameTarget && <button className="pill quiet" onClick={() => void navigator.clipboard.writeText(proposal.text).catch(report)}>Copy proposed Markdown</button>}
      <button className="pill quiet" onClick={() => { session.proposal = undefined; setEdit(false); session.changed() }}>Dismiss</button>
    </section>}
    {session.files.length > 0 && <div className="fy-chat-files"><h4>Research files</h4>{session.files.slice(0, 3).map(fileButton)}
      {session.files.length > 3 && <button type="button" className="fy-link" onClick={() => pane('files')}>See all {session.files.length} files</button>}
    </div>}
  </>
  return <section className="plugin-panel fy-panel" aria-label="Feynman research">
    <header className="fy-heading"><ResearchIcon name="atom" /><div className="fy-heading-title"><h3>Feynman</h3>
      <small title={session.root}>{[session.root.split('/').filter(Boolean).at(-1), session.title].filter(Boolean).join(' · ')}</small></div>
      <span className="fy-connection" data-status={status} title={label} aria-label={label} /><SessionMenu session={session} />
    </header>
    <nav className="fy-tabs" aria-label="Research tools">{(['chat', 'files', 'cli'] as const).map(tab =>
      <button type="button" key={tab} aria-pressed={session.tab === tab} onClick={() => pane(tab)}>
        <ResearchIcon name={tab === 'chat' ? 'chat-circle' : tab === 'files' ? 'files' : 'terminal-window'} />
        {tab === 'chat' ? 'Chat' : tab === 'files' ? 'Files' : 'CLI'}{tab === 'chat' && permission && <small aria-label="Action required">!</small>}{tab === 'files' && session.files.length > 0 && <small>{session.files.length}</small>}
      </button>)}</nav>
    {session.error && <div className="fy-error" role="alert"><span>{session.error}</span><button className="fy-icon" aria-label="Dismiss error"
      onClick={() => { session.error = ''; session.changed() }}><ResearchIcon name="x" /></button></div>}
    {session.tab === 'cli' ? <Suspense fallback={<p role="status">Opening console…</p>}><CLIConsole session={session} /></Suspense>
      : session.tab === 'files' ? <ScrollArea className="fy-files" contentClassName="fy-files-content">
        <div className="fy-files-heading"><h4>Research files</h4><button className="fy-icon" type="button" aria-label="Refresh files" title="Refresh files"
          disabled={busy} onClick={() => void session.refresh()}><ResearchIcon name="arrow-clockwise" /></button>
          <button type="button" className="fy-icon" title="Open research output folder" aria-label="Open research output folder"
            onClick={() => void ResearchHost.revealOutputs(session.root).catch(report)}><ResearchIcon name="folder-open" /></button></div>
        {!session.files.length && <p className="fy-muted">Completed research files appear here.</p>}
        {proposal && <button type="button" className="pill" onClick={() => void showPending().catch(report)}>Review pending edit · {proposal.base.path.split('/').at(-1)}</button>}
        {session.files.map(fileButton)}
      </ScrollArea> : <>
        <Conversation session={session} choose={choose} open={review} reviewSlot={reviewSlot} />
        {permission && <ResearchQuestion session={session} key={permission.requestId} question={permission} onError={report} />}
        {cliRunning && <div className="fy-offline"><span>Feynman CLI is running.</span><button className="pill" onClick={() => pane('cli')}>Return to CLI</button></div>}
        {status === 'offline' && !cliRunning && <div className="fy-offline"><button type="button" className="pill" onClick={() => void session.connect()}>Reconnect</button></div>}
        <Composer session={session} documentID={documentID} doc={doc} input={input} choose={choose} edit={edit} setEdit={setEdit} />
      </>}
  </section>
}
