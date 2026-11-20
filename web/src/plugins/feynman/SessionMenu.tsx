import { useEffect, useState } from 'react'
import { Popover } from 'radix-ui'
import {
  ResearchHost,
  type ResearchConversation,
  type ResearchForkMessage,
  type ResearchTree,
  type ResearchTreeNode,
} from '../../host/generated/mygo'
import type { ResearchSession } from './session'
import './session-menu.css'

type MenuView = 'list' | 'fork' | 'tree'

function TreeBranch({
  node,
  leafId,
  depth = 0,
}: {
  node: ResearchTreeNode
  leafId?: string
  depth?: number
}) {
  const current = leafId !== undefined && node.entry.id === leafId
  const label =
    node.label ||
    node.entry.name ||
    node.entry.text ||
    node.entry.summary ||
    node.entry.modelId ||
    node.entry.type
  return (
    <div className="fy-tree-node" style={{ paddingInlineStart: `${depth * 12}px` }}>
      <div className={`fy-tree-item${current ? ' current' : ''}`} data-entry-id={node.entry.id}>
        <span className="fy-tree-role meta">{node.entry.role || node.entry.type}</span>
        <span className="fy-tree-text">{label}</span>
        {current && <span className="fy-tree-leaf meta">Current</span>}
      </div>
      {node.children.map((child) => (
        <TreeBranch key={child.entry.id} node={child} leafId={leafId} depth={depth + 1} />
      ))}
    </div>
  )
}

