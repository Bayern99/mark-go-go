import '../src/styles.css'
import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'
import { page } from 'vitest/browser'
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

function treeSeed(root = '/work') {
  const file = (path: string, name: string, parent: string, depth: number) => {
    const text = `# ${name}\n\nBody.\n`
    const meta = fileMeta(text)
    return {
      path,
      name,
      parent,
      depth,
      text,
      title: meta.title,
      excerpt: meta.excerpt,
      notes: meta.notes,
      modifiedAt: 0,
      headings: meta.headings,
    }
  }
  return {
    files: [
      file(`${root}/alpha.md`, 'alpha.md', root, 0),
      file(`${root}/beta.md`, 'beta.md', root, 0),
      file(`${root}/notes/alpha/notes.md`, 'notes.md', `${root}/notes/alpha`, 2),
      file(`${root}/notes/beta/notes.md`, 'notes.md', `${root}/notes/beta`, 2),
    ],
    folders: [
      { path: `${root}/notes`, name: 'notes', parent: root, depth: 0, markdownCount: 2 },
      { path: `${root}/notes/alpha`, name: 'alpha', parent: `${root}/notes`, depth: 1, markdownCount: 1 },
      { path: `${root}/notes/beta`, name: 'beta', parent: `${root}/notes`, depth: 1, markdownCount: 1 },
    ],
    status: 'ready' as const,
  }
}

