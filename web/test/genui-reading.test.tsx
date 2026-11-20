import '../src/styles.css'
import { page, userEvent } from 'vitest/browser'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createBrowserHost, type BrowserHost, type FakeGenerator } from '../src/host/browserHost'
import { GenerationError } from '../src/host/generationError'
import type { GenerateFailure, GenerateProgress } from '../src/host/types'
import { extractSections } from '../src/markdown/sections'
import { SOURCE_DEBOUNCE_MS } from '../src/plugins/genui/sources'
import { mountApp, type MountedApp } from './helpers/app'

const DOCUMENT = `# Overview

First body.

# Metrics

Second body.
`

const POINTS = ['Keep the bytes.', 'Fold long lists.', 'Jump from Outline.', 'Stay read-only.', 'Keep the source footer.']

type Component = Record<string, unknown> & { id: string; component: string }

function viewOf(components: Component[], markdown = DOCUMENT): string {
  const sections = extractSections(markdown)
  const sources = Object.fromEntries(
    sections.map((section) => [
      section.heading.text.toLowerCase(),
      { heading: section.heading.text, occurrence: section.occurrence, hash: section.hash },
    ]),
  )
  return JSON.stringify({
    version: 2,
    sources,
    a2ui: [
      { version: '1.0', createSurface: { surfaceId: 'view', catalogId: 'markgogo.dev:genui/v1', components } },
    ],
  })
}

function keyPoints(count: number): Component[] {
  return [
    {
      id: 'root',
      component: 'KeyPoints',
      title: 'In short',
      items: POINTS.slice(0, count),
      sources: ['overview'],
    },
  ]
}

function scripted() {
  let progress: ((step: GenerateProgress) => void) | undefined
  let resolveText: ((text: string) => void) | undefined
  let rejectRun: ((error: Error) => void) | undefined
  let aborted: AbortSignal | undefined
  const generator: FakeGenerator = (_req, onProgress, signal) => {
    progress = onProgress
    aborted = signal
    return new Promise<string>((resolve, reject) => {
      resolveText = resolve
      rejectRun = reject
    })
  }
  return {
    generator,
    step(step: GenerateProgress) {
      progress?.(step)
    },
    finish(text: string) {
      resolveText?.(text)
    },
    fail(failure: GenerateFailure | Error) {
      rejectRun?.(failure instanceof Error ? failure : new GenerationError(failure))
    },
    get signal() {
      return aborted
    },
  }
}

let app: MountedApp | undefined

afterEach(async () => {
  await app?.unmount()
  app = undefined
})

async function showGenUI(host: BrowserHost): Promise<{ host: BrowserHost; container: HTMLDivElement; unmount(): Promise<void> }> {
  const mounted = await mountApp(DOCUMENT, host)
  app = mounted
  await page.getByTestId('view-mode-genui').click()
  await vi.waitFor(() => expect(page.getByTestId('genui-view').query()).not.toBeNull())
  return { ...mounted, host }
}

function genui() {
  return page.getByTestId('genui-view')
}

describe('KeyPoints folding',
  () => {
    it('leaves four or fewer points open, with the source footer still there',
      async () => {
        const host = createBrowserHost(DOCUMENT, 'report.md', { view: viewOf(keyPoints(4)) })
        const mounted = await showGenUI(host)
        await vi.waitFor(() => expect(genui()).toHaveAttribute('data-state', 'view'))
        await vi.waitFor(() => expect(mounted.container.querySelectorAll('.genui-keypoints li')).toHaveLength(4))
        expect(page.getByRole('button', { name: /^Show all/ }).query()).toBeNull()
        await expect.element(genui().getByText('From Overview')).toBeVisible()
        expect(mounted.host.getState().edited).toBe(false)
      })

    it('folds after four points, expands with Show all N, and keeps the source actions',
      async () => {
        const host = createBrowserHost(DOCUMENT, 'report.md', { view: viewOf(keyPoints(5)) })
        const mounted = await showGenUI(host)
        await vi.waitFor(() => expect(genui()).toHaveAttribute('data-state', 'view'))
        const update = vi.spyOn(mounted.host, 'update')
        await vi.waitFor(() => expect(mounted.container.querySelectorAll('.genui-keypoints li')).toHaveLength(4))
        expect(genui().getByText('Keep the source footer.').query()).toBeNull()
        const more = page.getByRole('button', { name: 'Show all 5' })
        await expect.element(more).toHaveAttribute('aria-expanded', 'false')
        await expect.element(genui().getByText('From Overview')).toBeVisible()
        await expect.element(page.getByRole('button', { name: 'Edit in source' })).toBeEnabled()

        more.element().focus()
        await userEvent.keyboard('{Enter}')
        await vi.waitFor(() => expect(mounted.container.querySelectorAll('.genui-keypoints li')).toHaveLength(5))
        await expect.element(genui().getByText('Keep the source footer.')).toBeVisible()
        await expect.element(page.getByRole('button', { name: 'Show fewer' })).toHaveAttribute('aria-expanded', 'true')
        await expect.element(genui().getByText('From Overview')).toBeVisible()

        await page.getByRole('button', { name: 'Show fewer' }).click()
        await vi.waitFor(() => expect(mounted.container.querySelectorAll('.genui-keypoints li')).toHaveLength(4))
        expect(update).not.toHaveBeenCalled()
        expect(mounted.host.getState().edited).toBe(false)
      })
  },
)

