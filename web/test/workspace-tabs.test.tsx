import '../src/styles.css'
import { userEvent } from 'vitest/browser'
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

async function mountWorkspace(
  workspace: WorkspaceAdapter,
  width?: number,
): Promise<{
  container: HTMLDivElement
  unmount(): Promise<void>
}> {
  const container = document.createElement('div')
  container.style.height = '100%'
  if (width !== undefined) container.style.width = `${width}px`
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
  it('keeps all fitting tabs visible at the full tab bar width', async () => {
    const documents = [
      createBrowserHost('# Alpha\n', 'alpha-document-long-name.md'),
      createBrowserHost('# Beta\n', 'beta-document-long-name.md'),
      createBrowserHost('# Gamma\n', 'gamma-document-long-name.md'),
    ]
    const workspace = createBrowserWorkspace(documents)
    const app = await mountWorkspace(workspace, 1200)
    try {
      const bar = app.container.querySelector<HTMLElement>('[data-testid="doc-tabs"]')
      const overflow = app.container.querySelector<HTMLButtonElement>('[data-testid="doc-tab-overflow"]')
      if (!bar || !overflow) throw new Error('tab bar or overflow button missing')
      await vi.waitFor(() => expect(bar.querySelectorAll('.doc-tab')).toHaveLength(documents.length))
      await vi.waitFor(() => expect(overflow.hidden).toBe(true))

      const tabs = [...bar.querySelectorAll<HTMLElement>('.doc-tab')]
      const usedWidth = tabs[tabs.length - 1]!.getBoundingClientRect().right - bar.getBoundingClientRect().left
      const targetWidth = Math.ceil(usedWidth + 20)
      const setFullWidth = (width: number) => {
        const reserved = overflow.hidden ? 0 : overflow.offsetWidth + 2
        app.container.style.width = `${app.container.getBoundingClientRect().width + width - reserved - bar.clientWidth}px`
      }
      setFullWidth(targetWidth)

      await vi.waitFor(() => {
        expect(overflow.hidden).toBe(true)
        expect(bar.querySelectorAll('.doc-tab:not(.tab-hidden)')).toHaveLength(documents.length)
      })

      setFullWidth(targetWidth - 30)
      await vi.waitFor(() => expect(overflow.hidden).toBe(false))

      setFullWidth(targetWidth)
      await vi.waitFor(() => {
        expect(overflow.hidden).toBe(true)
        expect(bar.querySelectorAll('.doc-tab:not(.tab-hidden)')).toHaveLength(documents.length)
      })
    } finally {
      await app.unmount()
    }
  })

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

  it('extends same-name labels only until cross-workspace paths differ', async () => {
    const a = createBrowserHost('# alpha\n', 'notes.md')
    const b = createBrowserHost('# beta\n', 'notes.md')
    const workspace = createBrowserWorkspace([
      { host: a, name: 'notes.md', path: '/Research/notes/notes.md' },
      { host: b, name: 'notes.md', path: '/Music/notes/notes.md' },
    ])
    const app = await mountWorkspace(workspace)
    try {
      await vi.waitFor(() => {
        const labels = [...app.container.querySelectorAll('[role="tab"] .nm')].map((el) => el.textContent)
        expect(labels).toEqual(['notes.md — Research/notes', 'notes.md — Music/notes'])
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

  it('shows a failed native command in the captured document window', async () => {
    const a = createBrowserHost('# A\n', 'a.md')
    const b = createBrowserHost('# B\n', 'b.md')
    const workspace = createBrowserWorkspace([a, b])
    const app = await mountWorkspace(workspace)
    try {
      await workspace.activate(b.id)
      a.patchState({ actionError: 'that file is already open in another document' })
      await vi.waitFor(() => {
        const banner = app.container.querySelector(
          `[data-document-id="${a.id}"] [data-testid="doc-action-error"]`,
        )
        if (banner?.textContent !== 'that file is already open in another document') {
          throw new Error('action error banner missing on the captured document')
        }
      })
      expect(
        app.container.querySelector(`[data-document-id="${b.id}"] [data-testid="doc-action-error"]`),
      ).toBeNull()

      a.patchState({ actionError: '' })
      await vi.waitFor(() => {
        expect(
          app.container.querySelector(
            `[data-document-id="${a.id}"] [data-testid="doc-action-error"]`,
          ),
        ).toBeNull()
      })
    } finally {
      await app.unmount()
    }
  })

  it('keeps overflowing tabs inside the bar and opens them from a searchable menu', async () => {
    const documents = Array.from({ length: 10 }, (_, index) =>
      createBrowserHost(`# Document ${index}\n`, `a-very-long-document-name-${index}.md`),
    )
    const workspace = createBrowserWorkspace(documents)
    const app = await mountWorkspace(workspace, 740)
    try {
      const overflow = await vi.waitFor(() => {
        const button = app.container.querySelector<HTMLButtonElement>('[data-testid="doc-tab-overflow"]')
        if (!button || button.hidden) throw new Error('overflow menu missing')
        return button
      })
      const bar = app.container.querySelector<HTMLElement>('[data-testid="doc-tabs"]')
      const row = app.container.querySelector<HTMLElement>('.doc-tabs-row')
      const leftPane = app.container.querySelector<HTMLElement>('.workspace-body > .side-pane')
      if (!bar || !row || !leftPane) throw new Error('tab bar or left pane missing')
      expect(row.getBoundingClientRect().left).toBeGreaterThanOrEqual(leftPane.getBoundingClientRect().right - 1)
      const visibleTabs = [...bar.querySelectorAll<HTMLElement>('.doc-tab:not(.tab-hidden)')]
      expect(visibleTabs.length).toBeGreaterThan(0)
      expect(visibleTabs.length).toBeLessThan(documents.length)
      for (const tab of visibleTabs) {
        const tabRect = tab.getBoundingClientRect()
        const barRect = bar.getBoundingClientRect()
        expect(tabRect.left).toBeGreaterThanOrEqual(barRect.left - 1)
        expect(tabRect.right).toBeLessThanOrEqual(barRect.right + 1)
      }

      await userEvent.click(overflow)
      const menu = await vi.waitFor(() => {
        const element = document.querySelector<HTMLElement>('[data-testid="doc-tab-menu"]')
        if (!element) throw new Error('tab menu missing')
        return element
      })
      const hiddenCount = bar.querySelectorAll('.doc-tab.tab-hidden').length
      expect(menu.querySelectorAll('[role="menuitemradio"]')).toHaveLength(hiddenCount)

      const filter = menu.querySelector<HTMLInputElement>('[data-testid="doc-tab-filter"]')
      if (!filter) throw new Error('tab filter missing')
      filter.focus()
      await userEvent.keyboard('name-9')
      await vi.waitFor(() => {
        expect(menu.querySelectorAll('[role="menuitemradio"]')).toHaveLength(1)
      })
      const last = documents[documents.length - 1]
      if (!last) throw new Error('last document missing')
      const item = menu.querySelector<HTMLButtonElement>(`[data-testid="doc-tab-menu-item-${last.id}"]`)
      if (!item) throw new Error('last document missing from tab menu')
      await userEvent.click(item)
      const activeTab = app.container.querySelector<HTMLElement>(`[data-testid="doc-tab-${last.id}"]`)
      if (!activeTab) throw new Error('active document tab missing')
      const barRect = bar.getBoundingClientRect()
      for (const tab of bar.querySelectorAll<HTMLElement>('.doc-tab:not(.tab-hidden)')) {
        expect(tab.getBoundingClientRect().right).toBeLessThanOrEqual(barRect.right + 1)
      }
      await vi.waitFor(async () => {
        expect((await workspace.snapshot()).activeId).toBe(last.id)
      })
      expect(document.querySelector('[data-testid="doc-tab-menu"]')).toBeNull()
      await vi.waitFor(() => {
        expect(activeTab?.classList.contains('tab-hidden')).toBe(false)
        expect(document.activeElement).toBe(activeTab)
      })
    } finally {
      await app.unmount()
    }
  })

  it('shows a full path on hover and restores menu focus after Escape', async () => {
    const documents = Array.from({ length: 10 }, (_, index) =>
      createBrowserHost(`# Document ${index}\n`, `a-very-long-document-name-${index}.md`),
    )
    const workspace = createBrowserWorkspace(documents)
    const app = await mountWorkspace(workspace, 740)
    try {
      const tab = await vi.waitFor(() => {
        const element = app.container.querySelector<HTMLButtonElement>(`[data-testid="doc-tab-${documents[0].id}"]`)
        if (!element || element.classList.contains('tab-hidden')) throw new Error('first tab not visible')
        return element
      })
      const label = tab.querySelector<HTMLElement>('.nm')
      if (!label) throw new Error('tab label missing')
      await userEvent.hover(label)
      await vi.waitFor(() => {
        const tooltip = document.querySelector('[role="tooltip"]')
        if (tooltip?.textContent !== '/demo/a-very-long-document-name-0.md') throw new Error('full path tooltip missing')
      })
      await userEvent.keyboard('{Escape}')
      await vi.waitFor(() => expect(document.querySelector('[role="tooltip"]')).toBeNull())

      const overflow = app.container.querySelector<HTMLButtonElement>('[data-testid="doc-tab-overflow"]')
      if (!overflow) throw new Error('overflow missing')
      await userEvent.click(overflow)
      await vi.waitFor(() => {
        const input = document.querySelector<HTMLInputElement>('[data-testid="doc-tab-filter"]')
        if (document.activeElement !== input) throw new Error('menu filter not focused')
      })
      await userEvent.keyboard('{Escape}')
      await vi.waitFor(() => {
        expect(document.querySelector('[data-testid="doc-tab-menu"]')).toBeNull()
        expect(document.activeElement).toBe(overflow)
      })
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
      const list = await vi.waitFor(() => {
        const el = app.container.querySelector('[data-testid="doc-tabs"]')
        if (!el || !el.querySelector(`[data-testid="doc-tab-${b.id}"]`)) throw new Error('tabs missing')
        return el
      })
      list.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }))
      await vi.waitFor(async () => {
        expect((await workspace.snapshot()).activeId).toBe(b.id)
      })
    } finally {
      await app.unmount()
    }
  })

  it('keeps keyboard focus when ArrowRight hides the previously focused tab', async () => {
    const documents = Array.from({ length: 6 }, (_, index) =>
      createBrowserHost(`# Document ${index}\n`, `a-very-long-document-name-${index}.md`),
    )
    const workspace = createBrowserWorkspace(documents)
    const app = await mountWorkspace(workspace, 520)
    try {
      await vi.waitFor(() => {
        expect(app.container.querySelectorAll('.doc-tab')).toHaveLength(documents.length)
      })
      const first = documents[0]
      const next = documents[1]
      if (!first || !next) throw new Error('documents missing')
      const firstTab = app.container.querySelector<HTMLButtonElement>(`[data-testid="doc-tab-${first.id}"]`)
      const nextTab = app.container.querySelector<HTMLButtonElement>(`[data-testid="doc-tab-${next.id}"]`)
      if (!firstTab || !nextTab) throw new Error('document tabs missing')
      await vi.waitFor(() => expect(firstTab.classList.contains('tab-hidden')).toBe(false))

      firstTab.focus()
      await userEvent.keyboard('{ArrowRight}')

      await vi.waitFor(async () => {
        if ((await workspace.snapshot()).activeId !== next.id) throw new Error('next tab not active')
        if (!firstTab.classList.contains('tab-hidden') || nextTab.classList.contains('tab-hidden') || document.activeElement !== nextTab) {
          throw new Error(`tab state not settled: ${firstTab.className}; ${nextTab.className}; focus ${document.activeElement?.textContent}`)
        }
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
