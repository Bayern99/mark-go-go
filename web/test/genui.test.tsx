import '../src/styles.css'
import { page, userEvent } from 'vitest/browser'
import { describe, expect, it, vi } from 'vitest'
import { createBrowserHost } from '../src/host/browserHost'
import type { HostAdapter } from '../src/host/types'
import { isRecord } from '../src/lib/guards'
import { extractSections } from '../src/markdown/sections'
import { CATALOG_COMPONENTS } from '../src/plugins/genui/a2ui'
import { SOURCE_DEBOUNCE_MS } from '../src/plugins/genui/sources'
import formatDoc from '../src/plugins/genui/view-format.md?raw'
import { mountApp } from './helpers/app'
import { getSourceView, replaceSourceText, visibleSource } from './helpers/source'

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

type Component = Record<string, unknown> & { id: string }

/** The format example with its component list changed by `edit`. */
function exampleWith(edit: (components: Component[]) => Component[]): string {
  const file: unknown = JSON.parse(formatExample())
  if (!isRecord(file) || !Array.isArray(file.a2ui)) throw new Error('unexpected example shape')
  const first: unknown = file.a2ui[0]
  if (!isRecord(first) || !isRecord(first.createSurface)) throw new Error('unexpected example shape')
  const components = first.createSurface.components
  if (!Array.isArray(components)) throw new Error('unexpected example shape')
  first.createSurface.components = edit(components.filter(isComponent))
  return JSON.stringify(file)
}

function isComponent(value: unknown): value is Component {
  return isRecord(value) && typeof value.id === 'string'
}

function replaceComponent(id: string, next: Component): (components: Component[]) => Component[] {
  return (components) => components.map((component) => (component.id === id ? next : component))
}

function viewMode(mode: 'richtext' | 'markdown' | 'genui') {
  return page.getByTestId(`view-mode-${mode}`)
}

async function showMode(mode: 'richtext' | 'markdown' | 'genui'): Promise<void> {
  await viewMode(mode).click()
  await vi.waitFor(() => expect(viewMode(mode)).toHaveAttribute('aria-checked', 'true'))
}

function genui() {
  return page.getByTestId('genui-view')
}

function surface(container: HTMLElement): HTMLElement {
  const element = container.querySelector<HTMLElement>('.genui-surface')
  if (!element) throw new Error('no rendered view')
  return element
}

function reportHost(view?: string) {
  return createBrowserHost(DOCUMENT, 'report.md', view === undefined ? {} : { view })
}

