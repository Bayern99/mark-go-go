import '../src/styles.css'
import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { userEvent } from 'vitest/browser'
import { describe, expect, it, vi } from 'vitest'
import { App } from '../src/App'
import {
  createBrowserHost,
  createBrowserWorkspace,
  type BrowserHost,
} from '../src/host/browserHost'
import type { WorkspaceAdapter } from '../src/host/types'

const DRAFT_NOTE = 'A draft note'
const DRAFT_BLOCK = '> [!NOTE]\n> A changed block'
const DOC_A = `# Alpha\n\nBody A.\n\n<!-- note: A note -->\n\n> [!WARNING]\n> A locked\n`
const DOC_B = `# Beta\n\nBody B.\n\n<!-- note: B note -->\n\n> [!WARNING]\n> B locked\n`

async function mountWorkspace(
  workspace: WorkspaceAdapter,
  hosts: BrowserHost[],
): Promise<{ container: HTMLDivElement; unmount(): Promise<void> }> {
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
      for (const host of hosts) {
        const pane = container.querySelector<HTMLElement>(`[data-document-id="${host.id}"]`)
        if (!pane) throw new Error(`pane missing for ${host.id}`)
        const mode = pane.querySelector<HTMLButtonElement>('[data-testid="view-mode-preview"]')
        if (!mode || mode.disabled) throw new Error(`document pane ${host.id} not ready`)
      }
    },
    { timeout: 8000 },
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

function paneOf(container: HTMLElement, host: BrowserHost): HTMLElement {
  const pane = container.querySelector<HTMLElement>(`[data-document-id="${host.id}"]`)
  if (!pane) throw new Error(`pane missing for ${host.id}`)
  return pane
}

async function openNoteComposer(
  pane: HTMLElement,
): Promise<{ composer: HTMLElement; input: HTMLTextAreaElement }> {
  const edit = await vi.waitFor(() => {
    const button = pane.querySelector<HTMLButtonElement>('[data-note] [data-note-action="edit"]')
    if (!button) throw new Error('no Note Edit action')
    return button
  })
  await userEvent.click(edit)
  return composerParts(pane, 'mgo-note-composer', 'Note')
}

async function openBlockComposer(
  pane: HTMLElement,
): Promise<{ composer: HTMLElement; input: HTMLTextAreaElement }> {
  const edit = await vi.waitFor(() => {
    const button = pane.querySelector<HTMLButtonElement>('.mgo-alert [data-block-action="edit"]')
    if (!button) throw new Error('no block Edit action')
    return button
  })
  await userEvent.click(edit)
  return composerParts(pane, 'mgo-block-composer', 'Markdown')
}

async function composerParts(
  pane: HTMLElement,
  className: string,
  inputLabel: string,
): Promise<{ composer: HTMLElement; input: HTMLTextAreaElement }> {
  const composer = await vi.waitFor(() => {
    const found = pane.querySelector<HTMLElement>(`.composer.${className}`)
    if (!found) throw new Error(`no ${className}`)
    return found
  })
  const input = composer.querySelector<HTMLTextAreaElement>(
    `textarea[aria-label="${inputLabel}"]`,
  )
  if (!input) throw new Error(`no ${inputLabel} textarea`)
  return { composer, input }
}

async function saveComposer(pane: HTMLElement, composer: HTMLElement): Promise<void> {
  const save = [...composer.querySelectorAll<HTMLButtonElement>('.composer-foot button')].find(
    (button) => button.textContent === 'Save',
  )
  if (!save) throw new Error('no Save action')
  await userEvent.click(save)
  await vi.waitFor(() => expect(composer.isConnected).toBe(false))
  expect(pane.querySelector('.mgo-note-composer, .mgo-block-composer')).toBeNull()
}

