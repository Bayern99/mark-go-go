import { userEvent } from 'vitest/browser'
import { describe, expect, it, vi } from 'vitest'
import { createBrowserHost } from '../src/host/browserHost'
import { OUTLINE_DEBOUNCE_MS } from '../src/plugins/outline/OutlinePanel'
import fixture from './fixtures/outline.md?raw'
import { mountApp } from './helpers/app'
import { getSourceView, replaceSourceText, sourceText } from './helpers/source'

/** `outline.md`: ATX H1–H3, a setext H2, a fenced pseudo-heading and a duplicate H2. */
const EXPECTED_HEADINGS = [
  { text: 'Alpha', level: '1' },
  { text: 'Beta', level: '2' },
  { text: 'Gamma', level: '3' },
  { text: 'Setext title', level: '2' },
  { text: 'Beta', level: '2' },
]

const SECOND_BETA_OFFSET = fixture.indexOf('## Beta', fixture.indexOf('## Beta') + 1)

function outlineButtons(container: HTMLElement): HTMLButtonElement[] {
  return Array.from(container.querySelectorAll<HTMLButtonElement>('.outline-link'))
}

function outlineEntries(container: HTMLElement): { text: string; level: string }[] {
  return outlineButtons(container).map((button) => ({
    text: button.textContent ?? '',
    level: button.getAttribute('data-level') ?? '',
  }))
}

function currentOutlineText(container: HTMLElement): string | null {
  return container.querySelector('.outline-link[aria-current="true"]')?.textContent ?? null
}

function editorHeadings(container: HTMLElement): HTMLElement[] {
  return Array.from(
    container.querySelectorAll<HTMLElement>(
      '[data-testid="editor-root"] h1, [data-testid="editor-root"] h2, [data-testid="editor-root"] h3',
    ),
  )
}

/** Index of the heading whose DOM contains the caret, or -1. */
function caretHeadingIndex(container: HTMLElement): number {
  const anchor: Node | null = window.getSelection()?.anchorNode ?? null
  if (!anchor) return -1
  return editorHeadings(container).findIndex((heading) => heading.contains(anchor))
}

function modeToggle(container: HTMLElement): HTMLButtonElement {
  const button = container.querySelector<HTMLButtonElement>('[data-testid="mode-toggle"]')
  if (!button) throw new Error('mode toggle missing')
  return button
}

function outlineToggle(container: HTMLElement): HTMLButtonElement {
  const button = container.querySelector<HTMLButtonElement>('[data-testid="outline-toggle"]')
  if (!button) throw new Error('outline toggle missing')
  return button
}

async function enterSourceMode(container: HTMLElement): Promise<void> {
  modeToggle(container).click()
  await vi.waitFor(() => {
    expect(modeToggle(container).getAttribute('aria-pressed')).toBe('true')
    expect(sourceText(container)).not.toBe('')
  })
}