describe('GenUI view mode', () => {
  it('offers Generate when the document has no view', async () => {
    const app = await mountApp(DOCUMENT)
    try {
      await showMode('genui')
      await expect.element(genui().getByText('No view yet.')).toBeVisible()
      await expect.element(genui().getByRole('button', { name: 'Generate' })).toBeEnabled()
    } finally {
      await app.unmount()
    }
  })

  it('renders the format document example, which uses every catalog component', async () => {
    const used = new Set<string>()
    exampleWith((components) => {
      for (const component of components) used.add(String(component.component))
      return components
    })
    expect([...used].sort()).toEqual([...CATALOG_COMPONENTS].sort())

    const app = await mountApp(DOCUMENT, reportHost(formatExample()))
    try {
      await showMode('genui')
      await vi.waitFor(() => expect(genui()).toHaveAttribute('data-state', 'view'))
      expect(app.container.querySelector<HTMLElement>('.editor-pane')?.hidden).toBe(true)
      expect(genui().getByRole('alert').query()).toBeNull()
      await expect.element(genui().getByText('gemini-3.8-flash', { exact: false })).toBeVisible()

      await expect.element(genui().getByText('In short')).toBeVisible()
      await expect.element(genui().getByText('Only the edited block is written again.')).toBeVisible()
      await expect.element(genui().getByRole('checkbox', { name: 'Open a real file through the host.' })).toBeVisible()
      await expect.element(genui().getByText('Documents edited')).toBeVisible()
      const bold = surface(app.container).querySelector('strong')
      expect(bold?.textContent).toBe('untouched blocks')

      await genui().getByText('Details').click()
      await expect.element(genui().getByRole('columnheader', { name: /Size \(MB\)/ })).toBeVisible()
      await expect.element(genui().getByText('First prototype')).toBeVisible()
    } finally {
      await app.unmount()
    }
  })

  it('changes only the display when the reader uses the view', async () => {
    const host = reportHost(formatExample())
    const app = await mountApp(DOCUMENT, host)
    const update = vi.spyOn(host, 'update')
    try {
      await showMode('genui')
      await genui().getByText('Details').click()
      await genui().getByRole('button', { name: /Size \(MB\)/ }).click()
      await expect.element(genui().getByText('History')).toBeVisible()
      await genui().getByText('Summary').click()
      await expect.element(genui().getByText('In short')).toBeVisible()

      await new Promise((resolve) => setTimeout(resolve, 350))
      expect(update).not.toHaveBeenCalled()
      expect(host.getState().edited).toBe(false)
    } finally {
      await app.unmount()
    }
  })

  it('shows an unknown component as an error where it is and renders the rest', async () => {
    const view = exampleWith(
      replaceComponent('nextSteps', { id: 'nextSteps', component: 'Chart', series: [1, 2] }),
    )
    const app = await mountApp(DOCUMENT, reportHost(view))
    try {
      await showMode('genui')
      await vi.waitFor(() => expect(genui()).toHaveAttribute('data-state', 'view'))
      const unknown = app.container.querySelector('.genui-unknown')
      expect(unknown?.textContent).toContain('“Chart”')
      expect(unknown?.closest('.genui-surface')).not.toBeNull()
      await expect.element(genui().getByText('Only the edited block is written again.')).toBeVisible()
    } finally {
      await app.unmount()
    }
  })

  it('shows Markdown text without links, images or HTML', async () => {
    const view = exampleWith(
      replaceComponent('lead', {
        id: 'lead',
        component: 'Text',
        text: 'See [the docs](https://example.test) ![chart](https://example.test/c.png) <b>raw</b> **bold** and `code`.',
      }),
    )
    const app = await mountApp(DOCUMENT, reportHost(view))
    try {
      await showMode('genui')
      await expect.element(genui().getByText('the docs')).toBeVisible()
      const shown = surface(app.container)
      expect(shown.querySelector('a, img, b')).toBeNull()
      expect(shown.querySelector('.genui-link')?.textContent).toBe('the docs')
      expect(shown.querySelector('strong')?.textContent).toBe('bold')
      expect(shown.querySelector('code')?.textContent).toBe('code')
    } finally {
      await app.unmount()
    }
  })

  it('follows the view file: errors, older views, removal', async () => {
    const host = reportHost()
    const app = await mountApp(DOCUMENT, host)
    try {
      await showMode('genui')
      await vi.waitFor(() => expect(genui()).toHaveAttribute('data-state', 'none'))

      const cases: Array<[string, string]> = [
        ['{ not json', 'The view file is not valid JSON.'],
        [
          JSON.stringify({ version: 1, sections: [] }),
          'This view was made by an older version of Mark Go Go. Generate it again.',
        ],
        [JSON.stringify({ version: 3, a2ui: [] }), 'version 3'],
        [JSON.stringify({ version: 2 }), '"a2ui"'],
      ]
      for (const [text, message] of cases) {
        host.writeView(text)
        await vi.waitFor(() => {
          expect(genui()).toHaveAttribute('data-state', 'error')
          expect(app.container.querySelector('.genui-file-error')?.textContent).toContain(message)
        })
      }

      host.writeView(formatExample())
      await expect.element(genui().getByText('In short')).toBeVisible()

      // A reference to no component makes the whole view one error.
      host.writeView(
        exampleWith(replaceComponent('nextCard', { id: 'nextCard', component: 'Card', child: 'missing' })),
      )
      await vi.waitFor(() => {
        expect(genui()).toHaveAttribute('data-state', 'error')
        expect(app.container.querySelector('.genui-surface')).toBeNull()
      })

      host.writeView(null)
      await expect.element(genui().getByText('No view yet.')).toBeVisible()
    } finally {
      await app.unmount()
    }
  })

  it('follows the view file only while GenUI is shown', async () => {
    const base = reportHost()
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
      expect(following).toBe(0)
      await showMode('genui')
      await vi.waitFor(() => expect(following).toBe(1))

      await showMode('markdown')
      expect(following).toBe(0)
      base.writeView(formatExample())

      await showMode('genui')
      await expect.element(genui().getByText('In short')).toBeVisible()
      expect(following).toBe(1)
    } finally {
      await app.unmount()
    }
  })

  it('keeps source edits and the editor mode across a visit to GenUI', async () => {
    const app = await mountApp(DOCUMENT)
    const update = vi.spyOn(app.host, 'update')
    try {
      await showMode('markdown')
      replaceSourceText(getSourceView(app.container), 'Second body.', 'Second body, edited.')
      await showMode('genui')
      await vi.waitFor(
        () => expect(update.mock.lastCall?.[0]).toContain('Second body, edited.'),
        { timeout: 3000 },
      )

      await showMode('markdown')
      expect(visibleSource(app.container)).toContain('Second body, edited.')
      expect(app.container.querySelector<HTMLElement>('.editor-pane')?.hidden).toBe(false)
    } finally {
      await app.unmount()
    }
  })

  it('returns to the rich text scroll position', async () => {
    const long = `# Long\n\n${Array.from({ length: 200 }, (_, index) => `Paragraph ${index}.`).join('\n\n')}\n`
    const app = await mountApp(long)
    try {
      const scroller = app.container.querySelector<HTMLElement>('.editor-scroll')
      if (!scroller) throw new Error('editor scroller missing')
      scroller.scrollTop = 1200
      await vi.waitFor(() => expect(scroller.scrollTop).toBe(1200))

      await showMode('genui')
      await showMode('richtext')
      await vi.waitFor(() => expect(scroller.scrollTop).toBe(1200))
    } finally {
      await app.unmount()
    }
  })

  it('remembers the view mode for the document', async () => {
    const first = await mountApp(DOCUMENT, reportHost(formatExample()))
    try {
      await showMode('genui')
    } finally {
      await first.unmount()
    }

    const second = await mountApp(DOCUMENT, reportHost(formatExample()))
    try {
      await vi.waitFor(() => expect(viewMode('genui')).toHaveAttribute('aria-checked', 'true'))
      await expect.element(genui().getByText('In short')).toBeVisible()
    } finally {
      await second.unmount()
    }
  })
})

