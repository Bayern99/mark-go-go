import '../src/styles.css'
import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { userEvent } from 'vitest/browser'
import { describe, expect, it, vi } from 'vitest'
import { App } from '../src/App'
import {
  applyNativeEdit,
  nativeComposerHasFocus,
  nativeEditCapabilities,
  nativeEditTarget,
  registerNativeEditTarget,
} from '../src/editor/nativeEdit'
import { createBrowserHost, createBrowserWorkspace } from '../src/host/browserHost'
import formatDoc from '../src/plugins/genui/view-format.md?raw'
import { mountApp } from './helpers/app'
import { mountEditor, replaceParagraph } from './helpers/editor'
import { getSourceView, replaceSourceText, visibleSource } from './helpers/source'

const RICH = 'First paragraph.\n\nSecond paragraph.\n'

function paragraph(container: HTMLElement, text: string): HTMLElement {
  const found = Array.from(container.querySelectorAll<HTMLElement>('.ProseMirror p')).find((el) =>
    (el.textContent ?? '').includes(text),
  )
  if (!found) throw new Error(`paragraph not found: ${text}`)
  return found
}

async function waitForTarget(id: string): Promise<void> {
  await vi.waitFor(
    () => {
      if (!nativeEditTarget(id)) throw new Error(`native edit target missing: ${id}`)
    },
    { timeout: 5000 },
  )
}

describe('MarkdownEditor history', () => {
  it('undoes and redoes a rich-text edit through the public editor API', async () => {
    const mounted = await mountEditor(RICH)
    try {
      expect(mounted.editor.canUndo()).toBe(false)
      expect(mounted.editor.canRedo()).toBe(false)
      replaceParagraph(mounted.editor, 'First paragraph.', 'First paragraph, edited.')
      expect(mounted.editor.getMarkdown()).toContain('First paragraph, edited.')
      expect(mounted.editor.canUndo()).toBe(true)
      expect(mounted.editor.canRedo()).toBe(false)

      expect(mounted.editor.undo()).toBe(true)
      expect(mounted.editor.getMarkdown()).toBe(RICH)
      expect(mounted.editor.canUndo()).toBe(false)
      expect(mounted.editor.canRedo()).toBe(true)

      expect(mounted.editor.redo()).toBe(true)
      expect(mounted.editor.getMarkdown()).toContain('First paragraph, edited.')
    } finally {
      await mounted.cleanup()
    }
  })

  it('undoes source-mode edits on the source history', async () => {
    const mounted = await mountEditor(RICH)
    try {
      mounted.editor.setMode('source')
      const view = getSourceView(mounted.root)
      replaceSourceText(view, 'First paragraph.', 'First paragraph, edited.')
      expect(mounted.editor.getMarkdown()).toContain('First paragraph, edited.')
      expect(mounted.editor.canUndo()).toBe(true)
      expect(mounted.editor.undo()).toBe(true)
      expect(mounted.editor.getMarkdown()).toBe(RICH)
      expect(mounted.editor.redo()).toBe(true)
      expect(mounted.editor.getMarkdown()).toContain('First paragraph, edited.')
    } finally {
      await mounted.cleanup()
    }
  })
})

