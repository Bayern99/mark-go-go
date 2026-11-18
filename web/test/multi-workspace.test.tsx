import '../src/styles.css'
import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'
import { userEvent } from 'vitest/browser'
import { App } from '../src/App'
import { createBrowserHost, createBrowserWorkspace } from '../src/host/browserHost'
import type { BrowserNavSeed } from '../src/workspace/nav'

function oneFileNav(root: string, name: string): BrowserNavSeed {
  return {
    files: [{
      path: `${root}/${name}`,
      name,
      parent: root,
      depth: 0,
      text: `# ${name}\n`,
      title: name,
      excerpt: '',
      notes: 0,
      modifiedAt: 0,
      headings: [],
    }],
    folders: [],
    status: 'ready',
  }
}

async function mount(workspace: ReturnType<typeof createBrowserWorkspace>) {
  const container = document.createElement('div')
  container.style.height = '100%'
  document.body.append(container)
  const root = createRoot(container)
  flushSync(() => root.render(<App workspace={workspace} />))
  await vi.waitFor(() => {
    if (!container.querySelector('[data-testid="workspace-switcher"]')) throw new Error('workspace switcher missing')
  })
  return {
    container,
    async unmount() {
      root.unmount()
      container.remove()
      await new Promise<void>((resolve) => setTimeout(resolve, 0))
    },
  }
}

