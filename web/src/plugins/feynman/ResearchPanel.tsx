import { useEffect, useLayoutEffect, useReducer, useRef, useState, useSyncExternalStore } from 'react'
import { MovingCapsule } from '../../components/MovingCapsule'
import { ScrollArea } from '../../components/ScrollArea'
import { radioKeys } from '../../components/radioKeys'
import type { PluginContext } from '../../core'
import { events, ResearchHost, type HunkDecision, type ResearchArtifact } from '../../host/generated/mygo'
import { DocumentDiff, reconcileProposal } from '../DocumentDiff'
import { AlphaXivPanel } from './AlphaXivPanel'
import { Composer, useCommand } from './Composer'
import { Conversation, ResearchIcon } from './Conversation'
import type { FeynmanSettingsSlotProps, OpenResearchDocument } from './index'
import { ResearchQuestion } from './ResearchQuestion'
import { SessionMenu } from './SessionMenu'
import type { Proposal, ResearchSession } from './session'
import './research.css'

function sourceLabel(source?: string): string {
  if (!source) return 'Agent'
  if (source.toLowerCase() === 'mcp') return 'MCP'
  if (source.toLowerCase() === 'feynman') return 'Feynman'
  return source.charAt(0).toUpperCase() + source.slice(1)
}

export default function ResearchPanel({
  session,
  ctx,
  documentID,
  open,
  renderSettings,
}: {
  session: ResearchSession
  ctx?: {
    documents: Pick<PluginContext['documents'], 'snapshot' | 'onDidChange'>
    commands: Pick<PluginContext['commands'], 'execute'>
  }
  documentID: string
  open: OpenResearchDocument
  renderSettings?: (props: FeynmanSettingsSlotProps) => React.ReactNode
}) {
  useSyncExternalStore(session.subscribe, session.snapshot)
  const [editTarget, setEditTarget] = useState('')
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [alphaOpen, setAlphaOpen] = useState(false)
  const edit = !!documentID && editTarget === documentID
  const setEdit = (value: boolean) => setEditTarget(value ? documentID : '')
  const input = useRef<HTMLTextAreaElement>(null)
  const tabsRef = useRef<HTMLDivElement>(null)
  const returnFocus = useRef(false)
  const choose = useCommand(session, input)
  const { tab } = session
  useLayoutEffect(() => {
    if (!settingsOpen && !alphaOpen && returnFocus.current) {
      returnFocus.current = false
      tabsRef.current?.querySelector<HTMLButtonElement>('[aria-checked="true"]')?.focus()
    }
  }, [settingsOpen, alphaOpen, tab])
  useEffect(() => {
    if (tab === 'chat') void session.connect()
  }, [session, tab])
  useEffect(() => {
    if (!session.root || settingsOpen) return
    const refreshReadyModels = () => {
      if (session.status === 'ready') {
        void ResearchHost.refreshModels?.(session.root)?.catch?.(() => {})
      }
    }
    window.addEventListener('focus', refreshReadyModels)
    let offFocus: (() => void) | undefined
    try {
      offFocus = events.researchFocus?.on((ev) => {
        if (ev.root === session.root) refreshReadyModels()
      })
    } catch {
      // Standalone test mocks may omit researchFocus event.
    }
    return () => {
      window.removeEventListener('focus', refreshReadyModels)
      offFocus?.()
    }
  }, [session, session.root, settingsOpen])
  const [, refreshDoc] = useReducer((n: number) => n + 1, 0)
  useLayoutEffect(() => {
    const sub = ctx?.documents.onDidChange(refreshDoc)
    return () => sub?.dispose()
  }, [ctx])
  const doc = ctx?.documents.snapshot()
  const { status, permission, proposals, proposal } = session
  const sameTarget = proposal && doc && proposal.documentID === documentID
  const movedOrReloaded = !!(
    sameTarget &&
    (doc.path !== proposal.base.path || doc.version !== proposal.base.version)
  )
  const reconciled =
    sameTarget && !movedOrReloaded
      ? reconcileProposal(proposal.base.text, proposal.text, doc.text, proposal.decisions)
      : null
  const stale =
    movedOrReloaded ||
    Boolean(reconciled && reconciled.hasConflict && reconciled.hunks.every((h) => h.conflict))
  const busy = status === 'working'
  const report = (error: unknown) => {
    session.error = String(error)
    session.changed()
  }
  const comparison = useRef<HTMLElement>(null)

  function showComparison() {
    session.tab = 'review'
    session.changed()
    requestAnimationFrame(() => comparison.current?.scrollIntoView({ block: 'nearest' }))
  }

  async function showPending(targetProposal: Proposal | undefined = session.proposal) {
    if (!targetProposal) return
    session.selectProposal(targetProposal.id)
    const target = await open(targetProposal.base.path, targetProposal.documentID)
    if (target.id !== targetProposal.documentID) {
      const live = !targetProposal.documentID ? await target.snapshot() : null
      const nextBase =
        live && live.text === targetProposal.base.text ? live : targetProposal.base
      await session.updateProposalBase(targetProposal.id, target.id, nextBase)
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
    const existing = session.proposals.find((p) => p.base.path === file.target)
    if (existing) return showPending(existing)
    const target = await open(file.target)
    const [base, text] = await Promise.all([
      target.snapshot(),
      ResearchHost.readArtifact(session.root, file.path),
    ])
    if (session.status === 'working') {
      throw new Error('Finish the current research action before reviewing this file.')
    }
    if (base.path !== file.target) {
      throw new Error('The document moved. Refresh research files before reviewing.')
    }
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
    await session.applyProposal(proposal, reconciled.remainingText, nextText)
    await session.refresh()
  }

  const label =
    status === 'ready'
      ? 'Connected'
      : status === 'working'
        ? 'Working'
        : status === 'connecting'
          ? 'Connecting…'
          : 'Offline'
  const pane = (nextTab: ResearchSession['tab']) => {
    setSettingsOpen(false)
    setAlphaOpen(false)
    session.tab = nextTab
    session.changed()
  }

  const returnToChat = () => {
    returnFocus.current = true
    pane('chat')
  }

  const insertToChat = (text: string) => {
    const trimmed = text.trim()
    if (!trimmed) return
    session.draft = session.draft.trim() ? `${session.draft.trimEnd()}\n\n${trimmed}` : trimmed
    session.tab = 'chat'
    session.changed()
    setSettingsOpen(false)
    setAlphaOpen(false)
    requestAnimationFrame(() => input.current?.focus())
  }

  function fileButton(file: ResearchArtifact) {
    const queued = session.proposals.some((p) => p.base.path === file.target)
    return (
      <div className="fy-file-wrap" key={file.path}>
        {file.error && (
          <p role="alert">
            {file.path} · {file.error}
          </p>
        )}
        {(file.target || (file.missing && !file.error)) && (
          <button
            type="button"
            className="research-file"
            title={queued ? `View queued proposal for ${file.path}` : file.path}
            disabled={busy && (file.proposal || file.missing)}
            onClick={() => void review(file).catch(report)}
          >
            <ResearchIcon
              name={
                file.missing
                  ? 'arrow-counter-clockwise'
                  : file.path.endsWith('.provenance.md')
                    ? 'link'
                    : file.proposal
                      ? 'git-diff'
                      : 'file-text'
              }
            />
            <span>
              <b>
                {file.missing ? 'Restore ' : file.proposal ? 'Review ' : ''}
                {file.path.split('/').at(-1)}
              </b>
              {file.path.includes('/') && <small>{file.path.slice(0, file.path.lastIndexOf('/'))}</small>}
            </span>
            <ResearchIcon name="arrow-up-right" />
          </button>
        )}
      </div>
    )
  }

  const canApplyMerged = Boolean(
    reconciled && (reconciled.mergedText !== doc?.text || !reconciled.hasConflict),
  )

  const widgetList = Array.from(session.widgets.entries())
  const aboveWidgets = widgetList.filter(([, w]) => w.placement !== 'belowEditor')
  const belowWidgets = widgetList.filter(([, w]) => w.placement === 'belowEditor')

  return (
    <section className="plugin-panel fy-panel" aria-label="Feynman research">
      <header className="fy-heading">
        <ResearchIcon name="atom" />
        <div className="fy-heading-title">
          <h2>Feynman</h2>
          <small title={session.root}>
            {[session.root.split('/').filter(Boolean).at(-1), session.title].filter(Boolean).join(' · ')}
          </small>
        </div>
        <div className="fy-header-actions" data-settings-slot>
          <span className="fy-connection" data-status={status} title={label} aria-label={label} />
          <SessionMenu session={session} />
          {renderSettings && (
            <button
              type="button"
              className="pill icon quiet"
              data-testid="feynman-settings-toggle"
              aria-label="Feynman settings"
              aria-pressed={settingsOpen}
              title="Feynman settings"
              onClick={() => {
                if (settingsOpen) returnToChat()
                else {
                  setAlphaOpen(false)
                  setSettingsOpen(true)
                }
              }}
            >
              <ResearchIcon name="gear-six" />
            </button>
          )}
        </div>
      </header>

      {!settingsOpen && !alphaOpen && (
        <div className="fy-tabs seg" role="radiogroup" aria-label="Research views" ref={tabsRef} onKeyDown={radioKeys}>
          <MovingCapsule
            trackRef={tabsRef}
            selector="[aria-checked='true']"
            watch={[tab, proposals.length, Boolean(permission)]}
          />
          <button
            type="button"
            role="radio"
            aria-checked={tab === 'chat'}
            tabIndex={tab === 'chat' ? 0 : -1}
            onClick={() => pane('chat')}
          >
            <ResearchIcon name="chat-circle" />
            <span>Chat</span>
            {permission && <span className="fy-attention-label">Action required</span>}
          </button>
          <button
            type="button"
            role="radio"
            aria-checked={tab === 'review'}
            tabIndex={tab === 'review' ? 0 : -1}
            onClick={() => pane('review')}
          >
            <ResearchIcon name="git-diff" />
            <span>Review</span>
            {proposals.length > 0 && (
              <span
                className="fy-tab-badge meta"
                data-testid="review-pending-badge"
                aria-label={`${proposals.length} pending`}
              >
                {proposals.length}
              </span>
            )}
          </button>
        </div>
      )}

      {session.error && (
        <div className="fy-error" role="alert">
          <span>{session.error}</span>
          <button
            className="fy-icon"
            type="button"
            aria-label="Dismiss error"
            onClick={() => {
              session.error = ''
              session.changed()
            }}
          >
            <ResearchIcon name="x" />
          </button>
        </div>
      )}

      {settingsOpen && renderSettings ? (
        <div className="fy-settings-host">
          {renderSettings({
            root: session.root,
            session,
            onClose: returnToChat,
            onInsertToChat: insertToChat,
          })}
        </div>
      ) : alphaOpen ? (
        <div className="fy-settings-host">
          <AlphaXivPanel
            root={session.root}
            onClose={returnToChat}
            onInsertToChat={insertToChat}
          />
        </div>
      ) : tab === 'review' ? (
        <ScrollArea className="fy-files" contentClassName="fy-files-content">
          <div className="fy-review-section">
            <div className="fy-files-heading">
              <h3>Change proposals</h3>
              {proposals.length > 0 && (
                <span className="fy-count-pill meta">{proposals.length}</span>
              )}
            </div>
            {!proposal ? (
              <p className="fy-muted">No pending change proposals.</p>
            ) : (
              <section ref={comparison} className="fy-proposal-review" aria-label="Proposed document edit">
                {proposals.length > 1 && (
                  <div className="diff-queue" role="list" aria-label="Pending proposals">
                    {proposals.map((item) => (
                      <button
                        key={item.id}
                        type="button"
                        className="pill quiet"
                        data-proposal-file={item.base.path}
                        aria-current={item.id === proposal.id ? 'true' : undefined}
                        onClick={() => void showPending(item).catch(report)}
                      >
                        {item.base.path.split('/').pop()}
                      </button>
                    ))}
                  </div>
                )}
                <div className="fy-proposal-meta">
                  <span className="checkpoint-badge" data-source={proposal.source || 'feynman'}>
                    {sourceLabel(proposal.source)}
                  </span>
                  <span className="fy-proposal-path">{proposal.base.path.split('/').pop()}</span>
                </div>
                {proposal.summary && <p className="fy-proposal-summary">{proposal.summary}</p>}
                {sameTarget ? (
                  <>
                    {reconciled ? (
                      <DocumentDiff
                        before={doc.text}
                        after={proposal.text}
                        hunks={reconciled.hunks}
                        onDecideHunk={(hunkId: string, decision: HunkDecision) =>
                          session.decideHunk(proposal.id, hunkId, decision, doc.text)
                        }
                        onAcceptAll={() =>
                          session.acceptAllHunks(
                            proposal.id,
                            reconciled.hunks.filter((h) => !h.conflict).map((h) => h.id),
                            doc.text,
                          )
                        }
                      />
                    ) : (
                      <DocumentDiff before={proposal.base.text} after={proposal.text} />
                    )}
                    {reconciled?.hasConflict && (
                      <p role="status">Some hunks conflict with concurrent edits in this document.</p>
                    )}
                    <div className="plugin-actions">
                      {stale ? (
                        <>
                          <p>The document changed since this comparison.</p>
                          <button
                            type="button"
                            className="pill"
                            onClick={() => {
                              if (ctx) {
                                void session.updateProposalBase(
                                  proposal.id,
                                  documentID,
                                  ctx.documents.snapshot(),
                                ).catch(report)
                              }
                            }}
                          >
                            Compare current text
                          </button>
                        </>
                      ) : (
                        <button
                          type="button"
                          className="pill primary"
                          disabled={!canApplyMerged}
                          onClick={() => void apply().catch(report)}
                        >
                          Apply in Markdown
                        </button>
                      )}
                      <button
                        type="button"
                        className="pill quiet"
                        onClick={() => {
                          session.dismissProposal(proposal.id)
                          setEdit(false)
                        }}
                      >
                        Dismiss
                      </button>
                    </div>
                  </>
                ) : (
                  <>
                    <DocumentDiff before={proposal.base.text} after={proposal.text} />
                    <div className="plugin-actions">
                      <button
                        type="button"
                        className="pill primary"
                        onClick={() => void showPending(proposal).catch(report)}
                      >
                        Open document {proposal.base.path.split('/').pop()}
                      </button>
                      <button
                        type="button"
                        className="pill quiet"
                        onClick={() =>
                          void navigator.clipboard.writeText(proposal.text).catch(report)
                        }
                      >
                        Copy proposed Markdown
                      </button>
                      <button
                        type="button"
                        className="pill quiet"
                        onClick={() => {
                          session.dismissProposal(proposal.id)
                          setEdit(false)
                        }}
                      >
                        Dismiss
                      </button>
                    </div>
                  </>
                )}
              </section>
            )}
          </div>

          <div className="fy-review-section">
            <div className="fy-files-heading">
              <h3>Research files</h3>
              <button
                className="fy-icon"
                type="button"
                aria-label="Refresh files"
                title="Refresh files"
                disabled={busy}
                onClick={() => void session.refresh()}
              >
                <ResearchIcon name="arrow-clockwise" />
              </button>
              <button
                type="button"
                className="fy-icon"
                title="Open research output folder"
                aria-label="Open research output folder"
                onClick={() => void ResearchHost.revealOutputs(session.root).catch(report)}
              >
                <ResearchIcon name="folder-open" />
              </button>
            </div>
            {!session.files.length && (
              <p className="fy-muted">Completed research files appear here.</p>
            )}
            {session.files.map(fileButton)}
          </div>
        </ScrollArea>
      ) : (
        <>
          {session.statuses.size > 0 && (
            <div className="fy-ext-statuses" data-testid="fy-ext-statuses" role="status">
              {Array.from(session.statuses, ([key, text]) => (
                <span key={key} className="fy-ext-status-pill" data-status-key={key}>
                  {text}
                </span>
              ))}
            </div>
          )}
          <Conversation
            session={session}
            choose={choose}
            open={review}
            onOpenReview={() => pane('review')}
          />
          {aboveWidgets.length > 0 && (
            <div className="fy-widgets" data-placement="aboveEditor">
              {aboveWidgets.map(([key, widget]) => (
                <div key={key} className="fy-widget" data-widget-key={key}>
                  {widget.lines.map((line, idx) => (
                    <div key={idx}>{line}</div>
                  ))}
                </div>
              ))}
            </div>
          )}
          {permission && (
            <ResearchQuestion
              session={session}
              key={permission.requestId}
              question={permission}
              onError={report}
            />
          )}
          {status === 'offline' && (
            <div className="fy-offline">
              <button type="button" className="pill" onClick={() => void session.connect()}>
                Reconnect
              </button>
            </div>
          )}
          <Composer
            session={session}
            documentID={documentID}
            doc={doc}
            input={input}
            choose={choose}
            edit={edit}
            setEdit={setEdit}
            onOpenAlpha={() => {
              setSettingsOpen(false)
              setAlphaOpen(true)
            }}
          />
          {belowWidgets.length > 0 && (
            <div className="fy-widgets" data-placement="belowEditor">
              {belowWidgets.map(([key, widget]) => (
                <div key={key} className="fy-widget" data-widget-key={key}>
                  {widget.lines.map((line, idx) => (
                    <div key={idx}>{line}</div>
                  ))}
                </div>
              ))}
            </div>
          )}
        </>
      )}
    </section>
  )
}