function typeInto(input: HTMLInputElement, value: string) {
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set?.call(input, value)
  input.dispatchEvent(new Event('input', { bubbles: true }))
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
    it('restores the file tree and each document outline to where the reader left them', async () => {
      const seed = navSeed()
      const files = Array.from({ length: 90 }, (_, i) => ({
        ...seed.files[0], name: 'file-' + i + '.md', path: '/work/file-' + i + '.md',
      }))
      const long = createBrowserHost(files.map((_, i) => '# Heading ' + i).join('\n\n'), 'long.md')
      const short = createBrowserHost('# Short document', 'short.md')
      const workspace = createBrowserWorkspace([long, short], { root: '/work', nav: { files } })
      const app = await mountNav(workspace)
      const viewport = (selector: string) => app.container.querySelector<HTMLElement>(selector + ' [data-overlayscrollbars-viewport]')!
      try {
        await vi.waitFor(() => expect(viewport('[role="tree"]').scrollHeight).toBeGreaterThan(1000))
        viewport('[role="tree"]').scrollTop = 360
        await page.getByRole('tab', { name: 'Outline' }).click()
        await vi.waitFor(() => expect(app.container.querySelector('.outline-count')?.textContent).toBe('90 headings'))
        viewport('.outline-scroll').scrollTop = 600
        await new Promise<void>(resolve => requestAnimationFrame(() => resolve()))
        await workspace.activate(short.id)
        await vi.waitFor(() => expect(app.container.querySelector('.outline-link')?.textContent).toBe('Short document'))
        await workspace.activate(long.id)
        await vi.waitFor(() => expect(viewport('.outline-scroll').scrollTop).toBe(600))
        await page.getByRole('tab', { name: 'Workspace' }).click()
        await vi.waitFor(() => expect(viewport('[role="tree"]').scrollTop).toBe(360))
        await page.getByRole('tab', { name: 'Outline' }).click()
        await vi.waitFor(() => expect(viewport('.outline-scroll').scrollTop).toBe(600))
      } finally {
        await app.unmount()
      }
    })

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
          expect(app.container.querySelector('[data-testid="welcome"]')).toBeNull()
        } finally {
          await app.unmount()
        }
      },
    )

    it('keeps every folder directly above its descendants in depth-first order',
      async () => {
        const workspace = createBrowserWorkspace([], {
          root: '/work',
          name: 'work',
          kind: 'folder',
          nav: treeSeed(),
        })
        const app = await mountNav(workspace)
        try {
          const ids = await vi.waitFor(() => {
            const rows = Array.from(
              app.container.querySelectorAll<HTMLElement>('[data-testid="workspace-tree"] [role="treeitem"]'),
            )
            if (rows.length !== 7) throw new Error(`rows ${rows.length}`)
            return rows.map((row) => row.dataset.navId)
          })
          expect(ids).toEqual([
            'folder:/work/notes',
            'folder:/work/notes/alpha',
            'file:/work/notes/alpha/notes.md',
            'folder:/work/notes/beta',
            'file:/work/notes/beta/notes.md',
            'file:/work/alpha.md',
            'file:/work/beta.md',
          ])
        } finally {
          await app.unmount()
        }
      },
    )

    it('collapses a folder and walks into its children with arrow keys',
      async () => {
        const workspace = createBrowserWorkspace([], {
          root: '/work',
          name: 'work',
          kind: 'folder',
          nav: treeSeed(),
        })
        const app = await mountNav(workspace)
        const key = (row: HTMLElement, name: string) => {
          flushSync(() => row.dispatchEvent(new KeyboardEvent('keydown', { key: name, bubbles: true })))
        }
        try {
          const folder = await vi.waitFor(() => {
            const el = app.container.querySelector<HTMLButtonElement>('[data-testid="folder-notes"]')
            if (!el) throw new Error('folder missing')
            return el
          })
          flushSync(() => folder.click())
          await vi.waitFor(() => {
            if (app.container.querySelector('[data-nav-id="folder:/work/notes/alpha"]')) {
              throw new Error('collapsed folder still lists children')
            }
          })
          expect(folder.getAttribute('aria-expanded')).toBe('false')

          flushSync(() => folder.focus())
          key(folder, 'ArrowRight')
          await vi.waitFor(() => {
            if (!app.container.querySelector('[data-nav-id="folder:/work/notes/alpha"]')) {
              throw new Error('expand did not reveal children')
            }
          })
          expect(folder.getAttribute('aria-expanded')).toBe('true')

          key(folder, 'ArrowDown')
          expect(document.activeElement?.getAttribute('data-nav-id')).toBe('folder:/work/notes/alpha')
          key(document.activeElement as HTMLElement, 'ArrowDown')
          expect(document.activeElement?.getAttribute('data-nav-id')).toBe('file:/work/notes/alpha/notes.md')
          key(document.activeElement as HTMLElement, 'ArrowLeft')
          expect(document.activeElement?.getAttribute('data-nav-id')).toBe('folder:/work/notes/alpha')
          key(document.activeElement as HTMLElement, 'ArrowLeft')
          await vi.waitFor(() => {
            if (app.container.querySelector('[data-nav-id="file:/work/notes/alpha/notes.md"]')) {
              throw new Error('ArrowLeft did not collapse the parent')
            }
          })
          expect(document.activeElement?.getAttribute('data-nav-id')).toBe('folder:/work/notes/alpha')
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

    it('reveals the heading sent by the host before the target editor is ready', async () => {
      const host = createBrowserHost(AUDIT, 'audit.md')
      const nested = createBrowserHost(NESTED, 'doc.md')
      const load = nested.load
      let finishLoad!: () => void
      const loading = new Promise<void>((resolve) => { finishLoad = resolve })
      nested.load = async () => { await loading; return load() }
      const workspace = createBrowserWorkspace([
        { host, name: 'audit.md', path: '/work/audit.md' },
        { host: nested, name: 'doc.md', path: '/work/notes/deep/doc.md' },
      ], {
        root: '/work',
        kind: 'folder',
        nav: navSeed(),
      })
      let reveal: ((detail: { id: string; offset: number }) => void) | undefined
      Object.assign(workspace, {
        subscribeReveal(fn: typeof reveal) {
          reveal = fn
          return { dispose() { reveal = undefined } }
        },
      })
      const app = await mountNav(workspace)
      try {
        const target = await workspace.open('/work/notes/deep/doc.md')
        reveal?.({ id: target.id, offset: NESTED.indexOf('## Findings') })
        finishLoad()
        await vi.waitFor(() => {
          expect(window.getSelection()?.anchorNode?.parentElement?.closest('h2')?.textContent).toBe('Findings')
        })
      } finally {
        await app.unmount()
      }
    })

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
          const input = await vi.waitFor(() => {
            const el = app.container.querySelector<HTMLInputElement>('input[aria-label="Filter files and headings"]')
            if (!el) throw new Error('filter missing')
            return el
          })
          flushSync(() => typeInto(input, 'Live Heading'))
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
          const input = await vi.waitFor(() => {
            const el = app.container.querySelector<HTMLInputElement>('input[aria-label="Filter files and headings"]')
            if (!el) throw new Error('filter missing')
            return el
          })
          flushSync(() => typeInto(input, 'zzzz'))
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
          await vi.waitFor(() => {
            expect(app.container.querySelector('[data-testid="recent-workspaces"]')?.textContent).toContain('work')
          })
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