describe('GenUI reading state',
  () => {
    it('keeps KeyPoints fold, table sort and Tabs after leaving the view, without writing the document',
      async () => {
        const host = createBrowserHost(DOCUMENT, 'report.md', {
          view: viewOf([
            {
              id: 'root',
              component: 'Tabs',
              tabs: [
                { title: 'Summary', child: 'points' },
                { title: 'Details', child: 'table' },
              ],
            },
            {
              id: 'points',
              component: 'KeyPoints',
              title: 'In short',
              items: POINTS,
              sources: ['overview'],
            },
            {
              id: 'table',
              component: 'ComparisonTable',
              title: 'Options',
              columns: ['Option', 'Size'],
              rows: [
                ['MyGo', '12'],
                ['Electron', '180'],
                ['Tauri', '9'],
              ],
              sources: ['metrics'],
            },
          ]),
        })
        const mounted = await showGenUI(host)
        await vi.waitFor(() => expect(genui()).toHaveAttribute('data-state', 'view'))
        const update = vi.spyOn(mounted.host, 'update')

        await page.getByRole('button', { name: 'Show all 5' }).click()
        await vi.waitFor(() => expect(mounted.container.querySelectorAll('.genui-keypoints li')).toHaveLength(5))
        await genui().getByText('Details').click()
        await page.getByRole('button', { name: /^Size/ }).click()
        await vi.waitFor(() => {
          const first = mounted.container.querySelector('.genui-comparison tbody th')?.textContent
          expect(first).toBe('Tauri')
        })
        await expect.element(page.getByRole('columnheader', { name: /^Size/ })).toHaveAttribute('aria-sort', 'ascending')

        await page.getByTestId('view-mode-preview').click()
        await page.getByTestId('view-mode-genui').click()
        await vi.waitFor(() => expect(genui()).toHaveAttribute('data-state', 'view'))

        await expect.element(page.getByRole('columnheader', { name: /^Size/ })).toBeVisible()
        await genui().getByText('Summary').click()
        await expect.element(page.getByRole('button', { name: 'Show fewer' })).toBeVisible()
        expect(mounted.container.querySelectorAll('.genui-keypoints li')).toHaveLength(5)
        await genui().getByText('Details').click()
        await vi.waitFor(() => {
          expect(mounted.container.querySelector('.genui-comparison tbody th')?.textContent).toBe('Tauri')
        })
        await expect.element(page.getByRole('columnheader', { name: /^Size/ })).toHaveAttribute('aria-sort', 'ascending')
        expect(update).not.toHaveBeenCalled()
        expect(mounted.host.getState().edited).toBe(false)
      })
  },
)