describe('Outline plugin', () => {
  it('lists exactly the real headings with correct levels', async () => {
    const app = await mountApp(fixture)
    try {
      expect(outlineEntries(app.container)).toEqual(EXPECTED_HEADINGS)
      expect(app.container.querySelector('[data-testid="outline-empty"]')).toBeNull()
      expect(
        Array.from(app.container.querySelectorAll('.outline-link')).some(
          (button) => button.textContent === 'Fenced heading',
        ),
      ).toBe(false)
    } finally {
      await app.unmount()
    }
  })

  it('opens a left sidebar while GenUI stays on the right', async () => {
    const app = await mountApp(fixture)
    try {
      const leftPane = app.container.querySelector('[data-testid="left-pane"]')
      const rightPane = app.container.querySelector('[data-testid="side-pane"]')
      expect(leftPane?.querySelector('.outline-panel')).not.toBeNull()
      expect(rightPane?.querySelector('.genui-panel')).not.toBeNull()
      expect(leftPane?.querySelector('.genui-panel')).toBeNull()
      expect(rightPane?.querySelector('.outline-panel')).toBeNull()
    } finally {
      await app.unmount()
    }
  })

  it('moves the caret to each entry, including the second duplicate, in rich text', async () => {
    const app = await mountApp(fixture)
    try {
      expect(editorHeadings(app.container).map((heading) => heading.textContent)).toEqual([
        'Alpha',
        'Beta',
        'Gamma',
        'Setext title',
        'Beta',
      ])

      const buttons = outlineButtons(app.container)
      for (let index = 0; index < buttons.length; index += 1) {
        buttons[index]?.click()
        await vi.waitFor(() => {
          expect(caretHeadingIndex(app.container)).toBe(index)
        })
      }
    } finally {
      await app.unmount()
    }
  })

  it('moves the source caret to each entry, including the second duplicate', async () => {
    const app = await mountApp(fixture)
    try {
      await enterSourceMode(app.container)
      const expectedOffsets = [
        fixture.indexOf('# Alpha'),
        fixture.indexOf('## Beta'),
        fixture.indexOf('### Gamma'),
        fixture.indexOf('Setext title'),
        SECOND_BETA_OFFSET,
      ]
      expect(expectedOffsets[4]).toBeGreaterThan(expectedOffsets[1])

      const buttons = outlineButtons(app.container)
      expect(buttons).toHaveLength(expectedOffsets.length)
      for (let index = 0; index < buttons.length; index += 1) {
        buttons[index]?.click()
        const offset = expectedOffsets[index] as number
        await vi.waitFor(() => {
          expect(getSourceView(app.container).state.selection.main.head).toBe(offset)
        })
      }
    } finally {
      await app.unmount()
    }
  })

  it('picks up added, renamed and deleted headings after the debounce', async () => {
    const app = await mountApp(fixture)
    try {
      await enterSourceMode(app.container)
      const view = getSourceView(app.container)

      replaceSourceText(view, 'Second beta body.', 'Second beta body.\n\n## Added section\n')
      await vi.waitFor(
        () => {
          expect(outlineEntries(app.container).map((entry) => entry.text)).toContain('Added section')
        },
        { timeout: OUTLINE_DEBOUNCE_MS + 2000 },
      )

      replaceSourceText(view, '## Added section', '## Renamed section')
      await vi.waitFor(
        () => {
          const texts = outlineEntries(app.container).map((entry) => entry.text)
          expect(texts).toContain('Renamed section')
          expect(texts).not.toContain('Added section')
        },
        { timeout: OUTLINE_DEBOUNCE_MS + 2000 },
      )

      replaceSourceText(view, '## Renamed section\n', '')
      await vi.waitFor(
        () => {
          expect(outlineEntries(app.container).map((entry) => entry.text)).not.toContain(
            'Renamed section',
          )
        },
        { timeout: OUTLINE_DEBOUNCE_MS + 2000 },
      )
    } finally {
      await app.unmount()
    }
  })

  it('highlights the section containing the caret', async () => {
    const app = await mountApp(fixture)
    try {
      const gammaBody = Array.from(
        app.container.querySelectorAll<HTMLElement>('[data-testid="editor-root"] p'),
      ).find((element) => element.textContent === 'Gamma body.')
      expect(gammaBody).toBeDefined()
      await userEvent.click(gammaBody as HTMLElement)

      await vi.waitFor(() => {
        expect(currentOutlineText(app.container)).toBe('Gamma')
      })

      const intro = Array.from(
        app.container.querySelectorAll<HTMLElement>('[data-testid="editor-root"] p'),
      ).find((element) => element.textContent === 'Intro text.')
      expect(intro).toBeDefined()
      await userEvent.click(intro as HTMLElement)

      await vi.waitFor(() => {
        expect(currentOutlineText(app.container)).toBe('Alpha')
      })
    } finally {
      await app.unmount()
    }
  })

  it('shows an empty state for a document without headings', async () => {
    const app = await mountApp('Just a paragraph.\n')
    try {
      expect(app.container.querySelector('[data-testid="outline-list"]')).toBeNull()
      expect(app.container.querySelector('[data-testid="outline-empty"]')?.textContent).toBe(
        'No headings in this document.',
      )
    } finally {
      await app.unmount()
    }
  })

  it('removes the left sidebar when disabled and keeps edits flowing to the host', async () => {
    const app = await mountApp(fixture, createBrowserHost(fixture))
    const update = vi.spyOn(app.host, 'update')
    try {
      expect(app.container.querySelector('[data-testid="left-pane"]')).not.toBeNull()

      outlineToggle(app.container).click()
      await vi.waitFor(() => {
        expect(outlineToggle(app.container).getAttribute('aria-pressed')).toBe('false')
        expect(app.container.querySelector('[data-testid="left-pane"]')).toBeNull()
        expect(app.container.querySelector('[data-testid="side-pane"]')).not.toBeNull()
      })

      await enterSourceMode(app.container)
      replaceSourceText(getSourceView(app.container), 'Intro text.', 'Intro text, edited.')
      await vi.waitFor(
        () => {
          expect(update.mock.lastCall?.[0]).toContain('Intro text, edited.')
        },
        { timeout: 3000 },
      )
    } finally {
      await app.unmount()
    }
  })

  it('leaves GenUI without its own heading list', async () => {
    const app = await mountApp(fixture)
    try {
      expect(app.container.querySelector('.genui-panel')).not.toBeNull()
      expect(app.container.querySelector('.genui-headings')).toBeNull()
      expect(app.container.querySelectorAll('.genui-heading-link')).toHaveLength(0)
      expect(app.container.querySelector('.genui-source')).not.toBeNull()
    } finally {
      await app.unmount()
    }
  })
})
