import '../src/styles.css'
import { page, userEvent } from 'vitest/browser'
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  createBrowserHost,
  createBrowserSettings,
  type BrowserHost,
  type FakeGenerator,
} from '../src/host/browserHost'
import { GenerationError } from '../src/host/generationError'
import type { GenerateFailure, GenerateProgress, ViewSnapshot } from '../src/host/types'
import { extractSections } from '../src/markdown/sections'
import { SOURCE_DEBOUNCE_MS } from '../src/plugins/genui/sources'
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

/** A view whose envelope is valid but nests Tabs inside a Card: the artifact the SDK rejects. */
function nestedBadView(): string {
  return JSON.stringify({
    version: 2,
    generatedAt: '2026-11-10T02:47:20Z',
    model: 'gemini-3.8-flash',
    sources: { overview: { heading: 'Overview', occurrence: 1 } },
    a2ui: [
      {
        version: '1.0',
        createSurface: {
          surfaceId: 'view',
          catalogId: 'markgogo.dev:genui/v1',
          components: [
            { id: 'root', component: 'Column', children: ['card'] },
            { id: 'card', component: 'Card', child: 'tabs' },
            { id: 'tabs', component: 'Tabs', tabs: [{ title: 'A tab', child: 'kp' }] },
            { id: 'kp', component: 'KeyPoints', items: ['A point.'], sources: ['overview'] },
          ],
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
  await page.getByTestId('right-tab-genui.panel').click()
  await vi.waitFor(() => {
    const state = app?.container.querySelector('[data-testid="genui-view"]')?.getAttribute('data-state')
    const panelEl = app?.container.querySelector('[data-testid="genui-panel"]')
    expect((state === 'none' || state === 'view') && panelEl !== null).toBe(true)
  })
  return app
}

const genui = () => page.getByTestId('genui-view')
const panel = () => page.getByTestId('genui-panel')
const generate = () => panel().getByRole('button', { name: 'Generate', exact: true })
const regenerateChanged = () => panel().getByRole('button', { name: /^Regenerate changed/ })
const regenerateAll = () => panel().getByRole('button', { name: 'Regenerate all' })
const undo = () => panel().getByRole('button', { name: 'Undo' })
const requestBox = () => panel().getByRole('textbox', { name: 'Generation request' })

describe('GenUI generation', () => {
  it('sends the editor text, its sections, the format and the request, then shows the new view', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', { generator: agent.generator })
    const mounted = await openGenUI(host)

    // Center view has no generate button, textbox, or model menu.
    expect(genui().getByRole('button', { name: 'Generate', exact: true }).query()).toBeNull()
    expect(genui().getByRole('textbox').query()).toBeNull()

    await requestBox().fill('Focus on the metrics')
    await generate().click()

    await expect.element(page.getByTestId('genui-progress')).toBeVisible()
    expect(mounted.container.querySelector('.genui-skeleton')).not.toBeNull()
    await containsText(panel().getByRole('status'), 'Starting the agent')
    agent.step({ phase: 'reading', text: 'File source.md' })
    await containsText(panel().getByRole('status'), 'File source.md')
    agent.step({ phase: 'writing', text: 'File out.json' })
    await containsText(panel().getByRole('status'), 'File out.json')

    // One run at a time: the only action while it runs is Cancel.
    expect(generate().query()).toBeNull()
    await expect.element(panel().getByRole('button', { name: 'Cancel' })).toBeVisible()

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

    agent.finish(viewFile('Made by the agent', 'Focus on the metrics'))
    await expect.element(genui().getByText('Made by the agent', { exact: false })).toBeVisible()
    expect(page.getByTestId('genui-progress').query()).toBeNull()
    // Nothing changed since: Up to date with only Regenerate all shown.
    await expect.element(panel().getByText('Up to date')).toBeVisible()
    expect(regenerateChanged().query()).toBeNull()
    await expect.element(regenerateAll()).toBeEnabled()
    // The view's Focus request is displayed read-only until Edit is clicked.
    await expect.element(panel().getByTestId('focus-value')).toHaveTextContent('Focus on the metrics')
    expect(requestBox().query()).toBeNull()
    expect(host.getState().edited).toBe(false)
  })

  it('keeps editing possible while the agent works, and sends the text of the moment', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', { generator: agent.generator })
    const mounted = await openGenUI(host)
    await generate().click()
    await expect.element(page.getByTestId('genui-progress')).toBeVisible()

    await page.getByTestId('view-mode-preview').click()
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
    await expect.element(genui().getByText('After the edit', { exact: false })).toBeVisible()
  })

  it('keeps the old view, shows what the agent said in the right panel, and retries the same request', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', {
      view: viewFile('The old view'),
      generator: agent.generator,
    })
    await openGenUI(host)
    await expect.element(genui().getByText('The old view', { exact: false })).toBeVisible()

    await panel().getByTestId('focus-edit').click()
    await requestBox().fill('Compare the metrics')
    await panel().getByRole('button', { name: 'Save', exact: true }).click()
    await regenerateAll().click()
    const quota = 'Individual quota reached.\nResets in 3h 12m.'
    agent.fail({
      kind: 'agent-message',
      message: 'The agent finished without writing a view.',
      hint: "Read the agent's message; retry once it is resolved.",
      detail: quota,
    })

    // Error alert appears in the right panel, NOT in the center genui-view.
    expect(genui().getByRole('alert').query()).toBeNull()
    const alert = panel().getByRole('alert')
    await containsText(alert, 'The agent finished without writing a view.')
    await containsText(alert, "Read the agent's message; retry once it is resolved.")
    await expect.element(page.getByTestId('right-tab-dot-genui.panel')).toHaveAttribute('data-tone', 'danger')
    // The agent's words verbatim, line breaks included.
    expect(alert.element().querySelector('pre')?.textContent).toBe(quota)
    await expect.element(genui().getByText('The old view', { exact: false })).toBeVisible()
    expect(panel().getByRole('button', { name: 'Cancel' }).query()).toBeNull()
    expect(panel().getByRole('button', { name: 'Open agent settings' }).query()).toBeNull()

    await panel().getByRole('button', { name: 'Retry' }).click()
    await expect.element(page.getByTestId('genui-progress')).toBeVisible()
    expect(panel().getByRole('alert').query()).toBeNull()
    expect(host.generations.map((req) => req.request)).toEqual(['Compare the metrics', 'Compare the metrics'])
    agent.finish(viewFile('Made on retry', 'Compare the metrics'))
    await expect.element(genui().getByText('Made on retry', { exact: false })).toBeVisible()
    expect(panel().getByRole('alert').query()).toBeNull()
    expect(page.getByTestId('right-tab-dot-genui.panel').query()).toBeNull()
  })

  it('lists the models the agent offers and updates the model menu on model-invalid', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', { generator: agent.generator })
    await openGenUI(host)
    await generate().click()
    agent.fail({
      kind: 'model-invalid',
      message: 'The agent does not know the model "gemini-9".',
      hint: 'Choose one of the models it offers.',
      models: ['gemini-3.8-flash', 'claude-opus-5-5'],
    })

    const alert = panel().getByRole('alert')
    await containsText(alert, 'The agent does not know the model "gemini-9".')
    expect(alert.getByRole('button', { name: 'Locate agent…' }).query()).toBeNull()
    expect(Array.from(alert.element().querySelectorAll('li'), (li) => li.textContent)).toEqual([
      'gemini-3.8-flash',
      'claude-opus-5-5',
    ])
    const trigger = page.getByTestId('model-menu-trigger')
    await trigger.click()
    const menu = page.getByRole('listbox', { name: 'Model' })
    await expect.element(menu).toBeVisible()
    expect(
      Array.from(menu.element().querySelectorAll('[role="option"] .t'), (el) => el.textContent),
    ).toEqual(['Agent’s choice', 'gemini-3.8-flash', 'claude-opus-5-5'])
  })

  it('shows an error that is not a failure as an agent error, and dismisses it', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', { generator: agent.generator })
    await openGenUI(host)
    await generate().click()
    agent.fail(new Error('The agent exited before answering initialize.'))

    const alert = panel().getByRole('alert')
    await containsText(alert, 'The agent exited before answering initialize.')
    await containsText(alert, 'run the agent command in Terminal')
    expect(alert.getByRole('button', { name: 'Locate agent…' }).query()).toBeNull()
    await panel().getByRole('button', { name: 'Dismiss' }).click()
    expect(panel().getByRole('alert').query()).toBeNull()
    await expect.element(generate()).toBeEnabled()
  })

  it('shows Locate agent… only on agent-not-found failure and saves agentCommand', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', { generator: agent.generator })
    await openGenUI(host)
    await generate().click()
    agent.fail({
      kind: 'agent-not-found',
      message: 'Could not find the agent command "agy".',
      hint: 'Install the agent, or enter the command or path that runs it.',
    })

    const alert = panel().getByRole('alert')
    await containsText(alert, 'Could not find the agent command "agy".')
    const locateBtn = alert.getByRole('button', { name: 'Locate agent…' })
    await expect.element(locateBtn).toBeVisible()
    await locateBtn.click()

    const cmdInput = alert.getByRole('textbox', { name: 'Agent command' })
    await expect.element(cmdInput).toHaveValue('agy')
    await cmdInput.fill('/opt/bin/agy')
    cmdInput.element().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', metaKey: true, bubbles: true }))
    await expect.poll(() => alert.getByRole('textbox', { name: 'Agent command' }).query()).toBeNull()

    expect(await host.getSettings()).toMatchObject({
      agentCommand: '/opt/bin/agy',
    })
  })

  it('triggers generation with ⌘↩ and blurs on Escape in the no-view Generation request composer', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', { generator: agent.generator })
    await openGenUI(host)

    const input = requestBox()
    await input.fill('Summarize key points')
    input.element().focus()
    expect(document.activeElement).toBe(input.element())
    input.element().dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(document.activeElement).not.toBe(input.element())

    input.element().focus()
    input.element().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', metaKey: true, bubbles: true }))
    await expect.element(page.getByTestId('genui-progress')).toBeVisible()
    expect(host.generations[0]?.request).toBe('Summarize key points')
    agent.finish(viewFile('Done', 'Summarize key points'))
  })

  it('cancels a run and leaves the view as it was', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', {
      view: viewFile('The old view'),
      generator: agent.generator,
    })
    await openGenUI(host)
    await regenerateAll().click()
    await expect.element(page.getByTestId('genui-progress')).toBeVisible()

    await panel().getByRole('button', { name: 'Cancel' }).click()
    expect(agent.signal?.aborted).toBe(true)
    await expect.element(genui().getByText('The old view', { exact: false })).toBeVisible()
    expect(panel().getByRole('alert').query()).toBeNull()
    // A result arriving after the cancel changes nothing.
    agent.finish(viewFile('Too late'))
    await new Promise((resolve) => setTimeout(resolve, 50))
    expect(page.getByText('Too late', { exact: false }).query()).toBeNull()
  })

  it('does not generate for a document that was never saved', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', { generator: agent.generator, untitled: true })
    await openGenUI(host)
    await expect.element(generate()).toBeDisabled()
    await expect.element(panel().getByText('Save the document first', { exact: false })).toBeVisible()
    expect(host.generations).toHaveLength(0)
  })

  it('edits Focus in place with ⌘↩ to save and Escape to cancel', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', {
      view: viewFile('Existing view', 'Initial focus'),
      generator: agent.generator,
    })
    await openGenUI(host)

    // Focus is read-only by default when a view exists.
    await expect.element(panel().getByTestId('focus-value')).toHaveTextContent('Initial focus')
    expect(requestBox().query()).toBeNull()

    // Click Edit -> opens inline composer; Escape cancels and reverts.
    await panel().getByTestId('focus-edit').click()
    const input = requestBox()
    await expect.element(input).toHaveValue('Initial focus')
    await input.fill('Discarded focus')
    input.element().dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await expect.poll(() => requestBox().query()).toBeNull()
    await expect.element(panel().getByTestId('focus-value')).toHaveTextContent('Initial focus')

    // Click Edit -> type new focus -> ⌘↩ commits and closes composer.
    await panel().getByTestId('focus-edit').click()
    const input2 = requestBox()
    await input2.fill('Updated via shortcut')
    input2.element().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', metaKey: true, bubbles: true }))
    await expect.poll(() => requestBox().query()).toBeNull()
    await expect.element(panel().getByTestId('focus-value')).toHaveTextContent('Updated via shortcut')

    await regenerateAll().click()
    expect(host.generations[0]?.request).toBe('Updated via shortcut')
    agent.finish(viewFile('Updated view', 'Updated via shortcut'))
  })

  it('selects models via ModelMenu, supports Escape and outside click, disables while running, and omits text inputs', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', {
      generator: agent.generator,
      settings: createBrowserSettings({
        model: '',
        agentCommand: 'agy',
        models: ['gemini-3.8-flash', 'gemini-4-argon', 'claude-opus-5-5'],
      }),
    })
    await openGenUI(host)

    // Routine UI does not render text inputs for Model or Agent command.
    expect(page.getByRole('textbox', { name: 'Model' }).query()).toBeNull()
    expect(page.getByRole('textbox', { name: 'Agent command' }).query()).toBeNull()

    const trigger = page.getByTestId('model-menu-trigger')
    await expect.element(trigger).toHaveTextContent('Agent’s choice')

    // Click trigger to open upward popover (.mmenu.pop).
    await trigger.click()
    const menu = page.getByRole('listbox', { name: 'Model' })
    await expect.element(menu).toBeVisible()
    expect(menu.element().classList.contains('mmenu')).toBe(true)
    expect(menu.element().classList.contains('pop')).toBe(true)

    // Current selection ("Agent’s choice") is marked selected via aria-selected.
    const defaultOpt = menu.getByRole('option', { name: 'Agent’s choice' })
    await expect.element(defaultOpt).toHaveAttribute('aria-selected', 'true')

    // Pressing Escape when document.body has focus closes the menu.
    document.body.focus()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await expect.poll(() => page.getByRole('listbox', { name: 'Model' }).query()).toBeNull()

    // Reopen and click outside to close.
    await trigger.click()
    await expect.element(page.getByRole('listbox', { name: 'Model' })).toBeVisible()
    document.body.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true }))
    await expect.poll(() => page.getByRole('listbox', { name: 'Model' }).query()).toBeNull()

    // Reopen and choose a model: saves immediately and closes the menu.
    await trigger.click()
    await page.getByRole('option', { name: 'claude-opus-5-5' }).click()
    await expect.poll(() => page.getByRole('listbox', { name: 'Model' }).query()).toBeNull()
    await expect.element(trigger).toHaveTextContent('claude-opus-5-5')
    expect(await host.getSettings()).toEqual({
      model: 'claude-opus-5-5',
      agentCommand: 'agy',
      models: ['gemini-3.8-flash', 'gemini-4-argon', 'claude-opus-5-5'],
    })

    // While generation is running, ModelMenu is disabled.
    await generate().click()
    await expect.element(trigger).toBeDisabled()
    agent.finish(viewFile('Done'))
    await expect.element(trigger).toBeEnabled()
  })
})

