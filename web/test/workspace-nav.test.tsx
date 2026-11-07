import '../src/styles.css'
import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'
import { App } from '../src/App'
import { createBrowserHost, createBrowserWorkspace } from '../src/host/browserHost'
import { fileMeta } from '../src/workspace/nav'
import type { WorkspaceAdapter } from '../src/host/types'

const NESTED = `# Nested doc\n\nDeep file in notes.\n\n## Findings\n\nMore nested content.\n`
const AUDIT = `# Audit\n\nRoot notes for the workspace.\n\n<!-- note: keep this -->\n`

function navSeed(root = '/work') {
  const nestedPath = `${root}/notes/deep/doc.md`
  const auditPath = `${root}/audit.md`
  const nestedMeta = fileMeta(NESTED)
  const auditMeta = fileMeta(AUDIT)
  return {
    files: [
      {
        path: auditPath,
        name: 'audit.md',
        parent: root,
        depth: 0,
        text: AUDIT,
        title: auditMeta.title,
        excerpt: auditMeta.excerpt,
        notes: auditMeta.notes,
        modifiedAt: Date.parse('2026-01-02T10:00:00Z'),
        headings: auditMeta.headings,
      },
      {
        path: nestedPath,
        name: 'doc.md',
        parent: `${root}/notes/deep`,
        depth: 2,
        text: NESTED,
        title: nestedMeta.title,
        excerpt: nestedMeta.excerpt,
        notes: nestedMeta.notes,
        modifiedAt: Date.parse('2026-01-03T10:00:00Z'),
        headings: nestedMeta.headings,
      },
    ],
    folders: [
      { path: `${root}/notes`, name: 'notes', parent: root, depth: 0, markdownCount: 1 },
      { path: `${root}/notes/deep`, name: 'deep', parent: `${root}/notes`, depth: 1, markdownCount: 1 },
    ],
    status: 'ready' as const,
    recents: [{ root, name: 'work', openedAt: 1 }],
  }
}

