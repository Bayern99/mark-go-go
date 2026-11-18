import '../src/styles.css'
import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { page } from 'vitest/browser'
import { describe, expect, it, vi } from 'vitest'
import { App } from '../src/App'
import type { AppPlugin } from '../src/core/types'
import { createBrowserHost } from '../src/host/browserHost'
import { OUTLINE_DEBOUNCE_MS } from '../src/plugins/outline/OutlinePanel'
import outlineExtra from './fixtures/outline-extra.md?raw'
import fixture from './fixtures/outline.md?raw'
import { mountApp } from './helpers/app'
import { getSourceView, replaceSourceText, sourceText } from './helpers/source'

const EXPECTED_HEADINGS = ['Alpha', 'Beta', 'Gamma', 'Setext title', 'Beta']

const LONG_DOCUMENT = [
  '# Top',
  ...Array.from(
    { length: 80 },
    (_, index) => `Filler paragraph ${index} with enough words to take up a line.`,
  ),
  '## Deep heading',
  ...Array.from({ length: 40 }, (_, index) => `Deep body paragraph ${index}.`),
].join('\n\n')

function outline() {
  return page.getByRole('navigation', { name: 'Document outline' })
}

function outlineButtons() {
  return outline().getByRole('button')
}

function outlineTexts(): string[] {
  return outlineButtons()
    .all()
    .map((button) => button.element().textContent?.trim() ?? '')
}

function currentOutlineTexts(): string[] {
  return outlineButtons()
    .elements()
    .filter((button) => button.getAttribute('aria-current') === 'true')
    .map((button) => button.textContent?.trim() ?? '')
}

function textLeft(button: Element): number {
  const range = document.createRange()
  range.selectNodeContents(button)
  return range.getBoundingClientRect().left
}

function viewMode(mode: 'preview' | 'markdown' | 'genui') {
  return page.getByTestId(`view-mode-${mode}`)
}

async function enterSourceMode(): Promise<void> {
  await viewMode('markdown').click()
  await vi.waitFor(() => {
    expect(viewMode('markdown').element().getAttribute('aria-checked')).toBe('true')
    expect(sourceText(document.body)).not.toBe('')
  })
}

function editorHeadings(): HTMLElement[] {
  return Array.from(
    document.querySelectorAll<HTMLElement>(
      '[data-testid="editor-root"] h1, [data-testid="editor-root"] h2, [data-testid="editor-root"] h3',
    ),
  )
}

function caretHeadingIndex(): number {
  const anchor: Node | null = window.getSelection()?.anchorNode ?? null
  if (!anchor) return -1
  return editorHeadings().findIndex((heading) => heading.contains(anchor))
}

function sourceCaretOffset(container: HTMLElement): number | null {
  const selection = window.getSelection()
  const anchor = selection?.anchorNode ?? null
  if (!anchor || !container.contains(anchor)) return null
  const content = anchor.nodeType === Node.TEXT_NODE ? anchor.parentElement : (anchor as Element)
  if (!content?.closest('.cm-content')) return null
  return getSourceView(container).posAtDOM(anchor, selection?.anchorOffset ?? 0)
}

function nearScrollTop(element: Element): boolean {
  const scroller = element.closest('.editor-scroll')
  if (!scroller) throw new Error('heading is not inside the editor scroll area')
  const top = element.getBoundingClientRect().top - scroller.getBoundingClientRect().top
  return top >= 0 && top <= 120
}

function editorHasFocus(container: HTMLElement): boolean {
  const root = container.querySelector('[data-testid="editor-root"]')
  return root ? root.contains(document.activeElement) : false
}

