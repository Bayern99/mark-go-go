import { useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { Popover } from 'radix-ui'
import type { AppPlugin, DocumentSnapshot, PluginContext } from '../../core/types'
import { CheckpointHost } from '../../host/generated/mygo'
import { DocumentDiff } from '../DocumentDiff'

type PanelProps = { ctx: PluginContext; documentID: string }
type Preview = { base: DocumentSnapshot; saved: string }

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
  const [ids, setIDs] = useState<string[] | null>(null)
  const [selected, setSelected] = useState('')
  const [preview, setPreview] = useState<Preview | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    if (!doc.path) return
    let active = true
    void CheckpointHost.list(documentID, doc.path).then(
      (values) => { if (active) setIDs(values) },
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

  if (!doc.path) return <p className="nav-empty">Save this document to use Git checkpoints.</p>
  const id = selected || ids?.[0] || ''
  const stale = preview && (preview.base.text !== doc.text || preview.base.version !== doc.version)
  return (
    <section className="plugin-panel" aria-label="Git checkpoints">
      <button type="button" className="pill" disabled={busy || !ids} onClick={() => void run(async () => {
        const current = ctx.documents.snapshot()
        const next = await CheckpointHost.create(documentID, current.path, current.text)
        setIDs(next)
        setSelected('')
        setPreview(null)
      })}>Create checkpoint</button>
      {ids?.length === 0 && <p>No checkpoints for this document.</p>}
      {id && <>
        <select aria-label="Saved checkpoint" disabled={busy} value={id} onChange={event => {
          setSelected(event.target.value)
          setPreview(null)
        }}>
          {ids?.map(value => <option key={value} value={value}>
            {new Date(Number(value) / 1000).toLocaleString()}
          </option>)}
        </select>
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
        title: 'Git checkpoints',
        location: 'document',
        mount(el) {
          const root = createRoot(el)
          root.render(<Popover.Root>
            <Popover.Trigger asChild>
              <button type="button" className="pill icon quiet" aria-label="Git checkpoints" title="Git checkpoints">
                <i className="ph ph-git-branch" aria-hidden="true" />
              </button>
            </Popover.Trigger>
            <Popover.Portal>
              <Popover.Content className="doc-tab-menu checkpoint-menu" aria-label="Git checkpoints" sideOffset={6} align="end" collisionPadding={8}>
                <h3>Git checkpoints</h3>
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
