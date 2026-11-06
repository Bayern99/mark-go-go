import { page, userEvent } from 'vitest/browser'
import { describe, expect, it, vi } from 'vitest'
import { createBrowserHost } from '../src/host/browserHost'
import type { HostAdapter } from '../src/host/types'
import { extractSections } from '../src/markdown/sections'
import { GENUI_DEBOUNCE_MS } from '../src/plugins/genui/GenUIPanel'
import formatDoc from '../src/plugins/genui/view-format.md?raw'
import { mountApp } from './helpers/app'
import { caretLineInSource, getSourceView, replaceSourceText } from './helpers/source'

const DOCUMENT = `# Overview

First body.

# Metrics

Second body.

# Next steps

Third body.
`

/** The example in the shipped format document, which the agent prompt will embed too. */
function formatExample(): string {
  const match = /```json\n([\s\S]*?)\n```/.exec(formatDoc)
  if (!match?.[1]) throw new Error('view-format.md has no JSON example')
  return match[1]
}

interface ViewInput {
  heading: string
  occurrence?: number
  sourceHash?: string
  blocks: unknown[]
}

function viewFile(sections: ViewInput[]): string {
  return JSON.stringify({ version: 1, sections })
}

/** A view whose sections carry the hashes of `markdown`, as the host writes them. */
function stampedView(markdown: string, sections: ViewInput[]): string {
  const current = extractSections(markdown)
  return viewFile(
    sections.map((section) => {
      const match = current.find(
        (candidate) =>
          candidate.heading.text === section.heading &&
          candidate.occurrence === (section.occurrence ?? 1),
      )
      if (!match) throw new Error(`no section ${section.heading}`)
      return { ...section, sourceHash: match.hash }
    }),
  )
}

const SUMMARY = { component: 'Summary', items: [{ text: 'A point.' }] }

function genui() {
  return page.getByRole('region', { name: 'Generated UI' })
}

/** Each rendered section's heading and status badge, as the reader sees them. */
function sectionRows(container: HTMLElement): Array<[string, string]> {
  return Array.from(container.querySelectorAll<HTMLElement>('.genui-section')).map((section) => [
    section.querySelector('.genui-heading')?.textContent ?? '',
    section.querySelector('.genui-status')?.textContent ?? '',
  ])
}

function sectionNamed(container: HTMLElement, heading: string): HTMLElement {
  const section = Array.from(container.querySelectorAll<HTMLElement>('.genui-section')).find(
    (candidate) => candidate.querySelector('.genui-heading')?.textContent === heading,
  )
  if (!section) throw new Error(`section not rendered: ${heading}`)
  return section
}

async function enterSourceMode(container: HTMLElement): Promise<void> {
  const toggle = container.querySelector<HTMLButtonElement>('[data-testid="mode-toggle"]')
  if (!toggle) throw new Error('mode toggle missing')
  await userEvent.click(toggle)
  await vi.waitFor(() => expect(toggle.getAttribute('aria-pressed')).toBe('true'))
}

function editorHeadings(container: HTMLElement): HTMLElement[] {
  return Array.from(
    container.querySelectorAll<HTMLElement>(
      '[data-testid="editor-root"] h1, [data-testid="editor-root"] h2',
    ),
  )
}

/** Index of the editor heading whose DOM contains the caret, or -1. */
function caretHeadingIndex(container: HTMLElement): number {
  const anchor: Node | null = window.getSelection()?.anchorNode ?? null
  if (!anchor) return -1
  return editorHeadings(container).findIndex((heading) => heading.contains(anchor))
}