async function mountNav(workspace: WorkspaceAdapter) {
  const container = document.createElement('div')
  container.style.height = '100%'
  document.body.append(container)
  const root = createRoot(container)
  flushSync(() => {
    root.render(<App workspace={workspace} />)
  })
  await vi.waitFor(() => {
    if (!container.querySelector('[data-testid="left-pane"]')) throw new Error('left pane missing')
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

describe('workspace navigation',
  () => {
    it('lists nested Markdown files and folder counts, not empty placeholders',
      async () => {
        const workspace = createBrowserWorkspace([], {
          root: '/work',
          name: 'work',
          kind: 'folder',
          nav: navSeed(),
        })
        const app = await mountNav(workspace)
        try {
          await vi.waitFor(() => {
            if (!app.container.querySelector('[data-testid="file-doc.md"]')) throw new Error('nested file missing')
          })
          expect(app.container.querySelector('[data-testid="file-audit.md"]')).not.toBeNull()
          expect(app.container.querySelector('[data-testid="folder-notes"]')?.textContent).toContain('1')
          expect(app.container.textContent).not.toContain('skip.txt')
          expect(app.container.querySelector('[data-testid="workspace-empty"]')).toBeNull()
          expect(app.container.querySelector('[data-testid="welcome"]')).toBeNull()
        } finally {
          await app.unmount()
        }
      },
    )

    it('opens a nested file as a tab from the tree',
      async () => {
        const workspace = createBrowserWorkspace([], {
          root: '/work',
          name: 'work',
          kind: 'folder',
          nav: navSeed(),
        })
        const app = await mountNav(workspace)
        try {
          const row = await vi.waitFor(() => {
            const el = app.container.querySelector<HTMLButtonElement>('[data-testid="file-doc.md"]')
            if (!el) throw new Error('nested file missing')
            return el
          })
          flushSync(() => row.click())
          await vi.waitFor(async () => {
            const snap = await workspace.snapshot()
            if (snap.documents.length !== 1) throw new Error(`docs ${snap.documents.length}`)
            expect(snap.documents[0].path).toBe('/work/notes/deep/doc.md')
          })
        } finally {
          await app.unmount()
        }
      },
    )

    it('filters headings including unsaved open text and keeps the document on clear',
      async () => {
        const host = createBrowserHost('# Live Heading\n\nUnsaved body.\n', 'audit.md')
        const workspace = createBrowserWorkspace([{ host, name: 'audit.md', path: '/work/audit.md' }], {
          root: '/work',
          name: 'work',
          kind: 'folder',
          nav: navSeed(),
        })
        const app = await mountNav(workspace)
        try {
          const input = app.container.querySelector<HTMLInputElement>('input[aria-label="Filter files and headings"]')
          if (!input) throw new Error('filter missing')
          flushSync(() => {
            input.value = 'Live Heading'
            input.dispatchEvent(new Event('input', { bubbles: true }))
          })
          await vi.waitFor(() => {
            const hit = app.container.querySelector('[data-testid^="heading-hit-"]')
            if (!hit || !hit.textContent?.includes('Live Heading')) throw new Error('unsaved heading missing')
          })
          expect(app.container.querySelector('[data-testid="filter-status"]')?.textContent).toMatch(/of 2 files/)
          const before = (await workspace.snapshot()).activeId
          flushSync(() => app.container.querySelector<HTMLButtonElement>('[data-testid="clear-filter"]')?.click())
          await vi.waitFor(() => {
            expect(app.container.querySelector('[data-testid="filter-results"]')).toBeNull()
          })
          expect((await workspace.snapshot()).activeId).toBe(before)
        } finally {
          await app.unmount()
        }
      },
    )

    it('does not report an empty filter while indexing',
      async () => {
        const workspace = createBrowserWorkspace([], {
          root: '/work',
          name: 'work',
          kind: 'folder',
          nav: { ...navSeed(), status: 'scanning' },
        })
        const app = await mountNav(workspace)
        try {
          const input = app.container.querySelector<HTMLInputElement>('input[aria-label="Filter files and headings"]')
          if (!input) throw new Error('filter missing')
          flushSync(() => {
            input.value = 'zzzz'
            input.dispatchEvent(new Event('input', { bubbles: true }))
          })
          await vi.waitFor(() => {
            if (!app.container.querySelector('[data-testid="filter-indexing"]')) throw new Error('indexing notice missing')
          })
          expect(app.container.querySelector('[data-testid="filter-empty"]')).toBeNull()
          expect(app.container.querySelector('[data-testid="filter-status"]')?.textContent).toMatch(/Indexing/)
        } finally {
          await app.unmount()
        }
      },
    )

    it('shows a keyboard-reachable splitter and path crumbs',
      async () => {
        const host = createBrowserHost(AUDIT, 'audit.md')
        const workspace = createBrowserWorkspace([{ host, name: 'audit.md', path: '/work/audit.md' }], {
          root: '/work',
          name: 'work',
          kind: 'folder',
          nav: navSeed(),
        })
        const app = await mountNav(workspace)
        try {
          const splitter = app.container.querySelector<HTMLElement>('[data-testid="sidebar-splitter"]')
          expect(splitter?.getAttribute('role')).toBe('separator')
          expect(splitter?.tabIndex).toBe(0)
          await vi.waitFor(() => {
            if (!app.container.querySelector('[data-testid="path-crumbs"]')) throw new Error('crumbs missing')
          })
          expect(app.container.querySelector('[data-testid="path-crumbs"]')?.textContent).toContain('audit.md')
        } finally {
          await app.unmount()
        }
      },
    )

    it('shows welcome with Open Folder, New Document and persisted recents',
      async () => {
        let picked = ''
        const workspace = createBrowserWorkspace([], {
          root: '',
          name: 'Untitled',
          kind: 'untitled',
          nav: {
            recents: [{ root: '/work', name: 'work', openedAt: 1 }],
            pickFolder: async () => {
              picked = '/picked'
              return '/picked'
            },
          },
        })
        const app = await mountNav(workspace)
        try {
          expect(app.container.querySelector('[data-testid="welcome"]')).not.toBeNull()
          expect(app.container.querySelector('[data-testid="open-folder"]')).not.toBeNull()
          expect(app.container.querySelector('[data-testid="new-document"]')).not.toBeNull()
          expect(app.container.querySelector('[data-testid="recent-workspaces"]')?.textContent).toContain('work')
          flushSync(() => app.container.querySelector<HTMLButtonElement>('[data-testid="open-folder"]')?.click())
          await vi.waitFor(() => {
            expect(picked).toBe('/picked')
          })
        } finally {
          await app.unmount()
        }
      },
    )

    it('keeps a folder workspace empty state with the file tree, not the welcome page',
      async () => {
        const workspace = createBrowserWorkspace([], {
          root: '/work',
          name: 'work',
          kind: 'folder',
          nav: navSeed(),
        })
        const app = await mountNav(workspace)
        try {
          await vi.waitFor(() => {
            expect(app.container.querySelector('[data-testid="workspace-empty"]')).not.toBeNull()
          })
          expect(app.container.querySelector('[data-testid="welcome"]')).toBeNull()
          expect(app.container.querySelector('[data-testid="workspace-tree"]')).not.toBeNull()
        } finally {
          await app.unmount()
        }
      },
    )
  },
)