describe('Outline plugin', () => {
  it('lists exactly the real headings, indented by level', async () => {
    const app = await mountApp(fixture)
    try {
      expect(outlineTexts()).toEqual(EXPECTED_HEADINGS)
      await expect.element(page.getByTestId('outline-count')).toHaveTextContent('5 headings')


      const alpha = textLeft(outlineButtons().nth(0).element())
      const beta = textLeft(outlineButtons().nth(1).element())
      const gamma = textLeft(outlineButtons().nth(2).element())
      const setext = textLeft(outlineButtons().nth(3).element())
      const secondBeta = textLeft(outlineButtons().nth(4).element())

      expect(alpha).toBeLessThan(beta)
      expect(beta).toBeLessThan(gamma)
      expect(setext).toBeCloseTo(beta, 0)
      expect(secondBeta).toBeCloseTo(beta, 0)
    } finally {
      await app.unmount()
    }
  })

  it('strips inline formatting and skips frontmatter and raw HTML headings', async () => {
    const app = await mountApp(outlineExtra)
    try {
      expect(outlineTexts()).toEqual(['Real heading', 'Bold and code', 'Tail'])
    } finally {
      await app.unmount()
    }
  })

  it('renders in the right pane in GenUI mode and jumps back to Preview', async () => {
    const app = await mountApp(fixture)
    try {
      expect(page.getByTestId('outline-toggle').query()).toBeNull()
      const left = page.getByTestId('left-pane')
      expect(left.query()).not.toBeNull()
      expect(left.getByRole('navigation', { name: 'Document outline' }).query()).toBeNull()

      await viewMode('genui').click()
      await vi.waitFor(() => {
        expect(page.getByTestId('center-pane').query()).not.toBeNull()
        expect(app.container.querySelector<HTMLElement>('.editor-pane')?.hidden).toBe(true)
      })
      const right = page.getByTestId('right-pane')
      expect(right.getByRole('navigation', { name: 'Document outline' }).query()).not.toBeNull()
      expect(right.getByTestId('genui-view').query()).toBeNull()

      await outlineButtons().nth(2).click()
      await vi.waitFor(() => {
        expect(viewMode('preview').element().getAttribute('aria-checked')).toBe('true')
        expect(page.getByTestId('center-pane').query()).toBeNull()
        expect(caretHeadingIndex()).toBe(2)
      })
    } finally {
      await app.unmount()
    }
  })

  it('moves the caret to each entry, including the second duplicate, in Preview', async () => {
    const app = await mountApp(fixture)
    try {
      expect(editorHeadings().map((heading) => heading.textContent)).toEqual(EXPECTED_HEADINGS)
      expect(caretHeadingIndex()).toBe(-1)

      for (let index = 0; index < EXPECTED_HEADINGS.length; index += 1) {
        await outlineButtons().nth(index).click()
        await vi.waitFor(() => {
          expect(caretHeadingIndex()).toBe(index)
        })
      }
    } finally {
      await app.unmount()
    }
  })

  it('moves the source caret to each entry, including the second duplicate', async () => {
    const app = await mountApp(fixture)
    try {
      await enterSourceMode()
      const offsets = [
        fixture.indexOf('# Alpha'),
        fixture.indexOf('## Beta'),
        fixture.indexOf('### Gamma'),
        fixture.indexOf('Setext title'),
        fixture.indexOf('## Beta', fixture.indexOf('## Beta') + 1),
      ]
      expect(offsets[4]).toBeGreaterThan(offsets[1])

      for (let index = 0; index < offsets.length; index += 1) {
        await outlineButtons().nth(index).click()
        const offset = offsets[index] as number
        await vi.waitFor(() => {
          expect(sourceCaretOffset(app.container)).toBe(offset)
        })
      }
    } finally {
      await app.unmount()
    }
  })

  it('picks up added, renamed and deleted headings after the debounce', async () => {
    const app = await mountApp(fixture)
    try {
      await enterSourceMode()

      replaceSourceText(
        getSourceView(app.container),
        'Second beta body.',
        'Second beta body.\n\n## Added section\n',
      )
      await vi.waitFor(
        () => {
          expect(outlineTexts()).toContain('Added section')
          expect(page.getByTestId('outline-count').element().textContent).toBe('6 headings')
        },
        { timeout: OUTLINE_DEBOUNCE_MS + 2000 },
      )

      replaceSourceText(getSourceView(app.container), '## Added section', '## Renamed section')
      await vi.waitFor(
        () => {
          const texts = outlineTexts()
          expect(texts).toContain('Renamed section')
          expect(texts).not.toContain('Added section')
        },
        { timeout: OUTLINE_DEBOUNCE_MS + 2000 },
      )

      replaceSourceText(getSourceView(app.container), '## Renamed section\n', '')
      await vi.waitFor(
        () => {
          expect(outlineTexts()).not.toContain('Renamed section')
          expect(page.getByTestId('outline-count').element().textContent).toBe('5 headings')
        },
        { timeout: OUTLINE_DEBOUNCE_MS + 2000 },
      )
    } finally {
      await app.unmount()
    }
  })

  it('highlights the section containing the caret and nothing before the first heading', async () => {
    const app = await mountApp(fixture)
    try {
      expect(currentOutlineTexts()).toEqual([])

      await page.getByText('Preamble text.').click()
      await vi.waitFor(() => {
        expect(currentOutlineTexts()).toEqual([])
      })

      await page.getByText('Gamma body.').click()
      await vi.waitFor(() => {
        expect(currentOutlineTexts()).toEqual(['Gamma'])
      })

      await page.getByText('First beta body.').click()
      await vi.waitFor(() => {
        expect(currentOutlineTexts()).toEqual(['Beta'])
        expect(outlineButtons().nth(1).element().getAttribute('aria-current')).toBe('true')
        expect(outlineButtons().nth(4).element().getAttribute('aria-current')).toBeNull()
      })
    } finally {
      await app.unmount()
    }
  })

  it('highlights the containing section in source mode', async () => {
    const app = await mountApp(fixture)
    try {
      await enterSourceMode()
      const view = getSourceView(app.container)
      const offset = fixture.indexOf('Second beta body.')
      expect(offset).toBeGreaterThan(fixture.indexOf('## Beta', fixture.indexOf('## Beta') + 1))
      view.dispatch({ selection: { anchor: offset } })

      await vi.waitFor(() => {
        expect(outlineButtons().nth(4).element().getAttribute('aria-current')).toBe('true')
        expect(outlineButtons().nth(1).element().getAttribute('aria-current')).toBeNull()
      })
    } finally {
      await app.unmount()
    }
  })

  it('scrolls the target into view and focuses the editor in Preview', async () => {
    const app = await mountApp(LONG_DOCUMENT)
    try {
      const heading = page.getByRole('heading', { name: 'Deep heading' })
      expect(nearScrollTop(heading.element())).toBe(false)

      await outlineButtons().last().click()
      await vi.waitFor(() => {
        expect(nearScrollTop(heading.element())).toBe(true)
        expect(editorHasFocus(app.container)).toBe(true)
      })
      expect(outlineButtons().last().element()).not.toBe(document.activeElement)
    } finally {
      await app.unmount()
    }
  })

  it('scrolls the target into view and focuses the editor in source mode', async () => {
    const app = await mountApp(LONG_DOCUMENT)
    try {
      await enterSourceMode()
      await outlineButtons().last().click()

      await vi.waitFor(() => {
        const line = Array.from(
          app.container.querySelectorAll<HTMLElement>('.cm-content .cm-line'),
        ).find((element) => element.textContent === '## Deep heading')
        expect(line).toBeDefined()
        expect(nearScrollTop(line as HTMLElement)).toBe(true)
        expect(editorHasFocus(app.container)).toBe(true)
      })
      expect(outlineButtons().last().element()).not.toBe(document.activeElement)
    } finally {
      await app.unmount()
    }
  })

  it('restores the right pane and highlighting when re-opened', async () => {
    const app = await mountApp(fixture)
    try {
      await page.getByTestId('right-pane-toggle').click()
      await vi.waitFor(() => {
        expect(page.getByTestId('right-pane').element().hasAttribute('hidden')).toBe(true)
        expect(page.getByTestId('right-pane-toggle').element().getAttribute('aria-expanded')).toBe(
          'false',
        )
      })

      await page.getByTestId('right-pane-toggle').click()
      await vi.waitFor(() => {
        expect(page.getByTestId('right-pane').element().hasAttribute('hidden')).toBe(false)
        expect(page.getByTestId('right-pane-toggle').element().getAttribute('aria-expanded')).toBe(
          'true',
        )
        expect(outlineTexts()).toEqual(EXPECTED_HEADINGS)
      })

      await page.getByText('Gamma body.').click()
      await vi.waitFor(() => {
        expect(currentOutlineTexts()).toEqual(['Gamma'])
      })
    } finally {
      await app.unmount()
    }
  })

  it('shows an empty state for a document without headings', async () => {
    const app = await mountApp('Just a paragraph.\n')
    try {
      expect(outlineButtons().elements()).toHaveLength(0)
      await expect.element(page.getByText('No headings in this document.')).toBeVisible()
    } finally {
      await app.unmount()
    }
  })

  it('collapses the right pane when toggled off and keeps edits flowing to the host', async () => {
    const app = await mountApp(fixture, createBrowserHost(fixture))
    const update = vi.spyOn(app.host, 'update')
    try {
      expect(page.getByTestId('right-pane').element().hasAttribute('hidden')).toBe(false)

      await page.getByTestId('right-pane-toggle').click()
      await vi.waitFor(() => {
        expect(page.getByTestId('right-pane-toggle').element().getAttribute('aria-expanded')).toBe(
          'false',
        )
        expect(page.getByTestId('right-pane').element().hasAttribute('hidden')).toBe(true)
      })

      await enterSourceMode()
      replaceSourceText(getSourceView(app.container), 'Preamble text.', 'Preamble text, edited.')
      await vi.waitFor(
        () => expect(update.mock.lastCall?.[0]).toContain('Preamble text, edited.'),
        { timeout: 3000 },
      )
    } finally {
      await app.unmount()
    }
  })
})