describe('per-document composer isolation', () => {
  it("keeps A's draft Note through B's reload, read-only switch and close, then saves A's text only", async () => {
    const a = createBrowserHost(DOC_A, 'a.md')
    const b = createBrowserHost(DOC_B, 'b.md')
    const workspace = createBrowserWorkspace([a, b])
    const app = await mountWorkspace(workspace, [a, b])
    try {
      const paneA = paneOf(app.container, a)
      const { composer, input } = await openNoteComposer(paneA)
      await userEvent.fill(input, DRAFT_NOTE)

      await workspace.activate(b.id)

      b.reload(DOC_B.replace('B locked', 'B locked, reloaded'))
      b.patchState({ readOnly: true })
      await workspace.closeDocument(b.id)

      await vi.waitFor(() =>
        expect(app.container.querySelector(`[data-document-id="${b.id}"]`)).toBeNull(),
      )
      expect(paneA.querySelector('.mgo-note-composer')).toBe(composer)
      expect(input.value).toBe(DRAFT_NOTE)

      await workspace.activate(a.id)
      await saveComposer(paneA, composer)
      expect(paneA.querySelector('[data-note] .mgo-note-text')?.textContent).toBe(DRAFT_NOTE)

      await workspace.closeDocument(a.id)
      expect((await a.load()).text).toBe(
        `# Alpha\n\nBody A.\n\n<!-- note: ${DRAFT_NOTE} -->\n\n> [!WARNING]\n> A locked\n`,
      )
    } finally {
      await app.unmount()
    }
  })

  it("keeps A's draft raw block through B's reload, read-only switch and close, then writes only that block", async () => {
    const a = createBrowserHost(DOC_A, 'a.md')
    const b = createBrowserHost(DOC_B, 'b.md')
    const workspace = createBrowserWorkspace([a, b])
    const app = await mountWorkspace(workspace, [a, b])
    try {
      const paneA = paneOf(app.container, a)
      const { composer, input } = await openBlockComposer(paneA)
      expect(input.value).toBe('> [!WARNING]\n> A locked')
      await userEvent.fill(input, DRAFT_BLOCK)

      await workspace.activate(b.id)
      b.reload(DOC_B.replace('B locked', 'B locked, reloaded'))
      b.patchState({ readOnly: true })
      await workspace.closeDocument(b.id)

      expect(paneA.querySelector('.mgo-block-composer')).toBe(composer)
      expect(input.value).toBe(DRAFT_BLOCK)

      await workspace.activate(a.id)
      await saveComposer(paneA, composer)
      expect(paneA.querySelector('.mgo-alert.note')?.textContent).toContain('A changed block')

      await workspace.closeDocument(a.id)
      expect((await a.load()).text).toBe(
        `# Alpha\n\nBody A.\n\n<!-- note: A note -->\n\n${DRAFT_BLOCK}\n`,
      )
    } finally {
      await app.unmount()
    }
  })

  it("closes B's own composer on B's reload, read-only switch and close", async () => {
    const a = createBrowserHost(DOC_A, 'a.md')
    const b = createBrowserHost(DOC_B, 'b.md')
    const workspace = createBrowserWorkspace([a, b])
    const app = await mountWorkspace(workspace, [a, b])
    try {
      await workspace.activate(b.id)
      const paneB = paneOf(app.container, b)

      const first = await openNoteComposer(paneB)
      await userEvent.fill(first.input, 'B draft')
      b.reload(DOC_B.replace('B locked', 'B locked, again'))
      await vi.waitFor(() => expect(first.composer.isConnected).toBe(false))

      const second = await openNoteComposer(paneB)
      await userEvent.fill(second.input, 'B draft 2')
      b.patchState({ readOnly: true })
      await vi.waitFor(() => expect(second.composer.isConnected).toBe(false))

      b.patchState({ readOnly: false })
      await vi.waitFor(() => {
        const view = paneB.querySelector('.ProseMirror')
        if (view?.getAttribute('contenteditable') !== 'true') throw new Error('B is still read only')
      })
      b.reload(DOC_B.replace('B locked', 'B locked, once more'))
      const third = await openNoteComposer(paneB)
      await userEvent.fill(third.input, 'B draft 3')
      await workspace.closeDocument(b.id)

      await vi.waitFor(() => expect(third.composer.isConnected).toBe(false))
      expect(app.container.querySelector('.mgo-note-composer')).toBeNull()
      expect(app.container.querySelector(`[data-document-id="${b.id}"]`)).toBeNull()
    } finally {
      await app.unmount()
    }
  })
})
