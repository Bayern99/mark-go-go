import { useEffect, useState } from 'react'
import { Popover } from 'radix-ui'
import { ResearchHost, type ResearchConversation } from '../../host/generated/mygo'
import type { ResearchSession } from './session'
import './session-menu.css'

export function SessionMenu({ session }: { session: ResearchSession }) {
  const [open, setOpen] = useState(false)
  const [rows, setRows] = useState<ResearchConversation[] | null>(null)
  const [error, setError] = useState('')
  const [deleting, setDeleting] = useState('')
  const [search, setSearch] = useState('')
  const [pending, setPending] = useState(false)
  useEffect(() => {
    if (!open) return
    let alive = true
    setRows(null)
    setError('')
    setDeleting('')
    setSearch('')
    ResearchHost.sessions(session.root).then(
      result => { if (alive) setRows(result) },
      reason => { if (alive) setError(String(reason)) },
    )
    return () => { alive = false }
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
      setRows(value => value?.filter(c => c.id !== id) ?? null)
      setDeleting('')
    } catch (err) {
      setError(String(err))
    } finally {
      setPending(false)
    }
  }
  const matches = rows?.filter(row => (row.title || 'Untitled conversation').toLowerCase().includes(search.trim().toLowerCase()))
  return <Popover.Root open={open} onOpenChange={setOpen}>
    <Popover.Trigger asChild>
      <button type="button" className="pill icon quiet" aria-label="Conversation history" title="Conversation history"
        disabled={session.status === 'offline' || session.status === 'connecting' || !session.root}>
        <i className="ph ph-clock-counter-clockwise" aria-hidden="true" />
      </button>
    </Popover.Trigger>
    <Popover.Portal>
      <Popover.Content className="research-history" align="end" sideOffset={6} collisionPadding={8} aria-label="Workspace conversations">
        <header>
          <h3>Conversations</h3>
          <button type="button" className="pill" disabled={busy || unsent} onClick={() => void choose('')}>
            <i className="ph ph-plus" aria-hidden="true" /> New
          </button>
        </header>
        {error && <p role="alert">{error}</p>}
        {unsent && <p>Send or clear this draft and finish any proposed edit to switch conversations.</p>}
        {rows === null && !error && <p role="status">Loading conversations…</p>}
        {rows && rows.length > 5 && <input className="research-history-search" type="search" aria-label="Find a conversation"
          value={search} placeholder="Find a conversation…" onChange={event => setSearch(event.target.value)} />}
        {search && matches?.length === 0 && <p>No matching conversations.</p>}
        {matches?.map(row => <div className="research-history-row" key={row.id}>
          {deleting === row.id ? <div className="research-history-confirm">
            <span>Delete “{row.title || 'Untitled conversation'}”?</span>
            <button type="button" className="pill" disabled={busy} onClick={() => setDeleting('')}>Cancel</button>
            <button type="button" className="pill" disabled={busy} onClick={() => void remove(row.id)}>Delete</button>
          </div> : <>
            <button type="button" className="research-conversation-choice" aria-current={row.current || undefined}
              disabled={busy || unsent || row.current} onClick={() => void choose(row.id)}>
              <span>{row.title || 'Untitled conversation'}</span>
              {row.updatedAt && <time dateTime={row.updatedAt}>{new Date(row.updatedAt).toLocaleString()}</time>}
            </button>
            {row.current ? <i className="ph ph-check" aria-label="Current conversation" />
              : <button type="button" className="pill icon quiet" aria-label={`Delete ${row.title || 'untitled conversation'}`}
                disabled={busy} onClick={() => setDeleting(row.id)}><i className="ph ph-trash" aria-hidden="true" /></button>}
          </>}
        </div>)}
      </Popover.Content>
    </Popover.Portal>
  </Popover.Root>
}