describe('card-local generation feedback',
  () => {
    it('skeletons only the replaced card, keeps the other readable, and restores it on cancel',
      async () => {
        const agent = scripted()
        const sections = extractSections(DOCUMENT)
        const source = (index: number) => {
          const section = sections[index]
          if (!section) throw new Error('section missing')
          return { heading: section.heading.text, occurrence: section.occurrence, hash: section.hash }
        }
        const current = JSON.stringify({
          version: 2,
          sources: { overview: source(0), metrics: source(1) },
          a2ui: [
            {
              version: '1.0',
              createSurface: {
                surfaceId: 'view',
                catalogId: 'markgogo.dev:genui/v1',
                components: [
                  { id: 'root', component: 'Column', children: ['overview', 'metrics'] },
                  {
                    id: 'overview',
                    component: 'KeyPoints',
                    title: 'Old overview',
                    items: ['O.'],
                    sources: ['overview'],
                  },
                  {
                    id: 'metrics',
                    component: 'KeyPoints',
                    title: 'Old metrics',
                    items: ['M.'],
                    sources: ['metrics'],
                  },
                ],
              },
            },
          ],
        })
        const host = createBrowserHost(DOCUMENT, 'report.md', { view: current, generator: agent.generator })
        const mounted = await showGenUI(host)
        await vi.waitFor(() => expect(genui().getByRole('heading', { name: 'Old metrics' }).query()).not.toBeNull())

        await page.getByTestId('view-mode-preview').click()
        const paragraph = Array.from(mounted.container.querySelectorAll<HTMLElement>('.ProseMirror p')).find(
          (p) => p.textContent === 'Second body.',
        )
        if (!paragraph) throw new Error('paragraph missing')
        await userEvent.click(paragraph)
        await userEvent.keyboard('{End} More.')
        await vi.waitFor(() => expect(host.getState().edited).toBe(true))
        await page.getByTestId('view-mode-genui').click()
        await page.getByTestId('genui-settings-toggle').click()
        await vi.waitFor(
          () => expect(page.getByRole('button', { name: /^Regenerate changed/ }).query()).not.toBeNull(),
          { timeout: SOURCE_DEBOUNCE_MS + 2000 },
        )
        await page.getByRole('button', { name: /^Regenerate changed/ }).click()
        await expect.element(page.getByTestId('genui-progress')).toBeVisible()

        const metrics = mounted.container.querySelector('[data-component-id="metrics"]')
        const overview = mounted.container.querySelector('[data-component-id="overview"]')
        expect(metrics?.querySelector('[data-testid="genui-card-skeleton"]')).not.toBeNull()
        expect(metrics?.getAttribute('aria-busy')).toBe('true')
        expect(overview?.querySelector('[data-testid="genui-card-skeleton"]')).toBeNull()
        await expect.element(genui().getByText('Old overview', { exact: false })).toBeVisible()
        expect(overview?.querySelector<HTMLButtonElement>('.genui-source-edit')?.disabled).toBe(false)
        expect(metrics?.querySelector('.genui-source')).not.toBeNull()

        await page.getByRole('button', { name: 'Cancel' }).click()
        expect(agent.signal?.aborted).toBe(true)
        await vi.waitFor(() => {
          expect(mounted.container.querySelector('[data-testid="genui-card-skeleton"]')).toBeNull()
        })
        await expect.element(genui().getByRole('heading', { name: 'Old metrics' })).toBeVisible()
        await expect.element(genui().getByText('Old overview', { exact: false })).toBeVisible()
      })
  },
)

describe('first view snapshot feedback',
  () => {
    it('shows loading until the first snapshot, then the empty, view or error that snapshot names',
      async () => {
        const emptyHost = createBrowserHost(DOCUMENT, 'report.md', { holdFirstView: true })
        const empty = await showGenUI(emptyHost)
        await vi.waitFor(() => expect(genui()).toHaveAttribute('data-state', 'loading'))
        await expect.element(page.getByTestId('genui-loading')).toBeVisible()
        expect(page.getByTestId('genui-empty').query()).toBeNull()
        expect(empty.container.querySelector('.genui-surface')).toBeNull()

        emptyHost.releaseFirstView()
        await expect.element(genui().getByText('No view yet.')).toBeVisible()
        await expect.element(genui()).toHaveAttribute('data-state', 'none')
        expect(page.getByTestId('genui-loading').query()).toBeNull()
        await empty.unmount()
        app = undefined

        const viewHost = createBrowserHost(DOCUMENT, 'report.md', {
          view: viewOf(keyPoints(2)),
          holdFirstView: true,
        })
        const withView = await showGenUI(viewHost)
        await vi.waitFor(() => expect(genui()).toHaveAttribute('data-state', 'loading'))
        await expect.element(page.getByTestId('genui-loading')).toBeVisible()
        viewHost.releaseFirstView()
        await expect.element(genui().getByText('Keep the bytes.')).toBeVisible()
        await expect.element(genui()).toHaveAttribute('data-state', 'view')
        expect(page.getByTestId('genui-loading').query()).toBeNull()
        await withView.unmount()
        app = undefined

        const errorHost = createBrowserHost(DOCUMENT, 'report.md', { holdFirstView: true })
        const broken = await showGenUI(errorHost)
        await vi.waitFor(() => expect(genui()).toHaveAttribute('data-state', 'loading'))
        errorHost.writeView('{ not json')
        await vi.waitFor(() => expect(genui()).toHaveAttribute('data-state', 'error'))
        await expect.element(broken.container.querySelector('.genui-file-error') as HTMLElement).toBeVisible()
        await expect.element(genui().getByRole('button', { name: 'View generation details' })).toBeVisible()
        expect(page.getByTestId('genui-loading').query()).toBeNull()
      })
  },
)