/**
 * A view of both sections whose hashes match DOCUMENT, so that a section
 * edited afterwards shows as changed; `update` is appended as the host
 * appends an update's message.
 */
function trackedView(update?: { id: string; title: string }): string {
  const sections = extractSections(DOCUMENT)
  const source = (index: number) => {
    const section = sections[index]
    if (!section) throw new Error('section missing')
    return { heading: section.heading.text, occurrence: section.occurrence, hash: section.hash }
  }
  return JSON.stringify({
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
            { id: 'overview', component: 'KeyPoints', title: 'Old overview', items: ['O.'], sources: ['overview'] },
            { id: 'metrics', component: 'KeyPoints', title: 'Old metrics', items: ['M.'], sources: ['metrics'] },
          ],
        },
      },
      ...(update
        ? [
            {
              version: '1.0',
              updateComponents: {
                surfaceId: 'view',
                components: [
                  { id: update.id, component: 'KeyPoints', title: update.title, items: ['N.'], sources: [update.id] },
                ],
              },
            },
          ]
        : []),
    ],
  })
}

/** Types at the end of the Metrics section in Preview, then goes back to GenUI. */
async function editMetrics(mounted: MountedApp, host: BrowserHost): Promise<void> {
  await page.getByTestId('view-mode-preview').click()
  const paragraph = Array.from(mounted.container.querySelectorAll<HTMLElement>('.ProseMirror p')).find(
    (p) => p.textContent === 'Second body.',
  )
  if (!paragraph) throw new Error('paragraph missing')
  await userEvent.click(paragraph)
  await userEvent.keyboard('{End} More.')
  await vi.waitFor(() => expect(host.getState().edited).toBe(true))
  await page.getByTestId('view-mode-genui').click()
}