describe('multiple workspaces in one window', () => {
  it('uses one tab group and preserves both mounted panes across repeated switches', async () => {
    const discussion = createBrowserHost('# Discussion\n', 'Discussion.md')
    const score = createBrowserHost('# CC129\n', 'CC129.md')
    const workspace = createBrowserWorkspace([
      { host: discussion, path: '/Research/Discussion.md', workspaceId: 'research' },
      { host: score, path: '/Music Department/CC129.md', workspaceId: 'music' },
    ], {
      workspaces: [
        { id: 'research', root: '/Research', name: 'Research', nav: oneFileNav('/Research', 'Discussion.md') },
        { id: 'music', root: '/Music Department', name: 'Music Department', nav: oneFileNav('/Music Department', 'CC129.md') },
      ],
    })
    const app = await mount(workspace)
    try {
      await vi.waitFor(() => expect(app.container.querySelectorAll('[data-document-id]')).toHaveLength(2))
      const paneA = app.container.querySelector(`[data-document-id="${discussion.id}"]`)
      const paneB = app.container.querySelector(`[data-document-id="${score.id}"]`)
      await discussion.update('# Discussion edited\n')
      await workspace.activate(score.id)
      await vi.waitFor(async () => expect((await workspace.snapshot()).root).toBe('/Music Department'))
      await workspace.activate(discussion.id)
      await workspace.activate(score.id)
      await workspace.activate(discussion.id)
      await vi.waitFor(async () => expect((await workspace.snapshot()).root).toBe('/Research'))
      expect((await discussion.load()).text).toBe('# Discussion edited\n')
      expect(app.container.querySelector(`[data-document-id="${discussion.id}"]`)).toBe(paneA)
      expect(app.container.querySelector(`[data-document-id="${score.id}"]`)).toBe(paneB)
      expect(paneA?.hasAttribute('hidden')).toBe(false)
      expect(paneB?.hasAttribute('hidden')).toBe(true)
    } finally {
      await app.unmount()
    }
  })

  it('switches recent documents with Ctrl+Tab', async () => {
    const a = createBrowserHost('# A\n', 'a.md')
    const b = createBrowserHost('# B\n', 'b.md')
    const workspace = createBrowserWorkspace([
      { host: a, path: '/Research/a.md', workspaceId: 'research' },
      { host: b, path: '/Music/b.md', workspaceId: 'music' },
    ], {
      workspaces: [
        { id: 'research', root: '/Research', name: 'Research' },
        { id: 'music', root: '/Music', name: 'Music' },
      ],
    })
    const app = await mount(workspace)
    try {
      await vi.waitFor(() => {
        expect(app.container.querySelector(`[data-testid="doc-tab-${a.id}"]`)?.getAttribute('aria-selected')).toBe('true')
      })
      await workspace.activate(b.id)
      await vi.waitFor(() => {
        expect(app.container.querySelector(`[data-testid="doc-tab-${b.id}"]`)?.getAttribute('aria-selected')).toBe('true')
      })
      await userEvent.keyboard('{Control>}{Tab}{/Control}')
      await vi.waitFor(() => {
        expect(app.container.querySelector(`[data-testid="doc-tab-${a.id}"]`)?.getAttribute('aria-selected')).toBe('true')
      })
      await userEvent.keyboard('{Control>}{Tab}{/Control}')
      await vi.waitFor(async () => expect((await workspace.snapshot()).activeId).toBe(b.id))
    } finally {
      await app.unmount()
    }
  })

  it('selects roots from the existing workspace name and shows an empty root without unmounting tabs', async () => {
    const a = createBrowserHost('# A\n', 'a.md')
    const workspace = createBrowserWorkspace([{ host: a, path: '/Research/a.md', workspaceId: 'research' }], {
      workspaces: [
        { id: 'research', root: '/Research', name: 'Research', nav: oneFileNav('/Research', 'a.md') },
        { id: 'empty', root: '/Empty', name: 'Empty' },
      ],
    })
    const app = await mount(workspace)
    try {
      await userEvent.click(app.container.querySelector<HTMLElement>('[data-testid="workspace-switcher"]')!)
      const item = await vi.waitFor(() => {
        const element = document.querySelector<HTMLElement>('[data-testid="workspace-menu-item-empty"]')
        if (!element) throw new Error('empty workspace missing')
        return element
      })
      await userEvent.click(item)
      await vi.waitFor(() => {
        expect(app.container.querySelector('[data-testid="workspace-empty"]')).not.toBeNull()
      })
      expect(app.container.querySelector(`[data-document-id="${a.id}"]`)).not.toBeNull()
      expect((await workspace.snapshot()).documents).toHaveLength(1)
      expect((await workspace.snapshot()).activeId).toBe('')
    } finally {
      await app.unmount()
    }
  })

  it('keeps a canceled root close intact and closes only that root on confirmation', async () => {
    const a = createBrowserHost('# A\n', 'a.md')
    const b = createBrowserHost('# B\n', 'b.md')
    let cancel = true
    const workspace = createBrowserWorkspace([
      { host: a, path: '/Research/a.md', workspaceId: 'research' },
      { host: b, path: '/Music/b.md', workspaceId: 'music' },
    ], {
      workspaces: [
        { id: 'research', root: '/Research', name: 'Research' },
        { id: 'music', root: '/Music', name: 'Music' },
      ],
      confirmClose: () => ({ closed: !cancel, canceled: cancel }),
    })
    const canceled = await workspace.closeWorkspaceRoot('research')
    expect(canceled.canceled).toBe(true)
    expect((await workspace.snapshot()).documents).toHaveLength(2)
    expect((await workspace.snapshot()).workspaces).toHaveLength(2)

    cancel = false
    const closed = await workspace.closeWorkspaceRoot('research')
    expect(closed.closed).toBe(true)
    const snap = await workspace.snapshot()
    expect(snap.documents.map((doc) => doc.id)).toEqual([b.id])
    expect(snap.workspaces.map((root) => root.id)).toEqual(['music'])
    expect(snap.activeId).toBe(b.id)
  })

  it('closes the selected root from the workspace menu', async () => {
    const a = createBrowserHost('# A\n', 'a.md')
    const b = createBrowserHost('# B\n', 'b.md')
    const workspace = createBrowserWorkspace([
      { host: a, path: '/Research/a.md', workspaceId: 'research' },
      { host: b, path: '/Music/b.md', workspaceId: 'music' },
    ], {
      workspaces: [
        { id: 'research', root: '/Research', name: 'Research' },
        { id: 'music', root: '/Music', name: 'Music' },
      ],
    })
    const app = await mount(workspace)
    try {
      await vi.waitFor(() => expect(app.container.querySelector(`[data-testid="doc-tab-${a.id}"]`)).not.toBeNull())
      await userEvent.click(app.container.querySelector<HTMLElement>('[data-testid="workspace-switcher"]')!)
      const close = await vi.waitFor(() => {
        const element = document.querySelector<HTMLElement>('[data-testid="workspace-close"]')
        if (!element) throw new Error('close workspace action missing')
        return element
      })
      await userEvent.click(close)
      await vi.waitFor(async () => {
        expect((await workspace.snapshot()).documents.map((doc) => doc.id)).toEqual([b.id])
      })
      expect((await workspace.snapshot()).workspaces.map((root) => root.id)).toEqual(['music'])
    } finally {
      await app.unmount()
    }
  })

  it('keeps filters scoped to their own roots', async () => {
    const workspace = createBrowserWorkspace([], {
      workspaces: [
        { id: 'research', root: '/Research', name: 'Research', nav: oneFileNav('/Research', 'Discussion.md') },
        { id: 'music', root: '/Music', name: 'Music', nav: oneFileNav('/Music', 'CC129.md') },
      ],
    })
    await workspace.setFilter('Discussion')
    await workspace.activateWorkspace('music')
    expect((await workspace.setFilter('')).hits).toEqual([])
    await workspace.activateWorkspace('research')
    expect((await workspace.setFilter('Discussion')).matched).toBe(1)
    expect((await workspace.navSnapshot()).root).toBe('/Research')
  })
})
