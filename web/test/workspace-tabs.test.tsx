import '../src/styles.css'
import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'
import { App } from '../src/App'
import {
  createBrowserHost,
  createBrowserWorkspace,
  wrapHostAsWorkspace,
} from '../src/host/browserHost'
import type { WorkspaceAdapter } from '../src/host/types'

async function mountWorkspace(workspace: WorkspaceAdapter): Promise<{
  container: HTMLDivElement
  unmount(): Promise<void>
}> {
  const container = document.createElement('div')
  container.style.height = '100%'
  document.body.append(container)
  const root = createRoot(container)
  flushSync(() => {
    root.render(<App workspace={workspace} />)
  })
  await vi.waitFor(
    () => {
      if (!container.querySelector('[role="tablist"][aria-label="Documents"]')) {
        throw new Error('document tablist missing')
      }
    },
    { timeout: 5000 },
  )
  return {
    container,
    async unmount() {
      root.unmount()
      container.remove()
      await new Promise<void>((resolve) => setTimeout(resolve, 0))
    },
  }
}

describe('createBrowserWorkspace', () => {
  it('returns the same HostAdapter for an open session', async () => {
    const a = createBrowserHost('# A\n', 'a.md')
    const b = createBrowserHost('# B\n', 'b.md')
    const workspace = createBrowserWorkspace([a, b])
    expect(workspace.document(a.id)).toBe(a)
    expect(workspace.document(a.id)).toBe(workspace.document(a.id))
    expect(workspace.document(b.id)).toBe(b)
    expect(workspace.document(a.id)).not.toBe(workspace.document(b.id))
    expect((await workspace.snapshot()).documents.map((doc) => doc.id)).toEqual([a.id, b.id])
  })

  it('wrapHostAsWorkspace still binds one document stably', () => {
    const a = createBrowserHost('# A\n', 'a.md')
    const workspace = wrapHostAsWorkspace(a, 'a.md')
    expect(workspace.document(a.id)).toBe(workspace.document(a.id))
    expect(workspace.document(a.id).id).toBe(a.id)
  })

  it('flushes A without writing B when switching', async () => {
    const a = createBrowserHost('# A\n', 'a.md')
    const b = createBrowserHost('# B\n', 'b.md')
    const flushedA: string[] = []
    const flushedB: string[] = []
    const flushA = a.flush.bind(a)
    const flushB = b.flush.bind(b)
    a.flush = async (text: string) => {
      flushedA.push(text)
      await flushA(text)
    }
    b.flush = async (text: string) => {
      flushedB.push(text)
      await flushB(text)
    }
    const workspace = createBrowserWorkspace([a, b])
    await workspace.document(a.id).flush('# A edited\n')
    await workspace.activate(b.id)
    const snap = await workspace.snapshot()
    expect(snap.activeId).toBe(b.id)
    expect(flushedA).toEqual(['# A edited\n'])
    expect(flushedB).toEqual([])
    expect((await a.load()).text).toBe('# A edited\n')
    expect((await b.load()).text).toBe('# B\n')
  })

  it('open of an already-open path activates that document and does not allocate a second adapter', async () => {
    const a = createBrowserHost('# A\n', 'a.md')
    const b = createBrowserHost('# B\n', 'b.md')
    const workspace = createBrowserWorkspace([
      { host: a, path: '/work/a.md' },
      { host: b, path: '/work/b.md' },
    ])
    const before = workspace.document(a.id)
    const opened = await workspace.open('/work/a.md')
    expect(opened.id).toBe(a.id)
    expect(workspace.document(a.id)).toBe(before)
    expect((await workspace.snapshot()).documents).toHaveLength(2)
    expect((await workspace.snapshot()).activeId).toBe(a.id)
  })

  it('cancel close keeps both documents and the adapter cache', async () => {
    const a = createBrowserHost('# A\n', 'a.md')
    const b = createBrowserHost('# B\n', 'b.md')
    const workspace = createBrowserWorkspace([a, b], {
      confirmClose: async () => ({ closed: false, canceled: true }),
    })
    const adapter = workspace.document(a.id)
    const result = await workspace.closeDocument(a.id, '# pending\n')
    expect(result.canceled).toBe(true)
    const snap = await workspace.snapshot()
    expect(snap.documents.map((doc) => doc.id)).toEqual([a.id, b.id])
    expect(workspace.document(a.id)).toBe(adapter)
    expect((await a.load()).text).toBe('# pending\n')
  })
})

