import '../src/styles.css'
import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { userEvent } from 'vitest/browser'
import { App } from '../src/App'
import {
  createBrowserHost,
  createBrowserWorkspace,
  wrapHostAsWorkspace,
} from '../src/host/browserHost'
import type { WorkspaceAdapter } from '../src/host/types'
import { MOD } from './helpers/platform'

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

afterEach(() => {
  window.localStorage.clear()
})

async function waitForPanes(container: HTMLElement, count: number): Promise<void> {
  await vi.waitFor(
    () => {
      const modes = [...container.querySelectorAll<HTMLButtonElement>('[data-testid="view-mode-genui"]')]
      const editors = container.querySelectorAll('[data-testid="editor-root"] .ProseMirror')
      if (modes.length < count || editors.length < count) throw new Error('document panes not ready')
      if (modes.some((mode) => mode.disabled)) throw new Error('document pane not ready')
    },
    { timeout: 5000 },
  )
}

async function enterSourceMode(container: HTMLElement): Promise<void> {
  await userEvent.click(container.querySelector<HTMLButtonElement>('[data-testid="view-mode-markdown"]')!)
  await vi.waitFor(() => {
    if (!container.querySelector('.cm-content')) throw new Error('source editor missing')
  })
}

describe('restored bytes survive close and quit',
  () => {
    it('closes with the bytes the host loaded when nobody edited them',
      async () => {
        const alpha = createBrowserHost('# Alpha\n\nA body.\n', 'alpha.md')
        const beta = createBrowserHost('# Beta\n\nB body.\n', 'beta.md')
        const workspace = createBrowserWorkspace([alpha, beta], {
          confirmCloseWorkspace: async () => ({ closed: false, canceled: true }),
        })
        const app = await mountWorkspace(workspace)
        try {
          await waitForPanes(app.container, 2)
          await workspace.closeWorkspace()
          expect((await alpha.load()).text).toBe('# Alpha\n\nA body.\n')
          expect((await beta.load()).text).toBe('# Beta\n\nB body.\n')
        } finally {
          await app.unmount()
        }
      },
    )

    it('keeps a restored GenUI document byte for byte when it is closed',
      async () => {
        const host = createBrowserHost('# Hello\n\nBody.\n', 'notes.md')
        await host.setViewMode('genui')
        const workspace = createBrowserWorkspace([host], {
          root: '/work',
          name: 'work',
          kind: 'folder',
          confirmClose: async () => ({ closed: false, canceled: true }),
        })
        const app = await mountWorkspace(workspace)
        try {
          await waitForPanes(app.container, 1)
          await workspace.closeDocument(host.id)
          expect((await host.load()).text).toBe('# Hello\n\nBody.\n')
        } finally {
          await app.unmount()
        }
      },
    )

    it('leaves bytes alone while no pane has mounted its editor',
      async () => {
        const host = createBrowserHost('# Unmounted\n', 'quiet.md')
        const workspace = createBrowserWorkspace([host], {
          confirmCloseWorkspace: async () => ({ closed: false, canceled: true }),
        })
        await workspace.closeWorkspace()
        expect((await host.load()).text).toBe('# Unmounted\n')
      },
    )

    it('delivers a deliberate empty document instead of stale bytes',
      async () => {
        const host = createBrowserHost('# Clear me\n\nBody.\n', 'clear.md')
        const workspace = createBrowserWorkspace([host], {
          confirmClose: async () => ({ closed: false, canceled: true }),
        })
        const app = await mountWorkspace(workspace)
        try {
          await waitForPanes(app.container, 1)
          await enterSourceMode(app.container)
          await userEvent.click(app.container.querySelector<HTMLElement>('.cm-content')!)
          await userEvent.keyboard(`{${MOD}>}a{/${MOD}}{Backspace}`)
          await workspace.closeDocument(host.id)
          expect((await host.load()).text).toBe('')
        } finally {
          await app.unmount()
        }
      },
    )
  },
)