describe('App plugin activation', () => {
  it('unmounts cleanly while a plugin activation is still pending', async () => {
    let release!: () => void
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })
    let lateExecute: (() => Promise<unknown>) | null = null
    const unhandled: unknown[] = []
    const onUnhandled = (event: PromiseRejectionEvent) => unhandled.push(event.reason)
    window.addEventListener('unhandledrejection', onUnhandled)

    const slowPlugin: AppPlugin = {
      id: 'slow',
      title: 'Slow',
      async activate(ctx) {
        await gate
        ctx.commands.register('slow.ping', () => 'pong')
        lateExecute = () => ctx.commands.execute('slow.ping')
        ctx.editor.onSelectionChange(() => {})
      },
    }

    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    flushSync(() => {
      root.render(<App host={createBrowserHost(fixture)} plugins={[slowPlugin]} />)
    })

    try {
      await vi.waitFor(() => {
        expect(container.querySelector('.outline-panel')).not.toBeNull()
      })

      root.unmount()
      container.remove()
      release()
      await new Promise((resolve) => setTimeout(resolve, 100))

      expect(unhandled).toEqual([])
      expect(lateExecute).not.toBeNull()
      const execute = lateExecute as unknown as () => Promise<unknown>
      await expect(execute()).rejects.toThrow(/Unknown command/)
    } finally {
      window.removeEventListener('unhandledrejection', onUnhandled)
      container.remove()
    }
  })
})
