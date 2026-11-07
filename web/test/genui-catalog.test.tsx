import '../src/styles.css'
import { page, userEvent } from 'vitest/browser'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createBrowserHost, type BrowserHost } from '../src/host/browserHost'
import { catalogDefinition, CATALOG_COMPONENTS } from '../src/plugins/genui/a2ui'
import { extractSections } from '../src/markdown/sections'
import { mountApp, type MountedApp } from './helpers/app'

const DOCUMENT = `# Plans

We compare three options.

- [x] Ship the editor
- [ ] Ship generation

# History

It began in March.
`

const LONG = 'Pneumonoultramicroscopicsilicovolcanoconiosis'.repeat(4)

type Component = Record<string, unknown> & { id: string; component: string }

function viewOf(components: Component[], { stamped = true } = {}): string {
  const sections = extractSections(DOCUMENT)
  const ref = (heading: string) => ({
    heading,
    hash: stamped ? sections.find((section) => section.heading.text === heading)?.hash : undefined,
  })
  return JSON.stringify({
    version: 2,
    sources: { plans: ref('Plans'), history: ref('History') },
    a2ui: [
      { version: '1.0', createSurface: { surfaceId: 'view', catalogId: 'markgogo.dev:genui/v1', components } },
    ],
  })
}

/** Every content component once, under a Row of two Cards so widths are shared. */
const EVERY: Component[] = [
  { id: 'root', component: 'Column', children: ['row', 'table', 'timeline'] },
  { id: 'row', component: 'Row', children: ['left', 'right'] },
  { id: 'left', component: 'Card', child: 'metrics' },
  { id: 'right', component: 'Card', child: 'checklist' },
  {
    id: 'metrics',
    component: 'MetricGrid',
    title: 'Figures',
    metrics: [
      { label: 'Size', value: '12', unit: 'MB', change: '-3 MB', trend: 'down' },
      { label: LONG, value: '1234567890123', change: '+4%', trend: 'up' },
    ],
    sources: ['plans'],
  },
  {
    id: 'checklist',
    component: 'Checklist',
    title: 'Tasks',
    items: [
      { text: 'Ship the editor', done: true },
      { text: 'Generation', quote: 'Ship generation' },
      { text: LONG },
    ],
    sources: ['plans'],
  },
  {
    id: 'table',
    component: 'ComparisonTable',
    title: 'Options',
    columns: ['Option', 'Size', 'Notes'],
    rows: [
      ['MyGo', '12', 'Native menus'],
      ['Electron', '180', LONG],
      ['Tauri', '9', 'Rust'],
    ],
    sources: ['plans'],
  },
  {
    id: 'timeline',
    component: 'Timeline',
    title: 'History',
    events: [
      { when: 'March', title: 'First prototype', detail: LONG },
      { when: 'November 2026, after review', title: 'GenUI' },
    ],
    sources: ['history'],
  },
]

let app: MountedApp | undefined
let lastHost: BrowserHost | undefined

afterEach(async () => {
  await app?.unmount()
  app = undefined
  lastHost = undefined
  document.documentElement.style.colorScheme = ''
  await page.viewport(1280, 900)
})

async function showView(components: Component[], options?: { stamped?: boolean }): Promise<MountedApp> {
  const host = createBrowserHost(DOCUMENT, 'plans.md', { view: viewOf(components, options) })
  const mounted = await mountApp(DOCUMENT, host)
  app = mounted
  lastHost = host
  await page.getByTestId('view-mode-genui').click()
  await vi.waitFor(() => {
    const state = mounted.container.querySelector('[data-testid="genui-view"]')?.getAttribute('data-state')
    expect(state === 'view' || state === 'error').toBe(true)
  })
  return mounted
}

function viewError(mounted: MountedApp): string {
  return mounted.container.querySelector('.genui-file-error')?.textContent ?? ''
}

function firstColumn(mounted: MountedApp): string[] {
  return Array.from(mounted.container.querySelectorAll('.genui-comparison tbody th')).map(
    (cell) => cell.textContent ?? '',
  )
}

