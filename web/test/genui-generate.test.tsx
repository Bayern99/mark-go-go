import '../src/styles.css'
import { page, userEvent } from 'vitest/browser'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createBrowserHost, type BrowserHost, type FakeGenerator } from '../src/host/browserHost'
import { GenerationError } from '../src/host/generationError'
import type { GenerateFailure, GenerateProgress } from '../src/host/types'
import { extractSections } from '../src/markdown/sections'
import { CATALOG_COMPONENTS } from '../src/plugins/genui/a2ui'
import { mountApp, type MountedApp } from './helpers/app'

const DOCUMENT = `# Overview

First body.

# Metrics

Second body.
`

/** A view as the host writes it, with the request it was made from. */
function viewFile(title: string, request?: string): string {
  return JSON.stringify({
    version: 2,
    generatedAt: '2026-11-07T10:00:00Z',
    model: 'gemini-3.8-flash',
    ...(request ? { request } : {}),
    sources: { overview: { heading: 'Overview', occurrence: 1 } },
    a2ui: [
      {
        version: '1.0',
        createSurface: {
          surfaceId: 'view',
          catalogId: 'markgogo.dev:genui/v1',
          components: [{ id: 'root', component: 'KeyPoints', title, items: ['A point.'], sources: ['overview'] }],
        },
      },
    ],
  })
}

/** A generator the test steps through: each `step()` sends one progress, `finish` ends the run. */
function scripted() {
  let progress: ((step: GenerateProgress) => void) | undefined
  let settle: { resolve(text: string): void; reject(error: Error): void } | undefined
  let aborted: AbortSignal | undefined
  const generator: FakeGenerator = (_req, onProgress, signal) => {
    progress = onProgress
    aborted = signal
    return new Promise<string>((resolve, reject) => {
      settle = { resolve, reject }
    })
  }
  return {
    generator,
    step(step: GenerateProgress) {
      progress?.(step)
    },
    finish(text: string) {
      settle?.resolve(text)
    },
    fail(failure: GenerateFailure | Error) {
      settle?.reject(failure instanceof Error ? failure : new GenerationError(failure))
    },
    get signal() {
      return aborted
    },
  }
}

/** Waits until the element's text contains `text`. */
async function containsText(locator: { element(): Element }, text: string): Promise<void> {
  await vi.waitFor(() => expect(locator.element().textContent ?? '').toContain(text))
}

let app: MountedApp | undefined

afterEach(async () => {
  await app?.unmount()
  app = undefined
})

async function openGenUI(host: BrowserHost): Promise<MountedApp> {
  app = await mountApp(DOCUMENT, host)
  await page.getByTestId('view-mode-genui').click()
  await vi.waitFor(() => {
    const state = app?.container.querySelector('[data-testid="genui-view"]')?.getAttribute('data-state')
    expect(state === 'none' || state === 'view').toBe(true)
  })
  return app
}

const genui = () => page.getByTestId('genui-view')
const generate = () => page.getByRole('button', { name: /^Generate/ })
const requestBox = () => page.getByRole('textbox', { name: /What should the view focus on/ })