describe('GenUI regeneration', () => {
  it('replaces only the changed components, then goes back once', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', { view: trackedView(), generator: agent.generator })
    const mounted = await openGenUI(host)
    await expect.element(genui().getByText('Old metrics', { exact: false })).toBeVisible()
    await expect.element(panel().getByText('Up to date')).toBeVisible()
    expect(regenerateChanged().query()).toBeNull()
    expect(undo().query()).toBeNull()

    await editMetrics(mounted, host)
    await vi.waitFor(() => expect(regenerateChanged().element().textContent).toBe('Regenerate changed (1)'), {
      timeout: SOURCE_DEBOUNCE_MS + 2000,
    })
    await expect.element(panel().getByText('1 component out of date')).toBeVisible()
    await panel().getByTestId('focus-edit').click()
    await requestBox().fill('Shorter')
    await panel().getByRole('button', { name: 'Save', exact: true }).click()
    await regenerateChanged().click()

    const [req] = host.generations
    expect(req?.mode).toBe('update')
    expect(req?.replaceIds).toEqual(['metrics'])
    expect(req?.request).toBe('Shorter')
    expect(req?.text).toContain('Second body. More.')

    agent.finish(trackedView({ id: 'metrics', title: 'New metrics' }))
    // The update applies over the view: the other component stays.
    await expect.element(genui().getByText('New metrics', { exact: false })).toBeVisible()
    await expect.element(genui().getByText('Old overview', { exact: false })).toBeVisible()
    expect(genui().getByText('Old metrics', { exact: false }).query()).toBeNull()

    await undo().click()
    await expect.element(genui().getByText('Old metrics', { exact: false })).toBeVisible()
    expect(genui().getByText('New metrics', { exact: false }).query()).toBeNull()
    // Once: nothing is kept any more.
    await vi.waitFor(() => expect(undo().query()).toBeNull())
    expect(host.keptView()).toBeNull()
  })

  it('regenerates the whole view whether or not anything changed', async () => {
    const agent = scripted()
    const host = createBrowserHost(DOCUMENT, 'report.md', { view: trackedView(), generator: agent.generator })
    await openGenUI(host)
    await regenerateAll().click()
    expect(host.generations[0]?.mode).toBe('full')
    expect(host.generations[0]?.replaceIds).toBeUndefined()
    agent.finish(viewFile('All new'))
    await expect.element(genui().getByText('All new', { exact: false })).toBeVisible()
    expect(host.keptView()).toBe(trackedView())
    await expect.element(undo()).toBeEnabled()
  })

  it('keeps the old view and its undo when a run produces invalid nesting', async () => {
    const agent = scripted()
    const oldView = trackedView()
    const kept = viewFile('The earlier view')
    const host = createBrowserHost(DOCUMENT, 'report.md', { view: oldView, kept, generator: agent.generator })
    await openGenUI(host)
    await expect.element(genui().getByText('Old overview', { exact: false })).toBeVisible()

    await regenerateAll().click()
    agent.finish(nestedBadView())

    // The reader is told what is wrong, with the invalid-output hint.
    const alert = panel().getByRole('alert')
    await containsText(alert, 'UNALLOWED_')
    await containsText(alert, 'Tabs')
    await containsText(alert, 'Generate again')
    // Neither the view nor the view kept to go back to changed.
    expect(host.keptView()).toBe(kept)
    await expect.element(genui().getByText('Old overview', { exact: false })).toBeVisible()
    await expect.element(undo()).toBeEnabled()
    await undo().click()
    await expect.element(genui().getByText('The earlier view', { exact: false })).toBeVisible()
  })

  it('rejects an invalid generated view as invalid-output before it reaches the view or undo', async () => {
    const agent = scripted()
    const oldView = trackedView()
    const kept = viewFile('The earlier view')
    const host = createBrowserHost(DOCUMENT, 'report.md', { view: oldView, kept, generator: agent.generator })
    const snapshots: ViewSnapshot[] = []
    host.watchView((snap) => snapshots.push(snap))

    const run = host.generate(
      { text: DOCUMENT, mode: 'full', format: '[]', sections: [] },
      () => undefined,
      new AbortController().signal,
    )
    agent.finish(nestedBadView())
    const error: unknown = await run.then(
      () => undefined,
      (failure: unknown) => failure,
    )
    expect(error).toBeInstanceOf(GenerationError)
    expect((error as GenerationError).failure.kind).toBe('invalid-output')
    expect(host.keptView()).toBe(kept)
    await vi.waitFor(() => expect(snapshots.at(-1)?.text).toBe(oldView))
  })

  it('keeps the earlier view when a run fails', async () => {
    const agent = scripted()
    const kept = viewFile('The kept view')
    const host = createBrowserHost(DOCUMENT, 'report.md', { view: trackedView(), kept, generator: agent.generator })
    await openGenUI(host)
    await regenerateAll().click()
    // Going back waits while a run goes on.
    await expect.element(undo()).toBeDisabled()
    agent.fail({ kind: 'invalid-output', message: 'The view the agent wrote is not valid.', hint: 'Generate again.' })

    await containsText(panel().getByRole('alert'), 'The view the agent wrote is not valid.')
    expect(host.keptView()).toBe(kept)
    await undo().click()
    await expect.element(genui().getByText('The kept view', { exact: false })).toBeVisible()
  })
})