/** Plain text of the rich-text selection, blocks joined by `|`. */
function richSelection(container: HTMLElement): string {
  const selection = window.getSelection()
  const editor = container.querySelector('[data-testid="editor-root"] .ProseMirror')
  if (!selection || !editor?.contains(selection.anchorNode)) return ''
  return selection.toString().replace(/\n+/g, '|').trim()
}

const SOURCED = `# Overview

First body.

# Summary

Early summary.

# Metrics

Second body.

# Summary

Late summary.
`

/**
 * A view of `markdown` whose sources carry the section hashes the host
 * would record: one KeyPoints per entry, each naming its sources.
 */
function sourcedView(
  markdown: string,
  points: Array<{ id: string; sources: string[] }>,
  table: Record<string, { heading: string; occurrence?: number; stamp?: boolean }>,
): string {
  const sections = extractSections(markdown)
  const sources = Object.fromEntries(
    Object.entries(table).map(([id, ref]) => {
      const occurrence = ref.occurrence ?? 1
      const section = sections.find(
        (candidate) => candidate.heading.text === ref.heading && candidate.occurrence === occurrence,
      )
      if (!section) throw new Error(`no section ${ref.heading} #${occurrence}`)
      const hash = ref.stamp === false ? undefined : section.hash
      return [id, { heading: ref.heading, occurrence, hash }]
    }),
  )
  const components = [
    { id: 'root', component: 'Column', children: points.map((point) => point.id) },
    ...points.map((point) => ({
      id: point.id,
      component: 'KeyPoints',
      title: point.id,
      items: [`Point of ${point.id}.`],
      sources: point.sources,
    })),
  ]
  return JSON.stringify({
    version: 2,
    sources,
    a2ui: [
      {
        version: '1.0',
        createSurface: { surfaceId: 'view', catalogId: 'markgogo.dev:genui/v1', components },
      },
    ],
  })
}