describe('native Edit command routing', () => {
  it('undoes the captured document after switching tabs, leaving the other document unchanged', async () => {
    const a = createBrowserHost('Alpha line\n', 'alpha.md')
    const b = createBrowserHost('Beta line\n', 'beta.md')
    const workspace = createBrowserWorkspace([a, b])
    const container = document.createElement('div')
    container.style.height = '100%'
    document.body.append(container)
    const root = createRoot(container)
    flushSync(() => {
      root.render(<App workspace={workspace} />)
    })
    await vi.waitFor(
      () => {
        if (container.querySelectorAll('[data-document-id]').length !== 2) {
          throw new Error('panes not ready')
        }
        if (!container.querySelector(`[data-document-id="${a.id}"] .ProseMirror`)) {
          throw new Error('editor A not mounted')
        }
      },
      { timeout: 5000 },
    )
    await waitForTarget(a.id)

    try {
      const paneA = container.querySelector<HTMLElement>(`[data-document-id="${a.id}"]`)
      if (!paneA) throw new Error('pane A missing')
      await userEvent.click(paragraph(paneA, 'Alpha line'))
      await userEvent.keyboard('{End} edited')
      await vi.waitFor(() => {
        expect(paragraph(paneA, 'Alpha line').textContent).toContain('Alpha line edited')
      })
      await vi.waitFor(() => expect(a.getState().edited).toBe(true), { timeout: 3000 })
      const editedA = (await a.load()).text
      expect(editedA).toContain('Alpha line edited')

      const tabB = container.querySelector<HTMLButtonElement>(`[data-testid="doc-tab-${b.id}"]`)
      if (!tabB) throw new Error('tab B missing')
      flushSync(() => tabB.click())
      await vi.waitFor(async () => {
        expect((await workspace.snapshot()).activeId).toBe(b.id)
      })
      await waitForTarget(b.id)
      const textBBefore = (await b.load()).text

      expect(applyNativeEdit(a.id, 'undo')).toBe(true)
      await vi.waitFor(async () => {
        expect((await a.load()).text).toBe('Alpha line\n')
      }, { timeout: 3000 })
      expect((await b.load()).text).toBe(textBBefore)
      await vi.waitFor(() => expect(a.getState().edited).toBe(false), { timeout: 3000 })
    } finally {
      root.unmount()
      container.remove()
    }
  })

  it('clears the edited flag after undoing rich text back to the saved bytes', async () => {
    const host = createBrowserHost(RICH, 'draft.md')
    const app = await mountApp(RICH, host)
    try {
      await waitForTarget(host.id)
      await userEvent.click(paragraph(app.container, 'First paragraph.'))
      await userEvent.keyboard('{End}!')
      await vi.waitFor(() => expect(host.getState().edited).toBe(true), { timeout: 3000 })
      const flush = vi.spyOn(host, 'flush')
      expect(applyNativeEdit(host.id, 'undo')).toBe(true)
      await vi.waitFor(() => {
        expect(host.getState().edited).toBe(false)
        expect(paragraph(app.container, 'First paragraph.').textContent).toBe('First paragraph.')
      }, { timeout: 3000 })
      expect(flush).not.toHaveBeenCalled()
    } finally {
      await app.unmount()
    }
  })

  it('does not apply document undo while a composer textarea is focused', async () => {
    const source = 'One.\n\n<!-- note: keep -->\n'
    const mounted = await mountEditor(source)
    try {
      const note = mounted.root.querySelector<HTMLElement>('[data-note]')
      const edit = note?.querySelector<HTMLButtonElement>('[data-note-action="edit"]')
      if (!edit) throw new Error('note edit missing')
      await userEvent.click(edit)
      const input = await vi.waitFor(() => {
        const found = mounted.root.querySelector<HTMLTextAreaElement>('textarea[aria-label="Note"]')
        if (!found) throw new Error('composer missing')
        return found
      })
      input.focus()
      expect(nativeComposerHasFocus(mounted.root)).toBe(true)
      const id = 'composer-doc'
      const registration = registerNativeEditTarget(
        id,
        {
          capabilities: () => ({ canUndo: mounted.editor.canUndo(), canRedo: mounted.editor.canRedo() }),
          editorVisible: () => true,
          perform: () => {
            throw new Error('document undo must not run while the composer is focused')
          },
        },
        mounted.root,
      )
      try {
        applyNativeEdit(id, 'undo')
        expect(mounted.editor.getMarkdown()).toBe(source)
      } finally {
        registration.dispose()
      }
    } finally {
      await mounted.cleanup()
    }
  })

  it('does not treat GenUI table sorting as a document undo target', async () => {
    expect(nativeEditCapabilities('missing-doc')).toEqual({ canUndo: false, canRedo: false })
    expect(applyNativeEdit('missing-doc', 'undo')).toBe(false)
  })

  it('does not rewrite the document when GenUI is showing', async () => {
    const match = /```json\n([\s\S]*?)\n```/.exec(formatDoc)
    if (!match?.[1]) throw new Error('view-format.md has no JSON example')
    const document = `# Overview\n\nFirst body.\n\n# Metrics\n\nSecond body.\n\n# Next steps\n\nThird body.\n`
    const host = createBrowserHost(document, 'report.md', { view: match[1] })
    const app = await mountApp(document, host)
    try {
      await waitForTarget(host.id)
      await userEvent.click(paragraph(app.container, 'First body.'))
      await userEvent.keyboard('{End}!')
      await vi.waitFor(() => expect(host.getState().edited).toBe(true), { timeout: 3000 })
      const edited = (await host.load()).text
      await userEvent.click(app.container.querySelector('[data-testid="view-mode-genui"]') as HTMLButtonElement)
      await vi.waitFor(() => {
        expect(app.container.querySelector('[data-testid="view-mode-genui"]')?.getAttribute('aria-checked')).toBe('true')
        const editorPane = app.container.querySelector('.editor-pane')
        expect(editorPane instanceof HTMLElement && editorPane.hidden).toBe(true)
      })
      expect(applyNativeEdit(host.id, 'undo')).toBe(false)
      expect((await host.load()).text).toBe(edited)
    } finally {
      await app.unmount()
    }
  })

  it('undoes Markdown source through the native command of that document', async () => {
    const host = createBrowserHost(RICH, 'source.md')
    const app = await mountApp(RICH, host)
    try {
      await waitForTarget(host.id)
      await userEvent.click(app.container.querySelector('[data-testid="view-mode-markdown"]') as HTMLButtonElement)
      await vi.waitFor(() => {
        expect(visibleSource(app.container)).toContain('First paragraph.')
      })
      const view = getSourceView(app.container)
      replaceSourceText(view, 'First paragraph.', 'First paragraph, edited.')
      await vi.waitFor(() => expect(host.getState().edited).toBe(true), { timeout: 3000 })
      expect(applyNativeEdit(host.id, 'undo')).toBe(true)
      await vi.waitFor(() => {
        expect(visibleSource(app.container)).toContain('First paragraph.')
        expect(visibleSource(app.container)).not.toContain('First paragraph, edited.')
        expect(host.getState().edited).toBe(false)
      }, { timeout: 3000 })
    } finally {
      await app.unmount()
    }
  })
})