describe('App workspace tabs', () => {
  it('keeps both document panes mounted across a tab switch', async () => {
    const a = createBrowserHost('# Alpha doc\n', 'alpha.md')
    const b = createBrowserHost('# Beta doc\n', 'beta.md')
    const workspace = createBrowserWorkspace([a, b])
    const app = await mountWorkspace(workspace)
    try {
      await vi.waitFor(
        () => {
          const panes = app.container.querySelectorAll('[data-document-id]')
          if (panes.length !== 2) throw new Error(`expected 2 panes, got ${panes.length}`)
        },
        { timeout: 5000 },
      )
      const paneA = app.container.querySelector(`[data-document-id="${a.id}"]`)
      const paneB = app.container.querySelector(`[data-document-id="${b.id}"]`)
      if (!paneA || !paneB) throw new Error('document panes missing')
      expect(paneA.hasAttribute('hidden')).toBe(false)
      expect(paneB.hasAttribute('hidden')).toBe(true)

      const tabB = app.container.querySelector<HTMLButtonElement>(`[data-testid="doc-tab-${b.id}"]`)
      if (!tabB) throw new Error('tab B missing')
      flushSync(() => tabB.click())
      await vi.waitFor(() => {
        expect(tabB.getAttribute('aria-selected')).toBe('true')
      })
      expect(app.container.querySelector(`[data-document-id="${a.id}"]`)).toBe(paneA)
      expect(app.container.querySelector(`[data-document-id="${b.id}"]`)).toBe(paneB)
      expect(paneA.hasAttribute('hidden')).toBe(true)
      expect(paneB.hasAttribute('hidden')).toBe(false)
    } finally {
      await app.unmount()
    }
  })

  it('disambiguates same-name files by parent folder', async () => {
    const a = createBrowserHost('# alpha\n', 'notes.md')
    const b = createBrowserHost('# beta\n', 'notes.md')
    const workspace = createBrowserWorkspace([
      { host: a, name: 'notes.md', path: '/work/alpha/notes.md' },
      { host: b, name: 'notes.md', path: '/work/beta/notes.md' },
    ])
    const app = await mountWorkspace(workspace)
    try {
      await vi.waitFor(() => {
        const labels = [...app.container.querySelectorAll('[role="tab"] .nm')].map(
          (el) => el.textContent,
        )
        if (labels.length < 2) throw new Error('tabs not rendered')
        expect(labels).toEqual(['notes.md — alpha', 'notes.md — beta'])
      })
    } finally {
      await app.unmount()
    }
  })

  it('keeps both tabs when close is canceled', async () => {
    const a = createBrowserHost('# A\n', 'a.md')
    const b = createBrowserHost('# B\n', 'b.md')
    const workspace = createBrowserWorkspace([a, b], {
      confirmClose: async () => ({ closed: false, canceled: true }),
    })
    const app = await mountWorkspace(workspace)
    try {
      await vi.waitFor(() => {
        if (app.container.querySelectorAll('[role="tab"][data-testid^="doc-tab-"]').length !== 2) {
          throw new Error('tabs not ready')
        }
      })
      const closeA = app.container.querySelector<HTMLElement>(`[data-testid="doc-tab-close-${a.id}"]`)
      if (!closeA) throw new Error('close button missing')
      flushSync(() => closeA.click())
      await vi.waitFor(async () => {
        const snap = await workspace.snapshot()
        expect(snap.documents.map((doc) => doc.id)).toEqual([a.id, b.id])
      })
      expect(app.container.querySelector(`[data-testid="doc-tab-${a.id}"]`)).not.toBeNull()
      expect(app.container.querySelector(`[data-testid="doc-tab-${b.id}"]`)).not.toBeNull()
    } finally {
      await app.unmount()
    }
  })

  it('shows unsaved, conflict, deleted and read-only badges without activating the background tab', async () => {
    const a = createBrowserHost('# A\n', 'a.md')
    const b = createBrowserHost('# B\n', 'b.md')
    const workspace = createBrowserWorkspace([a, b])
    const app = await mountWorkspace(workspace)
    try {
      await workspace.activate(a.id)
      b.patchState({ edited: true, conflict: true, deleted: true, readOnly: true })
      await vi.waitFor(() => {
        const tab = app.container.querySelector(`[data-testid="doc-tab-${b.id}"]`)
        if (!tab) throw new Error('tab B missing')
        if (!tab.querySelector('.edited-dot')) throw new Error('edited badge missing')
        if (!tab.querySelector('.tab-conflict')) throw new Error('conflict badge missing')
        if (!tab.querySelector('.tab-deleted')) throw new Error('deleted badge missing')
        if (!tab.querySelector('.tab-readonly')) throw new Error('readonly badge missing')
      })
      expect((await workspace.snapshot()).activeId).toBe(a.id)
      const tabB = app.container.querySelector(`[data-testid="doc-tab-${b.id}"]`)
      expect(tabB?.getAttribute('aria-selected')).toBe('false')
    } finally {
      await app.unmount()
    }
  })

  it('activates the next tab with ArrowRight', async () => {
    const a = createBrowserHost('# A\n', 'a.md')
    const b = createBrowserHost('# B\n', 'b.md')
    const workspace = createBrowserWorkspace([a, b])
    const app = await mountWorkspace(workspace)
    try {
      const list = app.container.querySelector('[data-testid="doc-tabs"]')
      if (!list) throw new Error('tablist missing')
      list.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }))
      await vi.waitFor(async () => {
        expect((await workspace.snapshot()).activeId).toBe(b.id)
      })
    } finally {
      await app.unmount()
    }
  })
})