function block(container: HTMLElement, title: string): HTMLElement {
  const heading = Array.from(container.querySelectorAll<HTMLElement>('.genui-keypoints h4')).find(
    (candidate) => candidate.textContent === title,
  )
  const article = heading?.closest<HTMLElement>('.genui-keypoints')
  if (!article) throw new Error(`no component titled ${title}`)
  return article
}

function statuses(container: HTMLElement): Record<string, string> {
  return Object.fromEntries(
    Array.from(container.querySelectorAll<HTMLElement>('.genui-keypoints')).map((article) => [
      article.querySelector('h4')?.textContent ?? '',
      article.querySelector('.genui-source-status')?.textContent ?? '',
    ]),
  )
}

describe('GenUI source references', () => {
  const POINTS = [
    { id: 'overview', sources: ['overview'] },
    { id: 'metrics', sources: ['metrics'] },
    { id: 'both', sources: ['overview', 'metrics'] },
    { id: 'late', sources: ['lateSummary'] },
  ]
  const TABLE = {
    overview: { heading: 'Overview' },
    metrics: { heading: 'Metrics' },
    lateSummary: { heading: 'Summary', occurrence: 2 },
  }

  it('marks only the components whose sources changed, and missing headings', async () => {
    const host = createBrowserHost(SOURCED, 'report.md', { view: sourcedView(SOURCED, POINTS, TABLE) })
    const app = await mountApp(SOURCED, host)
    try {
      await showMode('genui')
      await vi.waitFor(() =>
        expect(statuses(app.container)).toEqual({ overview: '', metrics: '', both: '', late: '' }),
      )
      expect(block(app.container, 'late').textContent).toContain('From Summary')

      // The edits come from the source, as an agent or another editor would make them.
      host.reload(SOURCED.replace('Second body.', 'Second body, edited.'))
      await vi.waitFor(
        () =>
          expect(statuses(app.container)).toEqual({
            overview: '',
            metrics: 'Source changed',
            both: 'Source changed',
            late: '',
          }),
        { timeout: SOURCE_DEBOUNCE_MS + 2000 },
      )

      host.reload(SOURCED.replace('Second body.', 'Second body, edited.').replace('# Overview', '# Intro'))
      await vi.waitFor(
        () =>
          expect(statuses(app.container)).toEqual({
            overview: 'Source not found',
            metrics: 'Source changed',
            both: 'Source not found',
            late: '',
          }),
        { timeout: SOURCE_DEBOUNCE_MS + 2000 },
      )
      const missing = block(app.container, 'overview')
      expect(missing.textContent).toContain('Point of overview.')
      expect(missing.querySelector<HTMLButtonElement>('.genui-source-edit')?.disabled).toBe(true)

      host.reload(SOURCED)
      await vi.waitFor(
        () =>
          expect(statuses(app.container)).toEqual({ overview: '', metrics: '', both: '', late: '' }),
        { timeout: SOURCE_DEBOUNCE_MS + 2000 },
      )
    } finally {
      await app.unmount()
    }
  })

  it('shows no status for a hand-written view without recorded hashes', async () => {
    const view = sourcedView(SOURCED, [{ id: 'metrics', sources: ['metrics'] }], {
      metrics: { heading: 'Metrics', stamp: false },
    })
    const host = createBrowserHost(SOURCED, 'report.md', { view })
    const app = await mountApp(SOURCED, host)
    try {
      await showMode('genui')
      await vi.waitFor(() => expect(block(app.container, 'metrics').dataset.sourceStatus).toBe('unknown'))
      host.reload(SOURCED.replace('Second body.', 'Second body, edited.'))
      await new Promise((resolve) => setTimeout(resolve, SOURCE_DEBOUNCE_MS + 100))
      expect(statuses(app.container)).toEqual({ metrics: '' })
    } finally {
      await app.unmount()
    }
  })

  it('edits in source and comes back to the same place, the edit marked', async () => {
    const filler = Array.from({ length: 30 }, (_, index) => ({ id: `filler${index}`, sources: ['overview'] }))
    const view = sourcedView(SOURCED, [...filler, { id: 'late', sources: ['lateSummary'] }], TABLE)
    const app = await mountApp(SOURCED, createBrowserHost(SOURCED, 'report.md', { view }))
    try {
      await showMode('genui')
      const late = await vi.waitFor(() => block(app.container, 'late'))
      const scroller = app.container.querySelector<HTMLElement>('.center-view')
      if (!scroller) throw new Error('center view scroller missing')
      late.scrollIntoView({ block: 'center' })
      const top = scroller.scrollTop
      expect(top).toBeGreaterThan(200)

      const edit = late.querySelector<HTMLButtonElement>('.genui-source-edit')
      if (!edit) throw new Error('edit button missing')
      edit.click()
      await vi.waitFor(() => {
        expect(viewMode('richtext')).toHaveAttribute('aria-checked', 'true')
        // The second "Summary", not the first one with the same text.
        expect(richSelection(app.container)).toBe('Summary|Late summary.')
      })

      const paragraph = Array.from(
        app.container.querySelectorAll<HTMLElement>('[data-testid="editor-root"] .ProseMirror p'),
      ).find((element) => element.textContent === 'Late summary.')
      if (!paragraph) throw new Error('late summary paragraph missing')
      await userEvent.click(paragraph)
      await userEvent.keyboard('{End} Revised.')

      await page.getByTestId('return-to-view').click()
      await vi.waitFor(() => {
        expect(viewMode('genui')).toHaveAttribute('aria-checked', 'true')
        const again = app.container.querySelector<HTMLElement>('.center-view')
        expect(Math.abs((again?.scrollTop ?? 0) - top)).toBeLessThan(2)
        expect(statuses(app.container).late).toBe('Source changed')
        expect(statuses(app.container).filler0).toBe('')
      }, { timeout: SOURCE_DEBOUNCE_MS + 2000 })
      expect(page.getByTestId('return-to-view').query()).toBeNull()
    } finally {
      await app.unmount()
    }
  })

  it('offers the way back only until the reader picks a mode', async () => {
    const view = sourcedView(SOURCED, [{ id: 'metrics', sources: ['metrics'] }], TABLE)
    const app = await mountApp(SOURCED, createBrowserHost(SOURCED, 'report.md', { view }))
    try {
      await showMode('genui')
      const edit = await vi.waitFor(() => {
        const button = block(app.container, 'metrics').querySelector<HTMLButtonElement>('.genui-source-edit')
        if (!button) throw new Error('edit button missing')
        return button
      })
      edit.click()
      await vi.waitFor(() =>
        expect(page.getByTestId('return-to-view').element().textContent).toContain('Back to GenUI'),
      )
      await showMode('markdown')
      expect(page.getByTestId('return-to-view').query()).toBeNull()
    } finally {
      await app.unmount()
    }
  })
})