describe('GenUI generation', () => {
  it('sends the editor text, its sections, the format and the request, then shows the new view', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', { generator: agent.generator })
    const mounted = await openGenUI(host)

    await requestBox().fill('Focus on the metrics')
    await generate().click()

    await expect.element(page.getByTestId('genui-progress')).toBeVisible()
    expect(mounted.container.querySelector('.genui-skeleton')).not.toBeNull()
    await containsText(genui().getByRole('status'), 'Starting the agent')
    agent.step({ phase: 'reading', text: 'File source.md' })
    await containsText(genui().getByRole('status'), 'File source.md')
    agent.step({ phase: 'writing', text: 'File out.json' })
    await containsText(genui().getByRole('status'), 'File out.json')

    // One run at a time: the only action while it runs is Cancel.
    expect(generate().query()).toBeNull()
    await expect.element(page.getByRole('button', { name: 'Cancel' })).toBeVisible()
    await expect.element(requestBox()).toBeDisabled()

    const [req] = host.generations
    expect(req?.text).toBe(DOCUMENT)
    expect(req?.mode).toBe('full')
    expect(req?.request).toBe('Focus on the metrics')
    expect(req?.sections).toEqual(
      extractSections(DOCUMENT).map((s) => ({
        heading: s.heading.text,
        occurrence: s.occurrence,
        start: s.start,
        end: s.end,
      })),
    )
    // The agent learns the format and every catalog component's definition.
    expect(req?.format).toContain('# Mark Go Go view file format')
    for (const name of CATALOG_COMPONENTS) expect(req?.format).toContain(`"${name}"`)

    agent.finish(viewFile('Made by the agent', 'Focus on the metrics'))
    await expect.element(page.getByText('Made by the agent', { exact: false })).toBeVisible()
    expect(page.getByTestId('genui-progress').query()).toBeNull()
    await containsText(generate(), 'Generate again')
    // The view's own request is offered again.
    await expect.element(requestBox()).toHaveValue('Focus on the metrics')
    expect(host.getState().edited).toBe(false)
  })

  it('keeps editing possible while the agent works, and sends the text of the moment', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', { generator: agent.generator })
    const mounted = await openGenUI(host)
    await generate().click()
    await expect.element(page.getByTestId('genui-progress')).toBeVisible()

    await page.getByTestId('view-mode-richtext').click()
    const paragraph = Array.from(mounted.container.querySelectorAll<HTMLElement>('.ProseMirror p')).find(
      (p) => p.textContent === 'Second body.',
    )
    if (!paragraph) throw new Error('paragraph missing')
    await userEvent.click(paragraph)
    await userEvent.keyboard('{End} More.')
    await vi.waitFor(() => expect(host.getState().edited).toBe(true))
    expect(host.generations[0]?.text).toBe(DOCUMENT)

    // Back in GenUI, the run is still going.
    await page.getByTestId('view-mode-genui').click()
    await expect.element(page.getByTestId('genui-progress')).toBeVisible()
    agent.finish(viewFile('After the edit'))
    await expect.element(page.getByText('After the edit', { exact: false })).toBeVisible()
  })

  it('keeps the old view, shows what the agent said and retries the same request', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', {
      view: viewFile('The old view'),
      generator: agent.generator,
    })
    await openGenUI(host)
    await expect.element(page.getByText('The old view', { exact: false })).toBeVisible()

    await requestBox().fill('Compare the metrics')
    await generate().click()
    const quota = 'Individual quota reached.\nResets in 3h 12m.'
    agent.fail({
      kind: 'agent-message',
      message: 'The agent finished without writing a view.',
      hint: "Read the agent's message; retry once it is resolved.",
      detail: quota,
    })

    const alert = genui().getByRole('alert')
    await containsText(alert, 'The agent finished without writing a view.')
    await containsText(alert, "Read the agent's message; retry once it is resolved.")
    // The agent's words verbatim, line breaks included.
    expect(alert.element().querySelector('pre')?.textContent).toBe(quota)
    await expect.element(page.getByText('The old view', { exact: false })).toBeVisible()
    expect(page.getByRole('button', { name: 'Cancel' }).query()).toBeNull()
    expect(page.getByRole('button', { name: 'Open agent settings' }).query()).toBeNull()

    await page.getByRole('button', { name: 'Retry' }).click()
    await expect.element(page.getByTestId('genui-progress')).toBeVisible()
    expect(genui().getByRole('alert').query()).toBeNull()
    expect(host.generations.map((req) => req.request)).toEqual(['Compare the metrics', 'Compare the metrics'])
    agent.finish(viewFile('Made on retry', 'Compare the metrics'))
    await expect.element(page.getByText('Made on retry', { exact: false })).toBeVisible()
    expect(genui().getByRole('alert').query()).toBeNull()
  })

  it('lists the models the agent offers and opens the agent settings', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', { generator: agent.generator })
    await openGenUI(host)
    await generate().click()
    agent.fail({
      kind: 'model-invalid',
      message: 'The agent does not know the model "gemini-9".',
      hint: 'Choose one of the models it offers in Agent settings.',
      models: ['gemini-3.8-flash', 'claude-opus-5-5'],
    })

    const alert = genui().getByRole('alert')
    await containsText(alert, 'The agent does not know the model "gemini-9".')
    expect(Array.from(alert.element().querySelectorAll('li'), (li) => li.textContent)).toEqual([
      'gemini-3.8-flash',
      'claude-opus-5-5',
    ])
    await page.getByRole('button', { name: 'Open agent settings' }).click()
    await expect.element(page.getByRole('textbox', { name: 'Model' })).toHaveValue('gemini-3.8-flash')
  })

  it('shows an error that is not a failure as an agent error, and dismisses it', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', { generator: agent.generator })
    await openGenUI(host)
    await generate().click()
    agent.fail(new Error('The agent exited before answering initialize.'))

    const alert = genui().getByRole('alert')
    await containsText(alert, 'The agent exited before answering initialize.')
    await containsText(alert, 'run the agent command in Terminal')
    await page.getByRole('button', { name: 'Dismiss' }).click()
    expect(genui().getByRole('alert').query()).toBeNull()
    await expect.element(generate()).toBeEnabled()
  })

  it('cancels a run and leaves the view as it was', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', {
      view: viewFile('The old view'),
      generator: agent.generator,
    })
    await openGenUI(host)
    await generate().click()
    await expect.element(page.getByTestId('genui-progress')).toBeVisible()

    await page.getByRole('button', { name: 'Cancel' }).click()
    expect(agent.signal?.aborted).toBe(true)
    await expect.element(page.getByText('The old view', { exact: false })).toBeVisible()
    expect(genui().getByRole('alert').query()).toBeNull()
    // A result arriving after the cancel changes nothing.
    agent.finish(viewFile('Too late'))
    await new Promise((resolve) => setTimeout(resolve, 50))
    expect(page.getByText('Too late', { exact: false }).query()).toBeNull()
  })

  it('does not generate for a document that was never saved', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'Untitled', { generator: agent.generator, untitled: true })
    await openGenUI(host)
    await expect.element(generate()).toBeDisabled()
    await expect.element(page.getByText('Save the document first', { exact: false })).toBeVisible()
    expect(host.generations).toHaveLength(0)
  })

  it('keeps the agent settings', async () => {
    const host = createBrowserHost(DOCUMENT, 'report.md', { generator: scripted().generator })
    await openGenUI(host)
    await page.getByRole('button', { name: 'Agent settings' }).click()
    const model = page.getByRole('textbox', { name: 'Model' })
    await expect.element(model).toHaveValue('gemini-3.8-flash')
    await expect.element(page.getByRole('textbox', { name: 'Agent command' })).toHaveValue('agy')

    await model.fill('claude-opus-5-5')
    await page.getByRole('button', { name: 'Save' }).click()
    expect(await host.getSettings()).toEqual({ model: 'claude-opus-5-5', agentCommand: 'agy' })

    await page.getByRole('button', { name: 'Agent settings' }).click()
    await expect.element(page.getByRole('textbox', { name: 'Model' })).toHaveValue('claude-opus-5-5')
  })
})