describe('createBrowserWorkspace identity', () => {
  it('New and Duplicate create independent untitled sessions', async () => {
    const a = createBrowserHost('# A\n', 'a.md')
    const workspace = createBrowserWorkspace([{ host: a, path: '/work/a.md' }], { root: '/work' })
    const created = await workspace.newDocument()
    expect(created.id).not.toBe(a.id)
    expect(created.path).toBe('')
    expect(workspace.document(created.id)).toBe(workspace.document(created.id))

    const copy = await workspace.duplicate(a.id)
    expect(copy.id).not.toBe(a.id)
    expect(copy.id).not.toBe(created.id)
    expect(copy.path).toBe('')
    expect((await workspace.document(copy.id).load()).text).toBe('# A\n')
    expect((await workspace.document(a.id).load()).text).toBe('# A\n')
    await workspace.document(copy.id).update('# copy\n')
    expect((await workspace.document(a.id).load()).text).toBe('# A\n')
  })

  it('closing the last folder tab keeps an empty workspace snapshot', async () => {
    const a = createBrowserHost('# A\n', 'a.md')
    const workspace = createBrowserWorkspace([{ host: a, path: '/work/a.md' }], {
      root: '/work',
      kind: 'folder',
    })
    const result = await workspace.closeDocument(a.id)
    expect(result.closed).toBe(true)
    const snap = await workspace.snapshot()
    expect(snap.kind).toBe('folder')
    expect(snap.documents).toEqual([])
    expect(snap.activeId).toBe('')
  })

  it('close without explicit text flushes that document via bindFlush', async () => {
    const a = createBrowserHost('# A\n', 'a.md')
    const flushed: string[] = []
    const inner = a.flush.bind(a)
    a.flush = async (text: string) => {
      flushed.push(text)
      await inner(text)
    }
    const workspace = createBrowserWorkspace([a], {
      confirmClose: async () => ({ closed: false, canceled: true }),
    })
    a.bindFlush?.(() => '# pending A\n')
    await workspace.closeDocument(a.id)
    expect(flushed).toEqual(['# pending A\n'])
    expect((await a.load()).text).toBe('# pending A\n')
    expect((await workspace.snapshot()).documents.map((doc) => doc.id)).toEqual([a.id])
  })

  it('does not allocate a second session when opening a path already open', async () => {
    const a = createBrowserHost('# A\n', 'a.md')
    const workspace = createBrowserWorkspace([{ host: a, path: '/work/a.md' }])
    const first = workspace.document(a.id)
    const opened = await workspace.open('/work/a.md')
    expect(opened.id).toBe(a.id)
    expect(workspace.document(a.id)).toBe(first)
    expect((await workspace.snapshot()).documents).toHaveLength(1)
  })
})
