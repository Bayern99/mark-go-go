import '../src/styles.css'
import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
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

afterEach(() => {
  window.localStorage.clear()
})

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
        await untitled.setViewMode('richtext')
        expect((await wrapped.snapshot()).documents[0].viewMode).toBe('richtext')
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
          // Restoring the mode must not write the pre-restore default back.
          expect(host.getState().viewMode).toBe('genui')
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
  },
)
