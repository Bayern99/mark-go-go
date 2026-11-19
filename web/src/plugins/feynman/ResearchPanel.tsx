import { useEffect, useLayoutEffect, useReducer, useRef, useState, useSyncExternalStore } from 'react'
import type { PluginContext } from '../../core'
import { ResearchHost, type HunkDecision, type ResearchArtifact } from '../../host/generated/mygo'
import { ScrollArea } from '../../components/ScrollArea'
import { DocumentDiff, reconcileProposal } from '../DocumentDiff'
import { Conversation, ResearchIcon } from './Conversation'
import { Composer, useCommand } from './Composer'
import { SessionMenu } from './SessionMenu'
import { ResearchQuestion } from './ResearchQuestion'
import type { OpenResearchDocument } from './index'
import type { Proposal, ResearchSession } from './session'
import './research.css'

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
  const { tab } = session
  useEffect(() => { if (tab === 'chat') void session.connect() }, [session, tab])
  const [, refreshDoc] = useReducer(n => n + 1, 0)
  useLayoutEffect(() => {
    const sub = ctx?.documents.onDidChange(refreshDoc)
    return () => sub?.dispose()
  }, [ctx])
  const doc = ctx?.documents.snapshot()
  const { status, permission, proposals, proposal } = session
  const sameTarget = proposal && doc && proposal.documentID === documentID
  const movedOrReloaded = !!(sameTarget && (doc.path !== proposal.base.path || doc.version !== proposal.base.version))
  const reconciled = sameTarget && !movedOrReloaded
    ? reconcileProposal(proposal.base.text, proposal.text, doc.text, proposal.decisions)
    : null
  const stale = movedOrReloaded || Boolean(reconciled && reconciled.hasConflict && reconciled.hunks.every(h => h.conflict))
  const busy = status === 'working'
  const report = (error: unknown) => { session.error = String(error); session.changed() }
  const comparison = useRef<HTMLElement>(null)
  function showComparison() {
    session.tab = 'chat'
    session.changed()
    requestAnimationFrame(() => comparison.current?.scrollIntoView({ block: 'nearest' }))
  }
  async function showPending(targetProposal: Proposal | undefined = session.proposal) {
    if (!targetProposal) return
    session.selectProposal(targetProposal.id)
    const target = await open(targetProposal.base.path, targetProposal.documentID)
    if (target.id !== targetProposal.documentID) {
      session.updateProposalBase(targetProposal.id, target.id, targetProposal.base)
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
    const existing = session.proposals.find(p => p.base.path === file.target)
    if (existing) return showPending(existing)
    const target = await open(file.target)
    const [base, text] = await Promise.all([target.snapshot(), ResearchHost.readArtifact(session.root, file.path)])
    if (session.status === 'working') throw new Error('Finish the current research action before reviewing this file.')
    if (base.path !== file.target) throw new Error('The document moved. Refresh research files before reviewing.')
    if (text === base.text) return
    session.upsertProposal({ documentID: target.id, base, text, source: 'feynman' })
    showComparison()
  }
  async function apply() {
    if (!ctx || !proposal || proposal.documentID !== documentID || !doc || !reconciled) return
    const nextText = reconciled.mergedText
    if (nextText !== doc.text) {
      await ctx.commands.execute('editor.applyMarkdown', doc, nextText)
    }
    session.completeProposal(proposal.id)
    await session.refresh()
  }

  const label = status === 'ready' ? 'Connected' : status === 'working' ? 'Working' : status === 'connecting' ? 'Connecting…' : 'Offline'
  const pane = (tab: ResearchSession['tab']) => { session.tab = tab; session.changed() }
  function fileButton(file: ResearchArtifact) {
    const queued = session.proposals.some(p => p.base.path === file.target)
    return <div className="fy-file-wrap" key={file.path}>
      {file.error && <p role="alert">{file.path} · {file.error}</p>}
      {(file.target || (file.missing && !file.error)) && <button type="button" className="research-file"
        title={queued ? `View queued proposal for ${file.path}` : file.path}
        disabled={busy && (file.proposal || file.missing)} onClick={() => void review(file).catch(report)}>
        <ResearchIcon name={file.path.endsWith('.provenance.md') ? 'link' : file.proposal ? 'git-diff' : 'file-text'} />
        <span><b>{file.missing ? 'Restore ' : file.proposal ? 'Review ' : ''}{file.path.split('/').at(-1)}</b>
        {file.path.includes('/') && <small>{file.path.slice(0,file.path.lastIndexOf('/'))}</small>}</span>
        <ResearchIcon name="arrow-up-right" />
      </button>}
    </div>
  }
  const canApplyMerged = Boolean(reconciled && (reconciled.mergedText !== doc?.text || !reconciled.hasConflict))
  const reviewSlot = <>
    {proposal && <section ref={comparison} aria-label="Proposed document edit">
      {proposals.length > 1 && <div className="diff-queue" role="list" aria-label="Pending proposals">
        {proposals.map(item => <button
          key={item.id}
          type="button"
          className="pill quiet"
          data-proposal-file={item.base.path}
          aria-current={item.id === proposal.id ? 'true' : undefined}
          onClick={() => void showPending(item).catch(report)}
        >{item.base.path.split('/').pop()}</button>)}
      </div>}
      {sameTarget ? <>
        {reconciled ? (
          <DocumentDiff
            before={doc.text}
            after={proposal.text}
            hunks={reconciled.hunks}
            onDecideHunk={(hunkId: string, decision: HunkDecision) => session.decideHunk(proposal.id, hunkId, decision, doc.text)}
            onAcceptAll={() => session.acceptAllHunks(
              proposal.id,
              reconciled.hunks.filter(h => !h.conflict).map(h => h.id),
              doc.text,
            )}
          />
        ) : (
          <DocumentDiff before={proposal.base.text} after={proposal.text} />
        )}
        {reconciled?.hasConflict && <p role="status">Some hunks conflict with concurrent edits in this document.</p>}
        {stale ? <>
          <p>The document changed since this comparison.</p>
          <button className="pill" onClick={() => {
            if (ctx) session.updateProposalBase(proposal.id, documentID, ctx.documents.snapshot())
          }}>Compare current text</button>
        </> : <button className="pill" disabled={!canApplyMerged} onClick={() => void apply().catch(report)}>Apply in Markdown</button>}
      </> : <>
        <DocumentDiff before={proposal.base.text} after={proposal.text} />
        <button className="pill" onClick={() => void showPending(proposal).catch(report)}>Open document {proposal.base.path.split('/').pop()}</button>
      </>}
      {!sameTarget && <button className="pill quiet" onClick={() => void navigator.clipboard.writeText(proposal.text).catch(report)}>Copy proposed Markdown</button>}
      <button className="pill quiet" onClick={() => { session.dismissProposal(proposal.id); setEdit(false) }}>Dismiss</button>
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
    <nav className="fy-tabs" aria-label="Research tools">{(['chat', 'files'] as const).map(tab =>
      <button type="button" key={tab} aria-pressed={session.tab === tab} onClick={() => pane(tab)}>
        <ResearchIcon name={tab === 'chat' ? 'chat-circle' : 'files'} />
        {tab === 'chat' ? 'Chat' : 'Files'}{tab === 'chat' && permission && <small aria-label="Action required">!</small>}{tab === 'files' && session.files.length > 0 && <small>{session.files.length}</small>}
      </button>)}</nav>
    {session.error && <div className="fy-error" role="alert"><span>{session.error}</span><button className="fy-icon" aria-label="Dismiss error"
      onClick={() => { session.error = ''; session.changed() }}><ResearchIcon name="x" /></button></div>}
    {session.tab === 'files' ? <ScrollArea className="fy-files" contentClassName="fy-files-content">
        <div className="fy-files-heading"><h4>Research files</h4><button className="fy-icon" type="button" aria-label="Refresh files" title="Refresh files"
          disabled={busy} onClick={() => void session.refresh()}><ResearchIcon name="arrow-clockwise" /></button>
          <button type="button" className="fy-icon" title="Open research output folder" aria-label="Open research output folder"
            onClick={() => void ResearchHost.revealOutputs(session.root).catch(report)}><ResearchIcon name="folder-open" /></button></div>
        {!session.files.length && <p className="fy-muted">Completed research files appear here.</p>}
        {proposals.map(item => (
          <button key={item.id} type="button" className="pill" onClick={() => void showPending(item).catch(report)}>
            Review pending edit · {item.base.path.split('/').at(-1)}
          </button>
        ))}
        {session.files.map(fileButton)}
      </ScrollArea> : <>
        <Conversation session={session} choose={choose} open={review} reviewSlot={reviewSlot} />
        {permission && <ResearchQuestion session={session} key={permission.requestId} question={permission} onError={report} />}
        {status === 'offline' && <div className="fy-offline"><button type="button" className="pill" onClick={() => void session.connect()}>Reconnect</button></div>}
        <Composer session={session} documentID={documentID} doc={doc} input={input} choose={choose} edit={edit} setEdit={setEdit} />
      </>}
  </section>
}
