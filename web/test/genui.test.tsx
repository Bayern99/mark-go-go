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
import { scrollViewport } from './helpers/scroll'
import { getSourceView, replaceSourceText, visibleSource } from './helpers/source'

const DOCUMENT = `# Overview

First body.

# Metrics

Second body.

# Next steps

Third body.
`

function formatExample(): string {
  const match = /```json\n([\s\S]*?)\n```/.exec(formatDoc)
  if (!match?.[1]) throw new Error('view-format.md has no JSON example')
  return match[1]
}

type Component = Record<string, unknown> & { id: string }

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

function viewMode(mode: 'preview' | 'markdown' | 'genui') {
  return page.getByTestId(`view-mode-${mode}`)
}

async function showMode(mode: 'preview' | 'markdown' | 'genui'): Promise<void> {
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
  it('opens generation settings from the empty center view while preserving the selected right panel', async () => {
    const app = await mountApp(DOCUMENT)
    try {
      await showMode('genui')
      await expect.element(genui().getByText('No view yet.')).toBeVisible()
      expect(genui().getByRole('button', { name: 'Generate', exact: true }).query()).toBeNull()
      expect(genui().getByRole('textbox').query()).toBeNull()
      expect(app.container.querySelector('[data-testid="genui-view"] [data-testid="model-menu-trigger"]')).toBeNull()

      const rail = page.getByTestId('right-pane')
      const outline = rail.getByRole('tab', { name: 'Outline' })
      await expect.element(outline).toHaveAttribute('aria-selected', 'true')
      expect(rail.getByRole('tab', { name: 'GenUI' }).query()).toBeNull()
      await genui().getByRole('button', { name: 'Set up generation' }).click()
      const panel = page.getByTestId('genui-panel')
      await expect.element(panel).toBeVisible()
      await expect.element(panel.getByText('No GenUI view yet')).toBeVisible()
      await expect.element(panel.getByRole('button', { name: 'Generate', exact: true })).toBeEnabled()
      expect(app.container.querySelector('[data-testid="center-pane"]')?.contains(panel.element())).toBe(true)
      await expect.element(outline).toHaveAttribute('aria-selected', 'true')
      panel.getByRole('button', { name: 'Generate', exact: true }).element().focus()
      await userEvent.keyboard('{Escape}')
      await expect.poll(() => panel.query()).toBeNull()
      await expect.element(page.getByTestId('genui-settings-toggle')).toHaveFocus()
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
      expect(genui().getByRole('button', { name: /Generate|Regenerate/ }).query()).toBeNull()
      expect(app.container.querySelector('[data-testid="genui-view"] [data-testid="model-menu-trigger"]')).toBeNull()

      await expect.element(genui().getByText('In short')).toBeVisible()
      await expect.element(genui().getByText('Only the edited block is written again.')).toBeVisible()
      await expect.element(genui().getByRole('checkbox', { name: 'Open a real file through the host.' })).toBeVisible()
      await expect.element(genui().getByText('Documents edited')).toBeVisible()
      const bold = surface(app.container).querySelector('strong')
      expect(bold?.textContent).toBe('untouched blocks')

      await genui().getByText('Details').click()
      await expect.element(genui().getByRole('columnheader', { name: /Size \(MB\)/ })).toBeVisible()
      await expect.element(genui().getByText('First prototype')).toBeVisible()

      await page.getByTestId('genui-settings-toggle').click()
      const footer = page.getByTestId('genui-panel-footer')
      await expect.element(footer.getByText('Antigravity (agy)')).toBeVisible()
      await expect.element(footer.getByText('gemini-3.8-flash', { exact: false })).toBeVisible()
      await expect.element(footer.getByText('2026-11-06T09:30:00Z', { exact: false })).toBeVisible()
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
      await expect.element(genui().getByRole('heading', { name: 'History', exact: true })).toBeVisible()
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
        await expect.element(page.getByTestId('view-mode-dot-genui')).toHaveAttribute('data-tone', 'danger')
        await expect.element(page.getByTestId('view-mode-dot-genui')).toHaveAttribute('title', 'GenUI view error')
      }

      host.writeView(formatExample())
      await expect.element(genui().getByText('In short')).toBeVisible()
      await expect.element(page.getByTestId('view-mode-dot-genui')).toHaveAttribute('data-tone', 'warn')

      host.writeView(
        exampleWith(replaceComponent('nextCard', { id: 'nextCard', component: 'Card', child: 'missing' })),
      )
      await vi.waitFor(() => {
        expect(genui()).toHaveAttribute('data-state', 'error')
        expect(app.container.querySelector('.genui-surface')).toBeNull()
      })
      await expect.element(page.getByTestId('view-mode-dot-genui')).toHaveAttribute('data-tone', 'danger')

      host.writeView(null)
      await expect.element(genui().getByText('No view yet.')).toBeVisible()
      expect(page.getByTestId('view-mode-dot-genui').query()).toBeNull()
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

  it('returns to the Preview scroll position', async () => {
    const long = `# Long\n\n${Array.from({ length: 200 }, (_, index) => `Paragraph ${index}.`).join('\n\n')}\n`
    const app = await mountApp(long)
    try {
      const scroller = app.container.querySelector<HTMLElement>('.editor-scroll')
      if (!scroller) throw new Error('editor scroller missing')
      const viewport = scrollViewport(scroller)
      viewport.scrollTop = 1200
      await vi.waitFor(() => expect(viewport.scrollTop).toBe(1200))

      await showMode('genui')
      await showMode('preview')
      await vi.waitFor(() => expect(viewport.scrollTop).toBe(1200))
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
      expect(missing.querySelector<HTMLButtonElement>('.genui-source-note')?.disabled).toBe(true)
      expect(missing.querySelector('.genui-source-unavailable')?.textContent).toContain(
        'None of this part\'s sources are in the document now.',
      )
      const both = block(app.container, 'both')
      expect(both.querySelector('.genui-source-status')?.textContent).toBe('Source not found')
      expect(both.querySelector<HTMLButtonElement>('.genui-source-edit')?.disabled).toBe(false)
      expect(both.querySelector<HTMLButtonElement>('.genui-source-note')?.disabled).toBe(false)
      expect(both.querySelector('.genui-source-unavailable')).toBeNull()

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
      const top = scrollViewport(scroller).scrollTop
      expect(top).toBeGreaterThan(200)

      const edit = late.querySelector<HTMLButtonElement>('.genui-source-edit')
      if (!edit) throw new Error('edit button missing')
      edit.click()
      await vi.waitFor(() => {
        expect(viewMode('preview')).toHaveAttribute('aria-checked', 'true')
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
        if (!again) throw new Error('center view scroller missing')
        expect(Math.abs(scrollViewport(again).scrollTop - top)).toBeLessThan(2)
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

  it('lists out-of-date components in settings, marks GenUI mode, and reveals the chosen card', async () => {
    const host = createBrowserHost(SOURCED, 'report.md', { view: sourcedView(SOURCED, POINTS, TABLE) })
    const app = await mountApp(SOURCED, host)
    try {
      await showMode('genui')
      await page.getByTestId('genui-settings-toggle').click()

      expect(page.getByTestId('view-mode-dot-genui').query()).toBeNull()
      const panel = page.getByTestId('genui-panel')
      await expect.element(panel.getByText('Up to date')).toBeVisible()

      host.reload(SOURCED.replace('Second body.', 'Second body, edited.'))
      await vi.waitFor(
        () => {
          expect(panel.element().textContent).toContain('2 components out of date')
        },
        { timeout: SOURCE_DEBOUNCE_MS + 2000 },
      )

      const dot = page.getByTestId('view-mode-dot-genui')
      await expect.element(dot).toHaveAttribute('data-tone', 'warn')

      const metricsCard = app.container.querySelector<HTMLElement>(
        '[data-testid="genui-view"] [data-component-id="metrics"]',
      )
      expect(metricsCard).not.toBeNull()
      if (!metricsCard) throw new Error('metrics card missing')
      const scrollSpy = vi.spyOn(metricsCard, 'scrollIntoView')

      const item = panel.getByTestId('changed-item-metrics')
      await item.click()

      await expect.poll(() => panel.query()).toBeNull()
      expect(scrollSpy).toHaveBeenCalled()
      expect(metricsCard.getAttribute('data-highlighted')).toBe('true')
      expect(metricsCard.classList.contains('is-highlighted')).toBe(true)
    } finally {
      await app.unmount()
    }
  })

  it('edits the remaining source when the first heading is gone', async () => {
    const host = createBrowserHost(SOURCED, 'report.md', { view: sourcedView(SOURCED, POINTS, TABLE) })
    const app = await mountApp(SOURCED, host)
    try {
      await showMode('genui')
      host.reload(SOURCED.replace('# Overview', '# Intro'))
      const both = await vi.waitFor(
        () => {
          const card = block(app.container, 'both')
          expect(card.querySelector('.genui-source-status')?.textContent).toBe('Source not found')
          expect(card.querySelector<HTMLButtonElement>('.genui-source-edit')?.disabled).toBe(false)
          return card
        },
        { timeout: SOURCE_DEBOUNCE_MS + 2000 },
      )
      both.querySelector<HTMLButtonElement>('.genui-source-edit')?.click()
      await vi.waitFor(() => {
        expect(viewMode('preview')).toHaveAttribute('aria-checked', 'true')
        expect(richSelection(app.container)).toBe('Metrics|Second body.')
      })
    } finally {
      await app.unmount()
    }
  })

  it('edits the first remaining source when a later heading is gone', async () => {
    const host = createBrowserHost(SOURCED, 'report.md', { view: sourcedView(SOURCED, POINTS, TABLE) })
    const app = await mountApp(SOURCED, host)
    try {
      await showMode('genui')
      host.reload(SOURCED.replace('# Metrics', '# Figures'))
      const both = await vi.waitFor(
        () => {
          const card = block(app.container, 'both')
          expect(card.querySelector('.genui-source-status')?.textContent).toBe('Source not found')
          expect(card.querySelector<HTMLButtonElement>('.genui-source-edit')?.disabled).toBe(false)
          return card
        },
        { timeout: SOURCE_DEBOUNCE_MS + 2000 },
      )
      both.querySelector<HTMLButtonElement>('.genui-source-edit')?.click()
      await vi.waitFor(() => {
        expect(viewMode('preview')).toHaveAttribute('aria-checked', 'true')
        expect(richSelection(app.container)).toBe('Overview|First body.')
      })
    } finally {
      await app.unmount()
    }
  })

  it('disables Edit and Note only when every source is gone, then restores them', async () => {
    const host = createBrowserHost(SOURCED, 'report.md', { view: sourcedView(SOURCED, POINTS, TABLE) })
    const app = await mountApp(SOURCED, host)
    try {
      await showMode('genui')
      host.reload(SOURCED.replace('# Overview', '# Intro').replace('# Metrics', '# Figures'))
      const both = await vi.waitFor(
        () => {
          const card = block(app.container, 'both')
          expect(card.querySelector('.genui-source-status')?.textContent).toBe('Source not found')
          expect(card.querySelector<HTMLButtonElement>('.genui-source-edit')?.disabled).toBe(true)
          return card
        },
        { timeout: SOURCE_DEBOUNCE_MS + 2000 },
      )
      expect(both.querySelector<HTMLButtonElement>('.genui-source-note')?.disabled).toBe(true)
      expect(both.querySelector('.genui-source-unavailable')?.textContent).toContain(
        "None of this part's sources are in the document now.",
      )
      both.querySelector<HTMLButtonElement>('.genui-source-edit')?.click()
      expect(viewMode('genui')).toHaveAttribute('aria-checked', 'true')

      host.reload(SOURCED)
      await vi.waitFor(
        () => {
          expect(statuses(app.container)).toEqual({ overview: '', metrics: '', both: '', late: '' })
          expect(block(app.container, 'both').querySelector<HTMLButtonElement>('.genui-source-edit')?.disabled).toBe(
            false,
          )
        },
        { timeout: SOURCE_DEBOUNCE_MS + 2000 },
      )
      expect(block(app.container, 'both').querySelector('.genui-source-unavailable')).toBeNull()
      block(app.container, 'both').querySelector<HTMLButtonElement>('.genui-source-edit')?.click()
      await vi.waitFor(() => {
        expect(viewMode('preview')).toHaveAttribute('aria-checked', 'true')
        expect(richSelection(app.container)).toBe('Overview|First body.')
      })
    } finally {
      await app.unmount()
    }
  })

  it('does not use another heading of the same text as a missing occurrence', async () => {
    const host = createBrowserHost(SOURCED, 'report.md', { view: sourcedView(SOURCED, POINTS, TABLE) })
    const app = await mountApp(SOURCED, host)
    try {
      await showMode('genui')
      host.reload(SOURCED.replace('# Summary\n\nLate summary.\n', ''))
      const late = await vi.waitFor(
        () => {
          const card = block(app.container, 'late')
          expect(card.querySelector('.genui-source-status')?.textContent).toBe('Source not found')
          expect(card.querySelector<HTMLButtonElement>('.genui-source-edit')?.disabled).toBe(true)
          return card
        },
        { timeout: SOURCE_DEBOUNCE_MS + 2000 },
      )
      expect(late.querySelector('.genui-source-unavailable')?.textContent).toContain(
        "None of this part's sources are in the document now.",
      )
    } finally {
      await app.unmount()
    }
  })

  it('edits the exact source whose heading is clicked, including repeated headings', async () => {
    const view = sourcedView(SOURCED, [{ id: 'summaries', sources: ['earlySummary', 'lateSummary'] }], {
      earlySummary: { heading: 'Summary', occurrence: 1 },
      lateSummary: { heading: 'Summary', occurrence: 2 },
    })
    const app = await mountApp(SOURCED, createBrowserHost(SOURCED, 'report.md', { view }))
    try {
      await showMode('genui')
      const card = await vi.waitFor(() => block(app.container, 'summaries'))
      const early = card.querySelector<HTMLButtonElement>(
        '.genui-source-heading[data-source-id="earlySummary"]',
      )
      const late = card.querySelector<HTMLButtonElement>(
        '.genui-source-heading[data-source-id="lateSummary"]',
      )
      if (!early || !late) throw new Error('source heading buttons missing')
      expect(early.textContent).toBe('Summary')
      expect(late.textContent).toBe('Summary')
      expect(early.disabled).toBe(false)
      expect(late.disabled).toBe(false)

      late.click()
      await vi.waitFor(() => {
        expect(viewMode('preview')).toHaveAttribute('aria-checked', 'true')
        expect(richSelection(app.container)).toBe('Summary|Late summary.')
      })

      await showMode('genui')
      const again = await vi.waitFor(() => block(app.container, 'summaries'))
      again
        .querySelector<HTMLButtonElement>('.genui-source-heading[data-source-id="earlySummary"]')
        ?.click()
      await vi.waitFor(() => {
        expect(viewMode('preview')).toHaveAttribute('aria-checked', 'true')
        expect(richSelection(app.container)).toBe('Summary|Early summary.')
      })
    } finally {
      await app.unmount()
    }
  })

  it('moves with the keyboard to the source heading it picked', async () => {
    const host = createBrowserHost(SOURCED, 'report.md', { view: sourcedView(SOURCED, POINTS, TABLE) })
    const app = await mountApp(SOURCED, host)
    try {
      await showMode('genui')
      const both = await vi.waitFor(() => block(app.container, 'both'))
      const metrics = both.querySelector<HTMLButtonElement>(
        '.genui-source-heading[data-source-id="metrics"]',
      )
      if (!metrics) throw new Error('metrics heading button missing')
      metrics.focus()
      await userEvent.keyboard('{Enter}')
      await vi.waitFor(() => {
        expect(viewMode('preview')).toHaveAttribute('aria-checked', 'true')
        expect(richSelection(app.container)).toBe('Metrics|Second body.')
      })
    } finally {
      await app.unmount()
    }
  })

  it('keeps a missing source heading disabled instead of redirecting to a surviving sibling', async () => {
    const view = sourcedView(SOURCED, POINTS, TABLE)
    const host = createBrowserHost(SOURCED, 'report.md', { view })
    let viewText = ''
    const stop = host.watchView((snapshot) => {
      viewText = snapshot.text
    })
    const app = await mountApp(SOURCED, host)
    try {
      await showMode('genui')
      await vi.waitFor(() => expect(viewText).toBe(view))
      expect((await host.load()).text).toBe(SOURCED)

      host.reload(SOURCED.replace('# Overview', '# Intro'))
      const both = await vi.waitFor(
        () => {
          const card = block(app.container, 'both')
          expect(
            card.querySelector<HTMLButtonElement>('.genui-source-heading[data-source-id="overview"]')
              ?.disabled,
          ).toBe(true)
          return card
        },
        { timeout: SOURCE_DEBOUNCE_MS + 2000 },
      )
      const missing = both.querySelector<HTMLButtonElement>(
        '.genui-source-heading[data-source-id="overview"]',
      )
      const loaded = both.querySelector<HTMLButtonElement>(
        '.genui-source-heading[data-source-id="metrics"]',
      )
      if (!missing || !loaded) throw new Error('source heading buttons missing')
      expect(missing.getAttribute('data-source-available')).toBe('false')
      expect(loaded.disabled).toBe(false)

      missing.click()
      expect(viewMode('genui')).toHaveAttribute('aria-checked', 'true')
      expect(viewMode('preview')).toHaveAttribute('aria-checked', 'false')

      loaded.click()
      await vi.waitFor(() => {
        expect(viewMode('preview')).toHaveAttribute('aria-checked', 'true')
        expect(richSelection(app.container)).toBe('Metrics|Second body.')
      })
      expect((await host.load()).text).toBe(SOURCED.replace('# Overview', '# Intro'))
      expect(viewText).toBe(view)
    } finally {
      stop.dispose()
      await app.unmount()
    }
  })
})
