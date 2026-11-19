import { useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { Popover } from 'radix-ui'
import type { AppPlugin, DocumentSnapshot, PluginContext } from '../../core/types'
import { CheckpointHost, type Checkpoint } from '../../host/generated/mygo'
import { DocumentDiff } from '../DocumentDiff'

type PanelProps = { ctx: PluginContext; documentID: string }
type Preview = { base: DocumentSnapshot; saved: string }

function formatCheckpointSource(source?: string): string {
  const s = (source ?? '').trim()
  const lower = s.toLowerCase()
  if (!s || lower === 'user') return 'User'
  if (lower === 'feynman') return 'Feynman'
  if (lower === 'mcp') return 'MCP'
  return s
}

function formatCheckpointTime(cp: Checkpoint): string {
  const ms = cp.createdAt > 0 ? cp.createdAt : Number(cp.id) / 1000
  return Number.isFinite(ms) && ms > 0 ? new Date(ms).toLocaleString() : cp.id
}

function normalizeCheckpoints(values: readonly (Checkpoint | string)[]): Checkpoint[] {
  return values.map(value => {
    if (typeof value === 'string') {
      return { id: value, createdAt: Number(value) / 1000, source: 'user' }
    }
    return value
  })
}

function GitPanel({ ctx, documentID }: PanelProps) {
  const [doc, setDoc] = useState(() => ctx.documents.snapshot())
  useEffect(() => {
    const sub = ctx.documents.onDidChange(setDoc)
    setDoc(ctx.documents.snapshot())
    return () => sub.dispose()
  }, [ctx])
  return <Checkpoints key={doc.path} ctx={ctx} documentID={documentID} doc={doc} />
}

function Checkpoints({ ctx, documentID, doc }: PanelProps & { doc: DocumentSnapshot }) {
  const [checkpoints, setCheckpoints] = useState<Checkpoint[] | null>(null)
  const [label, setLabel] = useState('')
  const [selected, setSelected] = useState('')
  const [preview, setPreview] = useState<Preview | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    if (!doc.path) return
    let active = true
    void CheckpointHost.list(documentID, doc.path).then(
      (values) => { if (active) setCheckpoints(normalizeCheckpoints(values)) },
      (reason: unknown) => { if (active) setError(String(reason)) },
    )
    return () => { active = false }
  }, [documentID, doc.path])

  async function run(action: () => Promise<void>) {
    setBusy(true)
    setError('')
    try {
      await action()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason))
    } finally {
      setBusy(false)
    }
  }

  if (!doc.path) return <p className="nav-empty">Save this document to use Checkpoints.</p>
  const active = checkpoints?.find(item => item.id === selected) ?? checkpoints?.[0] ?? null
  const id = active?.id ?? ''
  const stale = preview && (preview.base.text !== doc.text || preview.base.version !== doc.version)
  return (
    <section className="plugin-panel" aria-label="Checkpoints">
      <form className="checkpoint-create-form" onSubmit={event => {
        event.preventDefault()
        void run(async () => {
          const current = ctx.documents.snapshot()
          const next = await CheckpointHost.create(documentID, current.path, current.text, label.trim(), 'user')
          setCheckpoints(normalizeCheckpoints(next))
          setLabel('')
          setSelected('')
          setPreview(null)
        })
      }}>
        <input
          type="text"
          className="checkpoint-label-input"
          aria-label="Checkpoint label"
          placeholder="Optional label (e.g. Before Feynman edit)"
          value={label}
          disabled={busy || !checkpoints}
          onChange={event => setLabel(event.target.value)}
        />
        <button type="submit" className="pill" disabled={busy || !checkpoints}>Create checkpoint</button>
      </form>
      {checkpoints?.length === 0 && <p>No checkpoints for this document.</p>}
      {active && checkpoints && checkpoints.length > 0 && <>
        <div className="checkpoint-list" role="listbox" aria-label="Saved checkpoints">
          {checkpoints.map(item => {
            const isSelected = item.id === id
            return (
              <button
                key={item.id}
                type="button"
                role="option"
                aria-selected={isSelected}
                disabled={busy}
                className={`checkpoint-item${isSelected ? ' active' : ''}`}
                onClick={() => {
                  setSelected(item.id)
                  setPreview(null)
                }}
              >
                <div className="checkpoint-item-head">
                  <span className="checkpoint-item-label">{item.label || 'Snapshot'}</span>
                  <span className="checkpoint-badge" data-source={item.source || 'user'}>
                    {formatCheckpointSource(item.source)}
                  </span>
                </div>
                <div className="checkpoint-item-meta">
                  <time className="checkpoint-time">{formatCheckpointTime(item)}</time>
                  {item.backend && (
                    <span className="checkpoint-backend" data-backend={item.backend}>
                      {item.backend === 'local' ? 'Local snapshot' : 'Git ref'}
                    </span>
                  )}
                </div>
              </button>
            )
          })}
        </div>
        <button type="button" className="pill" disabled={busy} onClick={() => void run(async () => {
          const base = ctx.documents.snapshot()
          setPreview({ base, saved: await CheckpointHost.read(documentID, base.path, id) })
        })}>Compare</button>
      </>}
      {preview && <>
        <DocumentDiff before={preview.base.text} after={preview.saved} />
        {stale && <p role="status">The document changed. Compare again before restoring.</p>}
        <button type="button" className="pill" disabled={busy || !!stale || preview.base.text === preview.saved}
          onClick={() => void run(async () => {
            const applied = await ctx.commands.execute('editor.applyMarkdown', preview.base, preview.saved)
            if (!applied) throw new Error('The document changed. Compare again before restoring.')
            setPreview(null)
          })}>Restore shown version</button>
      </>}
      {error && <p role="alert">{error}</p>}
    </section>
  )
}

export function createGitPlugin(documentID: string): AppPlugin {
  return {
    id: 'git',
    title: 'Git',
    activate(ctx) {
      ctx.views.register({
        id: 'git.panel',
        title: 'Checkpoints',
        location: 'document',
        mount(el) {
          const root = createRoot(el)
          root.render(<Popover.Root>
            <Popover.Trigger asChild>
              <button type="button" className="pill icon quiet" aria-label="Checkpoints" title="Checkpoints">
                <i className="ph ph-git-branch" aria-hidden="true" />
              </button>
            </Popover.Trigger>
            <Popover.Portal>
              <Popover.Content className="doc-tab-menu checkpoint-menu" aria-label="Checkpoints" sideOffset={6} align="end" collisionPadding={8}>
                <h3>Checkpoints</h3>
                <GitPanel ctx={ctx} documentID={documentID} />
              </Popover.Content>
            </Popover.Portal>
          </Popover.Root>)
          return { dispose: () => queueMicrotask(() => root.unmount()) }
        },
      })
    },
  }
}