describe('view mode and restore transport',
  () => {
    it('records setViewMode on the workspace snapshot, including untitled documents',
      async () => {
        const saved = createBrowserHost('# Saved\n', 'notes.md')
        const untitled = createBrowserHost('# Draft\n', 'Untitled.md', { untitled: true })
        const workspace = createBrowserWorkspace([saved, untitled])
        await saved.setViewMode('markdown')
        await untitled.setViewMode('genui')
        const snap = await workspace.snapshot()
        const untitledInfo = snap.documents.find((doc) => doc.id === untitled.id)
        const savedInfo = snap.documents.find((doc) => doc.id === saved.id)
        expect(savedInfo?.viewMode).toBe('markdown')
        expect(untitledInfo?.path).toBe('')
        expect(untitledInfo?.viewMode).toBe('genui')

        const wrapped = wrapHostAsWorkspace(untitled)
        expect((await wrapped.snapshot()).documents[0].viewMode).toBe('genui')
        await untitled.setViewMode('preview')
        expect((await wrapped.snapshot()).documents[0].viewMode).toBe('preview')
        expect((await wrapped.snapshot()).documents[0].path).toBe('')
      },
    )

    it('wrapHostAsWorkspace still binds one document stably',
      () => {
        const host = createBrowserHost('# A\n', 'a.md')
        const workspace = wrapHostAsWorkspace(host, 'a.md')
        expect(workspace.document(host.id)).toBe(workspace.document(host.id))
        expect(workspace.document(host.id)).toBe(host)
      },
    )

    it('cancel close keeps both documents and the adapter cache',
      async () => {
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
      },
    )

    it('restores genui from DocumentInfo.viewMode without localStorage',
      async () => {
        const host = createBrowserHost('# Hello\n\nBody.\n', 'notes.md')
        window.localStorage.setItem(`mgo.viewMode:${host.getState().path}`, 'markdown')
        await host.setViewMode('genui')
        const workspace = wrapHostAsWorkspace(host)
        const app = await mountWorkspace(workspace)
        try {
          await vi.waitFor(
            () => {
              const button = app.container.querySelector('[data-testid="view-mode-genui"]')
              if (button?.getAttribute('aria-checked') !== 'true') {
                throw new Error('genui not restored')
              }
            },
            { timeout: 5000 },
          )
          expect(app.container.querySelector('[data-testid="view-mode-markdown"]')?.getAttribute('aria-checked')).not.toBe(
            'true',
          )
          expect(host.generations).toEqual([])
          expect(host.getState().viewMode).toBe('genui')
        } finally {
          await app.unmount()
        }
      },
    )

    it.each(['session', 'local storage'])('restores an older %s in Preview and saves the new mode name',
      async (store) => {
        const host = createBrowserHost('# Hello\n\nBody.\n', 'legacy-preview.md')
        const storageKey = `mgo.viewMode:${host.getState().path}`
        if (store === 'session') {
          window.localStorage.setItem(storageKey, 'markdown')
          await host.setViewMode('richtext')
        } else {
          window.localStorage.setItem(storageKey, 'richtext')
        }

        const app = await mountWorkspace(wrapHostAsWorkspace(host))
        try {
          await waitForPanes(app.container, 1)
          const preview = app.container.querySelector<HTMLButtonElement>('[data-testid="view-mode-preview"]')!
          expect(preview.getAttribute('aria-checked')).toBe('true')
          expect(app.container.querySelector('.mgo-preview-root')?.hasAttribute('hidden')).toBe(false)
          await vi.waitFor(() => expect(window.localStorage.getItem(storageKey)).toBe('preview'))

          await enterSourceMode(app.container)
          await vi.waitFor(() => expect(host.getState().viewMode).toBe('markdown'))
          await userEvent.click(preview)
          await vi.waitFor(() => expect(host.getState().viewMode).toBe('preview'))
        } finally {
          await app.unmount()
        }
      },
    )

    it('writes the mode the user picks to that document',
      async () => {
        const host = createBrowserHost('# Hello\n\nBody.\n', 'notes.md')
        const app = await mountWorkspace(wrapHostAsWorkspace(host))
        try {
          const button = await vi.waitFor(() => {
            const el = app.container.querySelector<HTMLButtonElement>('[data-testid="view-mode-genui"]')
            if (!el || el.disabled) throw new Error('view mode choice not ready')
            return el
          })
          button.click()
          await vi.waitFor(() => expect(host.getState().viewMode).toBe('genui'), { timeout: 5000 })
        } finally {
          await app.unmount()
        }
      },
    )

    it('shows workspace restoreError without turning an empty folder into Welcome',
      async () => {
        const workspace = createBrowserWorkspace([], {
          root: '/work',
          name: 'work',
          kind: 'folder',
          restoreError: 'session.json could not be read',
        })
        const app = await mountWorkspace(workspace)
        try {
          await vi.waitFor(() => {
            if (!app.container.querySelector('[data-testid="restore-error"]')) {
              throw new Error('restore error missing')
            }
          })
          expect(app.container.querySelector('[data-testid="workspace-empty"]')).not.toBeNull()
          expect(app.container.querySelector('[data-testid="welcome"]')).toBeNull()
        } finally {
          await app.unmount()
        }
      },
    )

    it('shows per-document restoreError in that pane',
      async () => {
        const host = createBrowserHost('# Hello\n', 'notes.md')
        host.patchState({ restoreError: 'this document could not be restored fully' })
        const workspace = wrapHostAsWorkspace(host)
        const app = await mountWorkspace(workspace)
        try {
          await vi.waitFor(() => {
            if (!app.container.querySelector('[data-testid="doc-restore-error"]')) {
              throw new Error('doc restore error missing')
            }
          })
        } finally {
          await app.unmount()
        }
      },
    )

    it('shows a document load failure instead of remaining on Loading',
      async () => {
        const base = createBrowserHost('# Hello\n', 'notes.md')
        const host = {
          ...base,
          async load() {
            throw new Error('disk unavailable')
          },
        }
        const app = await mountWorkspace(wrapHostAsWorkspace(host))
        try {
          await vi.waitFor(() => {
            const error = app.container.querySelector<HTMLElement>('[data-testid="document-bootstrap-error"]')
            if (!error) throw new Error('document load failure missing')
            expect(error.textContent).toContain('disk unavailable')
          })
          expect(app.container.querySelector<HTMLButtonElement>('[data-testid="view-mode-preview"]')?.disabled).toBe(true)
        } finally {
          await app.unmount()
        }
      },
    )
  },
)