describe('GenUI content components', () => {
  it('sorts a comparison by any column, numbers by value, and only on screen', async () => {
    const mounted = await showView(EVERY)
    const update = vi.spyOn(mounted.host, 'update')
    const order = (expected: string[]) => vi.waitFor(() => expect(firstColumn(mounted)).toEqual(expected))
    await order(['MyGo', 'Electron', 'Tauri'])

    const size = page.getByRole('button', { name: /^Size/ })
    await size.click()
    await order(['Tauri', 'MyGo', 'Electron'])
    await expect.element(page.getByRole('columnheader', { name: /^Size/ })).toHaveAttribute('aria-sort', 'ascending')
    await size.click()
    await order(['Electron', 'MyGo', 'Tauri'])
    await expect.element(page.getByRole('columnheader', { name: /^Size/ })).toHaveAttribute('aria-sort', 'descending')
    await size.click()
    await order(['MyGo', 'Electron', 'Tauri'])
    await expect.element(page.getByRole('columnheader', { name: /^Size/ })).toHaveAttribute('aria-sort', 'none')

    await page.getByRole('button', { name: /^Option/ }).click()
    await order(['Electron', 'MyGo', 'Tauri'])

    await new Promise((resolve) => setTimeout(resolve, 350))
    expect(update).not.toHaveBeenCalled()
  })

  it('names the table and headers, sorts from the keyboard, and stays read-only', async () => {
    const mounted = await showView(EVERY)
    const update = vi.spyOn(mounted.host, 'update')
    const order = (expected: string[]) => vi.waitFor(() => expect(firstColumn(mounted)).toEqual(expected))
    await order(['MyGo', 'Electron', 'Tauri'])

    await expect.element(page.getByRole('table', { name: 'Options table' })).toBeVisible()
    await expect.element(page.getByRole('group', { name: 'Options table, scrollable' })).toBeVisible()
    for (const name of ['Option', 'Size', 'Notes']) {
      await expect.element(page.getByRole('columnheader', { name })).toHaveAttribute('scope', 'col')
      await expect.element(page.getByRole('columnheader', { name })).toHaveAttribute('aria-sort', 'none')
      await expect.element(page.getByRole('button', { name })).toBeVisible()
    }
    await expect.element(page.getByRole('rowheader', { name: 'MyGo' })).toHaveAttribute('scope', 'row')

    const scroller = mounted.container.querySelector<HTMLElement>('.genui-table-scroll')
    const headers = Array.from(mounted.container.querySelectorAll<HTMLButtonElement>('.genui-comparison thead button'))
    if (!scroller || headers.length !== 3) throw new Error('comparison headers missing')
    scroller.focus()
    expect(document.activeElement).toBe(scroller)
    await userEvent.keyboard('{Tab}')
    expect(document.activeElement).toBe(headers[0])
    await userEvent.keyboard('{Tab}')
    expect(document.activeElement).toBe(headers[1])
    await userEvent.keyboard('{Tab}')
    expect(document.activeElement).toBe(headers[2])

    headers[1].focus()
    await userEvent.keyboard('{Enter}')
    await order(['Tauri', 'MyGo', 'Electron'])
    await expect.element(page.getByRole('columnheader', { name: 'Size' })).toHaveAttribute('aria-sort', 'ascending')
    await expect.element(page.getByRole('columnheader', { name: 'Option' })).toHaveAttribute('aria-sort', 'none')
    expect(document.activeElement).toBe(headers[1])
    await expect.element(page.getByRole('rowheader', { name: 'Tauri' })).toHaveAttribute('scope', 'row')

    await userEvent.keyboard('{Enter}')
    await order(['Electron', 'MyGo', 'Tauri'])
    await expect.element(page.getByRole('columnheader', { name: 'Size' })).toHaveAttribute('aria-sort', 'descending')

    await userEvent.keyboard('{Enter}')
    await order(['MyGo', 'Electron', 'Tauri'])
    await expect.element(page.getByRole('columnheader', { name: 'Size' })).toHaveAttribute('aria-sort', 'none')

    headers[0].focus()
    await userEvent.keyboard(' ')
    await order(['Electron', 'MyGo', 'Tauri'])
    await expect.element(page.getByRole('columnheader', { name: 'Option' })).toHaveAttribute('aria-sort', 'ascending')
    expect(document.activeElement).toBe(headers[0])

    await new Promise((resolve) => setTimeout(resolve, 350))
    expect(update).not.toHaveBeenCalled()
    expect(lastHost?.getState().edited).toBe(false)
  })

  it('names an untitled comparison table', async () => {
    await showView([
      {
        id: 'root',
        component: 'ComparisonTable',
        columns: ['A', 'B'],
        rows: [
          ['x', '1'],
          ['y', '2'],
        ],
      },
    ])
    await expect.element(page.getByRole('table', { name: 'Comparison table' })).toBeVisible()
    await expect.element(page.getByRole('group', { name: 'Comparison table, scrollable' })).toBeVisible()
    await expect.element(page.getByRole('columnheader', { name: 'A' })).toHaveAttribute('aria-sort', 'none')
  })

  it('shows checklist state read-only, and a click selects the item in the document', async () => {
    const mounted = await showView(EVERY)
    const update = vi.spyOn(mounted.host, 'update')
    const done = page.getByRole('checkbox', { name: 'Ship the editor' })
    await expect.element(done).toHaveAttribute('aria-checked', 'true')
    await expect.element(done).toHaveAttribute('aria-readonly', 'true')

    await page.getByRole('checkbox', { name: 'Generation' }).click()
    await vi.waitFor(() => {
      expect(page.getByTestId('view-mode-richtext').element().getAttribute('aria-checked')).toBe('true')
      expect(window.getSelection()?.toString()).toContain('Ship generation')
    })
    // The view never writes the document: the item stays unchecked.
    expect(update).not.toHaveBeenCalled()
    expect(lastHost?.getState().edited).toBe(false)
  })

  it('disables checklist items whose source is gone', async () => {
    const mounted = await showView([
      { id: 'root', component: 'Checklist', items: [{ text: 'Ship the editor' }], sources: ['missing'] },
    ])
    expect(viewError(mounted)).toBe('')
    await expect.element(page.getByRole('checkbox', { name: 'Ship the editor' })).toBeDisabled()
  })

  it('names the component and the reason when properties are wrong', async () => {
    const cases: Array<[Component, string]> = [
      [{ id: 'root', component: 'MetricGrid', metrics: [{ label: 'Size' }] }, 'metrics.0.value: Expected string, received undefined'],
      [
        { id: 'root', component: 'MetricGrid', metrics: [{ label: 'Size', value: '1', trend: 'sideways' }] },
        "metrics.0.trend: Invalid enum value. Expected up | down | flat, received 'sideways'",
      ],
      [
        { id: 'root', component: 'ComparisonTable', columns: ['A', 'B'], rows: [['1', '2'], ['3']] },
        'Row 2 has 1 cells; there are 2 columns.',
      ],
      [{ id: 'root', component: 'Timeline', events: [] }, 'events: Array must contain at least 1 element'],
      [{ id: 'root', component: 'Checklist', items: [{ text: 'x', checked: true }] }, "'checked'"],
    ]
    for (const [component, reason] of cases) {
      const mounted = await showView([component])
      expect(viewError(mounted)).toContain(`'${component.component}' (root)`)
      expect(viewError(mounted)).toContain(reason)
      await mounted.unmount()
      app = undefined
    }
  })

  it('refuses components placed against the nesting rules', async () => {
    const keyPoints = (id: string): Component => ({ id, component: 'KeyPoints', items: ['a'] })
    const cases: Array<[Component[], string]> = [
      [
        [
          { id: 'root', component: 'Card', child: 'tabs' },
          { id: 'tabs', component: 'Tabs', tabs: [{ title: 'A', child: 'k' }] },
          keyPoints('k'),
        ],
        "UNALLOWED_CHILD: Container 'root' (Card) cannot contain child 'tabs' (Tabs).",
      ],
      [
        [
          { id: 'root', component: 'Row', children: ['inner'] },
          { id: 'inner', component: 'Row', children: ['k'] },
          keyPoints('k'),
        ],
        "UNALLOWED_PARENT: Component 'inner' (Row) cannot be placed under parent 'root' (Row).",
      ],
    ]
    for (const [components, reason] of cases) {
      const mounted = await showView(components)
      expect(viewError(mounted)).toContain(reason)
      expect(viewError(mounted)).not.toContain('MgoUnknown')
      await mounted.unmount()
      app = undefined
    }
  })

  it('fits narrow and wide panes without overflowing, long words wrapping', async () => {
    for (const width of [560, 1600]) {
      await page.viewport(width, 900)
      const mounted = await showView(EVERY)
      const pane = mounted.container.querySelector<HTMLElement>('.center-view')
      if (!pane) throw new Error('no center view')
      await vi.waitFor(() => expect(mounted.container.querySelector('.genui-timeline')).not.toBeNull())
      expect(pane.scrollWidth, `pane at ${width}px`).toBeLessThanOrEqual(pane.clientWidth)
      for (const block of mounted.container.querySelectorAll<HTMLElement>('.genui-block')) {
        // Wide tables scroll inside their own frame; nothing else may overflow.
        expect(block.scrollWidth, `${block.className} at ${width}px`).toBeLessThanOrEqual(block.clientWidth + 1)
      }
      const scroller = mounted.container.querySelector<HTMLElement>('.genui-table-scroll')
      expect(scroller?.getBoundingClientRect().right).toBeLessThanOrEqual(pane.getBoundingClientRect().right)
      await mounted.unmount()
      app = undefined
    }
  })

  it('follows the light and dark colour scheme', async () => {
    const mounted = await showView(EVERY)
    const block = mounted.container.querySelector<HTMLElement>('.genui-metrics')
    if (!block) throw new Error('no metric grid')
    const colours = () => [getComputedStyle(block).backgroundColor, getComputedStyle(block).color]

    document.documentElement.style.colorScheme = 'light'
    const light = colours()
    document.documentElement.style.colorScheme = 'dark'
    const dark = colours()
    expect(dark[0]).not.toBe(light[0])
    expect(dark[1]).not.toBe(light[1])
    expect(luminance(dark[0] ?? '')).toBeLessThan(luminance(light[0] ?? ''))
    expect(luminance(dark[1] ?? '')).toBeGreaterThan(luminance(dark[0] ?? ''))
  })
})