describe('GenUI view file', () => {
  it('offers a disabled Generate button when the document has no view', async () => {
    const app = await mountApp(DOCUMENT)
    try {
      await expect.element(genui().getByText('No view yet.')).toBeVisible()
      await expect.element(genui().getByRole('button', { name: 'Generate' })).toBeDisabled()
      await expect.element(genui().getByText('coming soon', { exact: false })).toBeVisible()
    } finally {
      await app.unmount()
    }
  })

  it('renders the format document example with every component', async () => {
    const markdown = '# Overview\n\nA.\n\n# Metrics\n\nB.\n\n# Next steps\n\nC.\n'
    const app = await mountApp(markdown, createBrowserHost(markdown, 'report.md', { view: formatExample() }))
    try {
      await vi.waitFor(() =>
        expect(sectionRows(app.container)).toEqual([
          ['Overview', ''],
          ['Metrics', ''],
          ['Next steps', ''],
        ]),
      )
      const panel = genui()
      expect(panel.getByRole('alert').query()).toBeNull()
      await expect.element(panel.getByText('gemini-3.8-flash', { exact: false })).toBeVisible()

      await expect.element(panel.getByRole('heading', { name: 'In short' })).toBeVisible()
      await expect
        .element(panel.getByText('Only the edited block is written again.'))
        .toBeVisible()

      await expect.element(panel.getByRole('rowheader', { name: 'Round trips' })).toBeVisible()
      await expect.element(panel.getByRole('cell', { name: '100%' })).toBeVisible()

      await expect
        .element(panel.getByRole('checkbox', { name: 'Open a real file through the host' }))
        .toBeChecked()
      await expect
        .element(panel.getByRole('checkbox', { name: 'Scroll to a heading from the panel' }))
        .not.toBeChecked()
    } finally {
      await app.unmount()
    }
  })

  it('reports a bad component in its own section and still renders the rest', async () => {
    const view = viewFile([
      {
        heading: 'Overview',
        blocks: [
          { component: 'Script', html: '<img src=x onerror="alert(1)">' },
          { component: 'Summary', items: [{ text: 'Kept point.' }, { txt: 'typo' }] },
        ],
      },
      {
        heading: 'Metrics',
        blocks: [{ component: 'Summary', items: [{ text: '<b>not bold</b> **nor this**' }] }],
      },
    ])
    const app = await mountApp(DOCUMENT, createBrowserHost(DOCUMENT, 'report.md', { view }))
    try {
      await vi.waitFor(() => expect(sectionRows(app.container)).toHaveLength(2))

      const overview = sectionNamed(app.container, 'Overview')
      const problems = overview.querySelector('[role="alert"]')?.textContent ?? ''
      expect(problems).toContain('"Script" is not in the whitelist')
      expect(problems).toContain('items 2')
      expect(overview.textContent).toContain('Kept point.')

      const metrics = sectionNamed(app.container, 'Metrics')
      expect(metrics.querySelector('[role="alert"]')).toBeNull()
      expect(metrics.textContent).toContain('<b>not bold</b> **nor this**')
      expect(app.container.querySelector('.genui-panel b, .genui-panel img, .genui-panel strong')).toBeNull()
    } finally {
      await app.unmount()
    }
  })

  it('shows a section with a bad anchor unmatched, and no card for a block without its list', async () => {
    const view = viewFile([
      {
        heading: 'Overview',
        occurrence: 0,
        blocks: [SUMMARY, { component: 'KeyValueTable', title: 'Empty', rows: 'none' }],
      },
    ])
    const app = await mountApp(DOCUMENT, createBrowserHost(DOCUMENT, 'report.md', { view }))
    try {
      await vi.waitFor(() => expect(sectionRows(app.container)).toEqual([['Overview', '']]))

      const overview = sectionNamed(app.container, 'Overview')
      expect(overview.querySelector<HTMLButtonElement>('.genui-heading')?.disabled).toBe(true)
      const problems = overview.querySelector('[role="alert"]')?.textContent ?? ''
      expect(problems).toContain('"occurrence" must be a whole number from 1.')
      expect(problems).toContain('"rows" must be an array')
      expect(overview.querySelectorAll('.genui-block')).toHaveLength(1)
      expect(overview.textContent).toContain('A point.')
      expect(overview.textContent).not.toContain('Empty')
    } finally {
      await app.unmount()
    }
  })

  it('shows one message for a file that is not a usable view', async () => {
    const host = createBrowserHost(DOCUMENT, 'report.md', { view: '{ "version": 1, ' })
    const app = await mountApp(DOCUMENT, host)
    try {
      await expect.element(genui().getByRole('alert')).toMatchTextContent('not valid JSON')

      host.writeView(JSON.stringify({ version: 2, sections: [] }))
      await expect.element(genui().getByRole('alert')).toMatchTextContent('version 2')
      expect(sectionRows(app.container)).toEqual([])
    } finally {
      await app.unmount()
    }
  })

  it('follows the view file as it is written, replaced and removed', async () => {
    const host = createBrowserHost(DOCUMENT, 'report.md')
    const app = await mountApp(DOCUMENT, host)
    try {
      await expect.element(genui().getByText('No view yet.')).toBeVisible()

      host.writeView(viewFile([{ heading: 'Metrics', blocks: [SUMMARY] }]))
      await vi.waitFor(() => expect(sectionRows(app.container)).toEqual([['Metrics', '']]))

      host.writeView(viewFile([{ heading: 'Next steps', blocks: [SUMMARY] }]))
      await vi.waitFor(() => expect(sectionRows(app.container)).toEqual([['Next steps', '']]))

      host.writeView(null)
      await expect.element(genui().getByText('No view yet.')).toBeVisible()
      expect(sectionRows(app.container)).toEqual([])
    } finally {
      await app.unmount()
    }
  })

  it('marks only the edited section stale and a renamed heading missing', async () => {
    const view = stampedView(DOCUMENT, [
      { heading: 'Overview', blocks: [SUMMARY] },
      { heading: 'Metrics', blocks: [SUMMARY] },
      { heading: 'Next steps', blocks: [SUMMARY] },
    ])
    const app = await mountApp(DOCUMENT, createBrowserHost(DOCUMENT, 'report.md', { view }))
    try {
      await vi.waitFor(() =>
        expect(sectionRows(app.container)).toEqual([
          ['Overview', ''],
          ['Metrics', ''],
          ['Next steps', ''],
        ]),
      )
      await enterSourceMode(app.container)

      replaceSourceText(getSourceView(app.container), 'Second body.', 'Second body, edited.')
      await vi.waitFor(
        () =>
          expect(sectionRows(app.container)).toEqual([
            ['Overview', ''],
            ['Metrics', 'Stale'],
            ['Next steps', ''],
          ]),
        { timeout: GENUI_DEBOUNCE_MS + 2000 },
      )

      replaceSourceText(getSourceView(app.container), '# Next steps', '# Later')
      await vi.waitFor(
        () =>
          expect(sectionRows(app.container)).toEqual([
            ['Overview', ''],
            ['Metrics', 'Stale'],
            ['Next steps', 'Missing'],
          ]),
        { timeout: GENUI_DEBOUNCE_MS + 2000 },
      )
      await expect.element(genui().getByRole('button', { name: 'Next steps' })).toBeDisabled()

      replaceSourceText(getSourceView(app.container), 'Second body, edited.', 'Second body.')
      await vi.waitFor(
        () => expect(sectionRows(app.container)[1]).toEqual(['Metrics', '']),
        { timeout: GENUI_DEBOUNCE_MS + 2000 },
      )
    } finally {
      await app.unmount()
    }
  })

  it('jumps from each section to its own heading, including a duplicate, in rich text', async () => {
    const markdown = `Overview
========

First overview body.

Overview
--------

Second overview body.

Metrics
-------

Body.
`
    const view = viewFile([
      { heading: 'Overview', blocks: [SUMMARY] },
      { heading: 'Overview', occurrence: 2, blocks: [SUMMARY] },
      { heading: 'Metrics', blocks: [SUMMARY] },
    ])
    const app = await mountApp(markdown, createBrowserHost(markdown, 'report.md', { view }))
    try {
      await vi.waitFor(() => expect(sectionRows(app.container)).toHaveLength(3))
      const buttons = genui().getByRole('button', { name: 'Overview' })

      await buttons.nth(1).click()
      await vi.waitFor(() => expect(caretHeadingIndex(app.container)).toBe(1))

      await genui().getByRole('button', { name: 'Metrics' }).click()
      await vi.waitFor(() => expect(caretHeadingIndex(app.container)).toBe(2))

      await buttons.nth(0).click()
      await vi.waitFor(() => expect(caretHeadingIndex(app.container)).toBe(0))
    } finally {
      await app.unmount()
    }
  })

  it('jumps to a section heading in source mode', async () => {
    const view = viewFile([{ heading: 'Metrics', blocks: [SUMMARY] }])
    const app = await mountApp(DOCUMENT, createBrowserHost(DOCUMENT, 'report.md', { view }))
    try {
      await enterSourceMode(app.container)
      await genui().getByRole('button', { name: 'Metrics' }).click()

      await vi.waitFor(() => {
        expect(caretLineInSource(app.container)?.textContent).toBe('# Metrics')
        expect(document.activeElement?.closest('.cm-editor')).not.toBeNull()
      })
    } finally {
      await app.unmount()
    }
  })

  it('stops following the view file while GenUI is off', async () => {
    const base = createBrowserHost(DOCUMENT, 'report.md')
    let following = 0
    const host: HostAdapter = {
      ...base,
      watchView(fn) {
        following += 1
        const watch = base.watchView(fn)
        return {
          dispose() {
            following -= 1
            watch.dispose()
          },
        }
      },
    }
    const app = await mountApp(DOCUMENT, host)
    try {
      expect(following).toBe(1)
      const toggle = page.getByTestId('genui-toggle')

      await toggle.click()
      await vi.waitFor(() => expect(toggle).toHaveAttribute('aria-pressed', 'false'))
      expect(following).toBe(0)
      base.writeView(viewFile([{ heading: 'Metrics', blocks: [SUMMARY] }]))

      await toggle.click()
      await vi.waitFor(() => expect(sectionRows(app.container)).toEqual([['Metrics', '']]))
      expect(following).toBe(1)
    } finally {
      await app.unmount()
    }
  })
})
