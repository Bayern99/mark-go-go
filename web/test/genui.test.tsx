import '../src/styles.css'
import { page } from 'vitest/browser'
import { describe, expect, it, vi } from 'vitest'
import { createBrowserHost } from '../src/host/browserHost'
import type { HostAdapter } from '../src/host/types'
import { isRecord } from '../src/lib/guards'
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
  it('offers a disabled Generate button when the document has no view', async () => {
    const app = await mountApp(DOCUMENT)
    try {
      await showMode('genui')
      await expect.element(genui().getByText('No view yet.')).toBeVisible()
      await expect.element(genui().getByRole('button', { name: 'Generate' })).toBeDisabled()
      await expect.element(genui().getByText('coming soon', { exact: false })).toBeVisible()
    } finally {
      await app.unmount()
    }
  })

  it('renders the format document example in place of the editor', async () => {
    const app = await mountApp(DOCUMENT, reportHost(formatExample()))
    try {
      await showMode('genui')
      await vi.waitFor(() => expect(genui()).toHaveAttribute('data-state', 'view'))
      expect(app.container.querySelector<HTMLElement>('.editor-pane')?.hidden).toBe(true)
      expect(genui().getByRole('alert').query()).toBeNull()
      await expect.element(genui().getByText('gemini-3.8-flash', { exact: false })).toBeVisible()

      await expect.element(genui().getByText('In short')).toBeVisible()
      await expect.element(genui().getByText('Only the edited block is written again.')).toBeVisible()
      await expect.element(genui().getByText('Open a real file through the host.')).toBeVisible()
      const bold = surface(app.container).querySelector('strong')
      expect(bold?.textContent).toBe('untouched blocks')

      await genui().getByText('Details').click()
      await expect.element(genui().getByText('100% round trips')).toBeVisible()
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
      await expect.element(genui().getByText('Figures')).toBeVisible()
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
      replaceComponent('nextPoints', { id: 'nextPoints', component: 'Chart', series: [1, 2] }),
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
