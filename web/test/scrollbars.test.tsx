import '../src/styles.css'
import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'
import { App } from '../src/App'
import { createBrowserHost, createBrowserWorkspace } from '../src/host/browserHost'
import type { WorkspaceAdapter } from '../src/host/types'
import { mountApp } from './helpers/app'
import { scrollViewport } from './helpers/scroll'

const LONG_DOCUMENT = `# Long\n\n${Array.from({ length: 240 }, (_, index) => `Paragraph ${index}.`).join('\n\n')}\n`

async function mountWorkspace(workspace: WorkspaceAdapter) {
  const container = document.createElement('div')
  container.style.height = '100%'
  container.style.width = '640px'
  document.body.append(container)
  const root = createRoot(container)
  flushSync(() => root.render(<App workspace={workspace} />))
  await vi.waitFor(() => {
    if (!container.querySelector('[data-testid="workspace-tree"] [data-overlayscrollbars-viewport]')) {
      throw new Error('workspace scroll viewport missing')
    }
  })
  return {
    container,
    async unmount() {
      root.unmount()
      container.remove()
      await Promise.resolve()
    },
  }
}

describe('overlay scrollbars', () => {
  it('shows editor scrolling without layout shift and hides after interaction stops', async () => {
    const app = await mountApp(LONG_DOCUMENT)
    try {
      const editor = app.container.querySelector<HTMLElement>('.editor-scroll')
      const outline = app.container.querySelector<HTMLElement>('.right-view-body')
      if (!editor || !outline) throw new Error('document scroll areas missing')
      for (const host of [editor, outline]) {
        expect(host.getAttribute('data-overlayscrollbars')).toContain('host')
        expect(host.querySelectorAll('[data-overlayscrollbars-viewport]')).toHaveLength(1)
      }

      const viewport = scrollViewport(editor)
      const content = app.container.querySelector<HTMLElement>('[data-testid="editor-root"]')
      const scrollbar = editor.querySelector<HTMLElement>('.os-scrollbar-vertical')
      if (!content || !scrollbar) throw new Error('editor scrollbar missing')
      await vi.waitFor(() => expect(scrollbar.classList.contains('os-scrollbar-auto-hide-hidden')).toBe(true))
      const restingWidth = content.getBoundingClientRect().width

      viewport.scrollTop = 800
      await vi.waitFor(() => {
        expect(viewport.scrollTop).toBeGreaterThan(0)
        expect(scrollbar.classList.contains('os-scrollbar-visible')).toBe(true)
      })
      expect(content.getBoundingClientRect().width).toBe(restingWidth)
      await vi.waitFor(
        () => expect(scrollbar.classList.contains('os-scrollbar-auto-hide-hidden')).toBe(true),
        { timeout: 1800 },
      )
    } finally {
      await app.unmount()
    }
  })

  it('uses overlay scrolling in the file tree but never in document tabs', async () => {
    const documents = Array.from({ length: 14 }, (_, index) =>
      createBrowserHost(`# Document ${index}\n`, `a-very-long-document-name-${index}.md`),
    )
    const workspace = createBrowserWorkspace(documents, {
      root: '/work',
      name: 'work',
      kind: 'folder',
      nav: { files: [], folders: [], status: 'ready' },
    })
    const app = await mountWorkspace(workspace)
    try {
      const tree = app.container.querySelector<HTMLElement>('[data-testid="workspace-tree"]')
      const tabs = app.container.querySelector<HTMLElement>('[data-testid="doc-tabs"]')
      if (!tree || !tabs) throw new Error('workspace navigation missing')
      expect(scrollViewport(tree)).not.toBeNull()
      expect(tabs.scrollWidth).toBe(tabs.clientWidth)
      expect(tabs.querySelectorAll('.doc-tab.tab-hidden').length).toBeGreaterThan(0)
      const overflow = app.container.querySelector<HTMLButtonElement>('[data-testid="doc-tab-overflow"]')
      expect(overflow?.hidden).toBe(false)
      expect(getComputedStyle(tabs).overflowX).toBe('hidden')
      expect(tabs.querySelector('.os-scrollbar')).toBeNull()
      expect(tabs.getAttribute('data-overlayscrollbars')).toBeNull()
    } finally {
      await app.unmount()
    }
  })
})