export function SessionMenu({ session }: { session: ResearchSession }) {
  const [open, setOpen] = useState(false)
  const [view, setView] = useState<MenuView>('list')
  const [rows, setRows] = useState<ResearchConversation[] | null>(null)
  const [forkItems, setForkItems] = useState<ResearchForkMessage[] | null>(null)
  const [treeData, setTreeData] = useState<ResearchTree | null>(null)
  const [renaming, setRenaming] = useState(false)
  const [draftName, setDraftName] = useState('')
  const [error, setError] = useState('')
  const [deleting, setDeleting] = useState('')
  const [search, setSearch] = useState('')
  const [pending, setPending] = useState(false)

  useEffect(() => {
    if (!open) return
    let alive = true
    setView('list')
    setRows(null)
    setForkItems(null)
    setTreeData(null)
    setRenaming(false)
    setDraftName(session.title || '')
    setError('')
    setDeleting('')
    setSearch('')
    ResearchHost.sessions(session.root).then(
      (result) => {
        if (alive) setRows(result)
      },
      (reason) => {
        if (alive) setError(String(reason))
      },
    )
    return () => {
      alive = false
    }
  }, [open, session])

  const busy = pending || session.status !== 'ready' || session.configuring
  const unsent = Boolean(session.draft.trim() || session.sources.length || session.proposal)

  async function choose(id: string) {
    setPending(true)
    setError('')
    try {
      await session.selectConversation(id)
      setOpen(false)
    } catch (err) {
      setError(String(err))
    } finally {
      setPending(false)
    }
  }

  async function remove(id: string) {
    setPending(true)
    setError('')
    try {
      await ResearchHost.deleteSession(session.root, id)
      setRows((value) => value?.filter((c) => c.id !== id) ?? null)
      setDeleting('')
    } catch (err) {
      setError(String(err))
    } finally {
      setPending(false)
    }
  }

  async function submitRename(e: React.FormEvent) {
    e.preventDefault()
    if (!draftName.trim()) return
    setPending(true)
    setError('')
    try {
      await session.renameSession(draftName)
      setRenaming(false)
      const next = await ResearchHost.sessions(session.root)
      setRows(next)
    } catch (err) {
      setError(String(err))
    } finally {
      setPending(false)
    }
  }

  async function openForkView() {
    setView('fork')
    setForkItems(null)
    setError('')
    try {
      const msgs = await ResearchHost.forkMessages(session.root)
      setForkItems(msgs)
    } catch (err) {
      setError(String(err))
    }
  }

  async function pickFork(entryId: string) {
    setPending(true)
    setError('')
    try {
      await session.forkFrom(entryId)
      setOpen(false)
    } catch (err) {
      setError(String(err))
    } finally {
      setPending(false)
    }
  }

  async function openTreeView() {
    setView('tree')
    setTreeData(null)
    setError('')
    try {
      const data = await ResearchHost.tree(session.root)
      setTreeData(data)
    } catch (err) {
      setError(String(err))
    }
  }

  async function runConversationAction(action: 'clone' | 'compact') {
    setPending(true)
    setError('')
    try {
      if (action === 'clone') {
        if (await ResearchHost.clone(session.root)) setOpen(false)
      } else {
        await ResearchHost.compact(session.root, '')
        setOpen(false)
      }
    } catch (err) {
      setError(String(err))
    } finally {
      setPending(false)
    }
  }

  const matches = rows?.filter((row) =>
    (row.title || 'Untitled conversation').toLowerCase().includes(search.trim().toLowerCase()),
  )

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>
        <button
          type="button"
          className="pill icon quiet"
          aria-label="Conversation history"
          title="Conversation history"
          disabled={session.status === 'offline' || session.status === 'connecting' || !session.root}
        >
          <i className="ph ph-clock-counter-clockwise" aria-hidden="true" />
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          className="research-history"
          align="end"
          sideOffset={6}
          collisionPadding={8}
          aria-label="Workspace conversations"
        >
          <header>
            {view === 'list' ? (
              <h3>Conversations</h3>
            ) : (
              <button type="button" className="pill quiet" onClick={() => setView('list')}>
                <i className="ph ph-arrow-left" aria-hidden="true" />
                {view === 'fork' ? 'Fork' : 'Tree'}
              </button>
            )}
            <div className="research-history-tools">
              {view === 'list' && (
                <>
                  <button
                    type="button"
                    className="pill quiet"
                    aria-label="Rename conversation"
                    title="Rename conversation"
                    disabled={busy}
                    onClick={() => {
                      setDraftName(session.title || '')
                      setRenaming((v) => !v)
                    }}
                  >
                    <i className="ph ph-pencil-simple" aria-hidden="true" />
                    Rename
                  </button>
                  <button
                    type="button"
                    className="pill quiet"
                    aria-label="Fork conversation"
                    title="Fork from a message"
                    disabled={busy || unsent}
                    onClick={() => void openForkView()}
                  >
                    <i className="ph ph-git-fork" aria-hidden="true" />
                    Fork
                  </button>
                  <button
                    type="button"
                    className="pill quiet"
                    aria-label="Clone conversation"
                    disabled={busy || unsent}
                    onClick={() => void runConversationAction('clone')}
                  >
                    <i className="ph ph-copy" aria-hidden="true" />
                    Clone
                  </button>
                  <button
                    type="button"
                    className="pill quiet"
                    aria-label="Compact conversation"
                    disabled={busy}
                    onClick={() => void runConversationAction('compact')}
                  >
                    <i className="ph ph-arrows-in" aria-hidden="true" />
                    Compact
                  </button>
                  <button
                    type="button"
                    className="pill quiet"
                    aria-label="Conversation tree"
                    title="View conversation tree"
                    disabled={busy}
                    onClick={() => void openTreeView()}
                  >
                    <i className="ph ph-tree-structure" aria-hidden="true" />
                    Tree
                  </button>
                  <button type="button" className="pill" disabled={busy || unsent} onClick={() => void choose('')}>
                    <i className="ph ph-plus" aria-hidden="true" /> New
                  </button>
                </>
              )}
            </div>
          </header>

          {renaming && view === 'list' && (
            <form className="research-rename-form" onSubmit={(e) => void submitRename(e)}>
              <input
                type="text"
                aria-label="Conversation name"
                placeholder="Conversation name…"
                value={draftName}
                onChange={(e) => setDraftName(e.target.value)}
                autoFocus
              />
              <button type="submit" className="pill primary" disabled={busy || !draftName.trim()}>
                Save
              </button>
              <button type="button" className="pill quiet" onClick={() => setRenaming(false)}>
                Cancel
              </button>
            </form>
          )}

          {error && <p role="alert">{error}</p>}

          {view === 'fork' ? (
            <div className="research-fork-list">
              {forkItems === null && !error && <p role="status">Loading messages…</p>}
              {forkItems && forkItems.length === 0 && <p>No forkable messages in this conversation yet.</p>}
              {forkItems?.map((item) => (
                <button
                  key={item.entryId}
                  type="button"
                  className="research-conversation-choice"
                  disabled={busy}
                  onClick={() => void pickFork(item.entryId)}
                >
                  <span>{item.text || '(Empty message)'}</span>
                </button>
              ))}
            </div>
          ) : view === 'tree' ? (
            <div className="research-tree-list" aria-label="Conversation tree">
              {treeData === null && !error && <p role="status">Loading tree…</p>}
              {treeData && treeData.tree.length === 0 && <p>No entries in this conversation yet.</p>}
              {treeData?.tree.map((node) => (
                <TreeBranch key={node.entry.id} node={node} leafId={treeData.leafId} />
              ))}
            </div>
          ) : (
            <>
              {unsent && <p>Send or clear this draft and finish any proposed edit to switch conversations.</p>}
              {rows === null && !error && <p role="status">Loading conversations…</p>}
              {rows && rows.length > 5 && (
                <input
                  className="research-history-search"
                  type="search"
                  aria-label="Find a conversation"
                  value={search}
                  placeholder="Find a conversation…"
                  onChange={(event) => setSearch(event.target.value)}
                />
              )}
              {search && matches?.length === 0 && <p>No matching conversations.</p>}
              {matches?.map((row) => (
                <div className="research-history-row" key={row.id}>
                  {deleting === row.id ? (
                    <div className="research-history-confirm">
                      <span>Delete “{row.title || 'Untitled conversation'}”?</span>
                      <button type="button" className="pill" disabled={busy} onClick={() => setDeleting('')}>
                        Cancel
                      </button>
                      <button type="button" className="pill" disabled={busy} onClick={() => void remove(row.id)}>
                        Delete
                      </button>
                    </div>
                  ) : (
                    <>
                      <button
                        type="button"
                        className="research-conversation-choice"
                        aria-current={row.current || undefined}
                        disabled={busy || unsent || row.current}
                        onClick={() => void choose(row.id)}
                      >
                        <span>{row.title || 'Untitled conversation'}</span>
                        {row.updatedAt && <time dateTime={row.updatedAt}>{new Date(row.updatedAt).toLocaleString()}</time>}
                      </button>
                      {row.current ? (
                        <i className="ph ph-check" aria-label="Current conversation" />
                      ) : (
                        <button
                          type="button"
                          className="pill icon quiet"
                          aria-label={`Delete ${row.title || 'untitled conversation'}`}
                          disabled={busy}
                          onClick={() => setDeleting(row.id)}
                        >
                          <i className="ph ph-trash" aria-hidden="true" />
                        </button>
                      )}
                    </>
                  )}
                </div>
              ))}
            </>
          )}
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}