function luminance(colour: string): number {
  const [r = 0, g = 0, b = 0] = (colour.match(/\d+(\.\d+)?/g) ?? []).map(Number)
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

describe('catalog definition for agents', () => {
  it('lists every component with its schema and nesting rules', () => {
    const definition = catalogDefinition()
    expect(definition.catalogId).toBe('markgogo.dev:genui/v1')
    const components = definition.components as Record<string, Record<string, unknown>>
    expect(Object.keys(components)).toEqual([...CATALOG_COMPONENTS])
    expect(Object.keys(components)).not.toContain('MgoUnknown')

    const props = (name: string) => components[name]?.properties as Record<string, Record<string, unknown>>
    expect(props('ComparisonTable').rows?.type).toBe('array')
    expect(props('MetricGrid').metrics).toMatchObject({ type: 'array', minItems: 1 })
    expect(props('Checklist').items).toMatchObject({ type: 'array' })
    expect(props('Timeline').sources?.description).toContain('source')

    expect(components.Row?.allowedParents).toEqual(['Surface', 'Column', 'Tabs'])
    expect(components.Tabs?.allowedParents).toEqual(['Surface', 'Column'])
    expect(components.Card?.allowedChildren).toEqual([
      'Column',
      'Text',
      'KeyPoints',
      'MetricGrid',
      'ComparisonTable',
      'Timeline',
      'Checklist',
    ])
  })
})
