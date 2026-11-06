import { describe, expect, it, vi } from 'vitest'
import { mountApp } from './helpers/app'

const DOCUMENT = `Overview
========

First overview body.

Overview
--------

Second overview body.

Metrics
-------

| Metric | Value |
| ------ | ----- |
| One    | 1     |

Next steps
----------

1. Do the thing.
`

function editorHeadings(container: HTMLElement): HTMLElement[] {
  return Array.from(
    container.querySelectorAll<HTMLElement>('[data-testid="editor-root"] h1, [data-testid="editor-root"] h2'),
  )
}

/** Index of the heading whose DOM contains the caret, or -1. */
function caretHeadingIndex(container: HTMLElement): number {
  const anchor: Node | null = window.getSelection()?.anchorNode ?? null
  if (!anchor) return -1
  return editorHeadings(container).findIndex((heading) => heading.contains(anchor))
}

describe('App navigation by Markdown source offset', () => {
  it('jumps to a setext heading referenced by a GenUI source chip', async () => {
    const app = await mountApp(DOCUMENT)
    try {
      const chip = Array.from(app.container.querySelectorAll<HTMLButtonElement>('.genui-source')).find(
        (button) => button.textContent === 'Metrics',
      )
      expect(chip).toBeDefined()

      chip?.click()

      await vi.waitFor(() => {
        expect(editorHeadings(app.container).map((heading) => heading.textContent)).toEqual([
          'Overview',
          'Overview',
          'Metrics',
          'Next steps',
        ])
        expect(editorHeadings(app.container)[caretHeadingIndex(app.container)]?.textContent).toBe(
          'Metrics',
        )
      })
    } finally {
      await app.unmount()
    }
  })

  it('reveals the second of two headings with identical text', async () => {
    const app = await mountApp(DOCUMENT)
    try {
      const links = Array.from(
        app.container.querySelectorAll<HTMLButtonElement>('.genui-heading-link'),
      ).filter((button) => button.textContent === 'Overview')
      expect(links).toHaveLength(2)

      links[1]?.click()

      await vi.waitFor(() => {
        expect(caretHeadingIndex(app.container)).toBe(1)
      })
    } finally {
      await app.unmount()
    }
  })
})
