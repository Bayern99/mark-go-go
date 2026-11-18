import '../src/styles.css'
import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'
import { App } from '../src/App'
import {
  createBrowserHost,
  createBrowserWorkspace,
  DEFAULT_MODELS,
  type BrowserHost,
  type FakeGenerator,
} from '../src/host/browserHost'
import { GenerationError } from '../src/host/generationError'
import type {
  GenerateFailure,
  GenerateProgress,
  GenerateRequest,
  WorkspaceAdapter,
} from '../src/host/types'
import { extractSections } from '../src/markdown/sections'

const DOC_A = `# Alpha\n\nFirst body.\n`
const DOC_B = `# Beta\n\nSecond body.\n`

function viewFile(title: string, request?: string, model = 'gemini-3.8-flash'): string {
  return JSON.stringify({
    version: 2,
    generatedAt: '2026-11-07T10:00:00Z',
    model,
    ...(request ? { request } : {}),
    sources: { overview: { heading: title, occurrence: 1 } },
    a2ui: [
      {
        version: '1.0',
        createSurface: {
          surfaceId: 'view',
          catalogId: 'markgogo.dev:genui/v1',
          components: [
            { id: 'root', component: 'KeyPoints', title, items: ['A point.'], sources: ['overview'] },
          ],
        },
      },
    ],
  })
}

function scripted() {
  let progress: ((step: GenerateProgress) => void) | undefined
  let settle: { resolve(text: string): void; reject(error: Error): void } | undefined
  let aborted: AbortSignal | undefined
  let received: GenerateRequest | undefined
  const generator: FakeGenerator = (req, onProgress, signal) => {
    received = req
    progress = onProgress
    aborted = signal
    return new Promise<string>((resolve, reject) => {
      settle = { resolve, reject }
    })
  }
  return {
    generator,
    get request() {
      return received
    },
    get signal() {
      return aborted
    },
    step(step: GenerateProgress) {
      progress?.(step)
    },
    finish(text: string) {
      settle?.resolve(text)
    },
    fail(failure: GenerateFailure | Error) {
      settle?.reject(failure instanceof Error ? failure : new GenerationError(failure))
    },
  }
}

async function mountWorkspace(workspace: WorkspaceAdapter): Promise<{
  container: HTMLDivElement
  unmount(): Promise<void>
}> {
  const container = document.createElement('div')
  container.style.height = '100%'
  document.body.append(container)
  const root = createRoot(container)
  flushSync(() => {
    root.render(<App workspace={workspace} />)
  })
  await vi.waitFor(
    () => {
      if (!container.querySelector('[role="tablist"][aria-label="Documents"]')) {
        throw new Error('document tablist missing')
      }
      const panes = container.querySelectorAll('[data-document-id]')
      if (panes.length === 0) throw new Error('document panes missing')
      for (const pane of panes) {
        const mode = pane.querySelector<HTMLButtonElement>('[data-testid="view-mode-preview"]')
        if (!mode || mode.disabled) throw new Error('document pane not ready')
      }
    },
    { timeout: 8000 },
  )
  return {
    container,
    async unmount() {
      root.unmount()
      container.remove()
      await new Promise<void>((resolve) => setTimeout(resolve, 0))
    },
  }
}

async function openPaneGenUI(container: HTMLElement, host: BrowserHost): Promise<void> {
  const pane = await vi.waitFor(() => {
    const found = container.querySelector<HTMLElement>(`[data-document-id="${host.id}"]`)
    if (!found) throw new Error(`pane missing for ${host.id}`)
    const genuiMode = found.querySelector<HTMLButtonElement>('[data-testid="view-mode-genui"]')
    if (!genuiMode || genuiMode.disabled) throw new Error('GenUI mode not ready')
    return found
  })
  const genuiMode = pane.querySelector<HTMLButtonElement>('[data-testid="view-mode-genui"]')
  if (!genuiMode) throw new Error('GenUI mode missing')
  flushSync(() => genuiMode.click())
  await vi.waitFor(() => {
    const panelTab = pane.querySelector<HTMLButtonElement>('[data-testid="right-tab-genui.panel"]')
    if (!panelTab) throw new Error('GenUI panel tab missing')
    return panelTab
  }).then((panelTab) => flushSync(() => panelTab.click()))
  await vi.waitFor(() => {
    const view = pane.querySelector('[data-testid="genui-view"]')
    const panel = pane.querySelector('[data-testid="genui-panel"]')
    const state = view?.getAttribute('data-state')
    if (!panel || (state !== 'none' && state !== 'view')) throw new Error('GenUI not ready')
  })
}

function paneOf(container: HTMLElement, host: BrowserHost): HTMLElement {
  const pane = container.querySelector<HTMLElement>(`[data-document-id="${host.id}"]`)
  if (!pane) throw new Error(`pane missing for ${host.id}`)
  return pane
}

function generateButton(pane: HTMLElement): HTMLButtonElement {
  const button = pane.querySelector<HTMLButtonElement>('.genui-generate')
  if (!button) throw new Error('Generate button missing')
  return button
}

function modelTrigger(pane: HTMLElement): HTMLButtonElement {
  const trigger = pane.querySelector<HTMLButtonElement>('[data-testid="model-menu-trigger"]')
  if (!trigger) throw new Error('Model menu missing')
  return trigger
}

/** Picks a model in this document's footer through its menu, as a reader does. */
async function chooseModel(pane: HTMLElement, model: string): Promise<void> {
  modelTrigger(pane).dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0 }))
  const option = await vi.waitFor(() => {
    const match = [...document.querySelectorAll<HTMLElement>('[role="option"]')].find(
      (candidate) => candidate.dataset.model === model,
    )
    if (!match) throw new Error(`model ${model} is not offered`)
    return match
  })
  flushSync(() => option.click())
}

/** Types a command in the Locate agent composer and saves it with Enter. */
function saveAgentCommand(alert: HTMLElement, command: string): void {
  const input = alert.querySelector<HTMLInputElement>('input[aria-label="Agent command"]')
  if (!input) throw new Error('Agent command input missing')
  const setValue = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set
  if (!setValue) throw new Error('no input value setter')
  flushSync(() => {
    setValue.call(input, command)
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
  flushSync(() => {
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
  })
}

describe('multi-document generation isolation', () => {
  it('runs A and B together, captures each request, and never mixes output or progress', async () => {
    const agentA = scripted()
    const agentB = scripted()
    const a = createBrowserHost(DOC_A, 'a.md', { generator: agentA.generator })
    const b = createBrowserHost(DOC_B, 'b.md', { generator: agentB.generator })
    const workspace = createBrowserWorkspace([a, b])
    const app = await mountWorkspace(workspace)
    try {
      await openPaneGenUI(app.container, a)
      await openPaneGenUI(app.container, b)

      const paneA = paneOf(app.container, a)
      const paneB = paneOf(app.container, b)
      flushSync(() => generateButton(paneA).click())
      flushSync(() => generateButton(paneB).click())

      await vi.waitFor(() => {
        if (!paneA.querySelector('[data-testid="genui-progress"]')) throw new Error('A progress missing')
        if (!paneB.querySelector('[data-testid="genui-progress"]')) throw new Error('B progress missing')
      })

      expect(a.generations[0]?.text).toBe(DOC_A)
      expect(b.generations[0]?.text).toBe(DOC_B)
      expect(a.getState().generating).toBe(true)
      expect(b.getState().generating).toBe(true)
      expect(app.container.querySelector(`[data-testid="doc-tab-generating-${a.id}"]`)).not.toBeNull()
      expect(app.container.querySelector(`[data-testid="doc-tab-generating-${b.id}"]`)).not.toBeNull()

      agentA.step({ phase: 'reading', text: 'File source.md A' })
      agentB.step({ phase: 'writing', text: 'File out.json B' })
      await vi.waitFor(() => {
        expect(paneA.querySelector('[data-testid="genui-progress"]')?.textContent ?? '').toContain(
          'File source.md A',
        )
        expect(paneB.querySelector('[data-testid="genui-progress"]')?.textContent ?? '').toContain(
          'File out.json B',
        )
      })
      expect(paneA.querySelector('[data-testid="genui-progress"]')?.textContent ?? '').not.toContain(
        'File out.json B',
      )
      expect(paneB.querySelector('[data-testid="genui-progress"]')?.textContent ?? '').not.toContain(
        'File source.md A',
      )

      await workspace.activate(b.id)
      expect(a.getState().generating).toBe(true)
      expect(paneA.querySelector('[data-testid="genui-progress"]')).not.toBeNull()

      await expect(a.generate({ text: DOC_A, mode: 'full', format: '', sections: [] }, () => {}, new AbortController().signal)).rejects.toMatchObject({
        failure: { kind: 'busy' },
      })

      agentA.finish(viewFile('Made A', 'alpha-focus'))
      agentB.fail({
        kind: 'agent-message',
        message: 'B failed without a view.',
        hint: "Read the agent's message; retry once it is resolved.",
      })

      await vi.waitFor(() => {
        expect(paneA.querySelector('[data-testid="genui-view"]')?.textContent ?? '').toContain('Made A')
        expect(paneB.querySelector('[role="alert"]')?.textContent ?? '').toContain('B failed without a view.')
      })
      expect(paneA.querySelector('[role="alert"]')).toBeNull()
      expect(paneB.querySelector('[data-testid="genui-view"]')?.textContent ?? '').not.toContain('Made A')
      expect(b.keptView()).toBeNull()
    } finally {
      await app.unmount()
    }
  })

  it('captures model and request at start; later global settings and edits do not change the in-flight job',
    async () => {
      const agentA = scripted()
      const a = createBrowserHost(DOC_A, 'a.md', { generator: agentA.generator })
      const b = createBrowserHost(DOC_B, 'b.md', { generator: () => new Promise<string>(() => {}) })
      // One window, one settings store: both tabs read the same values.
      createBrowserWorkspace([a, b])
      const req: GenerateRequest = { text: DOC_A, mode: 'full', request: 'keep-me', format: '{}', sections: [] }
      const run = a.generate(req, () => {}, new AbortController().signal)
      await vi.waitFor(() => expect(a.capturedSettings[0]?.model).toBe('gemini-3.8-flash'))
      const changed = await b.setSettings({ model: 'claude-opus-5-5' })
      expect(changed).toMatchObject({ model: 'claude-opus-5-5', agentCommand: 'agy' })
      expect((await a.getSettings()).model).toBe('claude-opus-5-5')
      await a.update('# Alpha\n\nEdited while running.\n')
      agentA.finish(viewFile('Kept start', 'keep-me', 'gemini-3.8-flash'))
      await run
      expect(a.capturedSettings[0]).toEqual({
        model: 'gemini-3.8-flash',
        agentCommand: 'agy',
        models: [...DEFAULT_MODELS],
      })
      expect(a.generations[0]?.text).toBe(DOC_A)
      expect(a.generations[0]?.request).toBe('keep-me')
      expect((await a.load()).text).toContain('Edited while running.')
    },
  )

  it('disables ModelMenu and path ops only on the generating document', async () => {
    const agentA = scripted()
    const a = createBrowserHost(DOC_A, 'a.md', { generator: agentA.generator })
    const b = createBrowserHost(DOC_B, 'b.md', { generator: scripted().generator })
    const workspace = createBrowserWorkspace([a, b])
    const app = await mountWorkspace(workspace)
    try {
      await openPaneGenUI(app.container, a)
      await openPaneGenUI(app.container, b)
      const paneA = paneOf(app.container, a)
      const paneB = paneOf(app.container, b)
      flushSync(() => generateButton(paneA).click())
      await vi.waitFor(() => {
        if (!paneA.querySelector('[data-testid="genui-progress"]')) throw new Error('A not running')
      })
      const triggerA = paneA.querySelector<HTMLButtonElement>('[data-testid="model-menu-trigger"]')
      const triggerB = paneB.querySelector<HTMLButtonElement>('[data-testid="model-menu-trigger"]')
      expect(triggerA?.disabled).toBe(true)
      expect(triggerB?.disabled).toBe(false)
      await expect(a.saveAs()).rejects.toThrow(/wait for it to finish, or cancel it/)
      await expect(a.rename()).rejects.toThrow(/wait for it to finish, or cancel it/)
      await expect(a.moveTo()).rejects.toThrow(/wait for it to finish, or cancel it/)
      await expect(a.undoView()).rejects.toThrow(/wait for it to finish, or cancel it/)
      await expect(b.saveAs()).resolves.toBeUndefined()
      await expect(b.rename()).resolves.toBeUndefined()
      await a.save()
      agentA.finish(viewFile('Done A'))
      await vi.waitFor(() => expect(a.getState().generating).toBe(false))
    } finally {
      await app.unmount()
    }
  })

  it('refuses to write after an external path change and after a real close, and cancelled close keeps the job',
    async () => {
      const agentA = scripted()
      const a = createBrowserHost(DOC_A, 'a.md', {
        generator: agentA.generator,
        view: viewFile('Old A'),
      })
      const b = createBrowserHost(DOC_B, 'b.md', { generator: scripted().generator })
      const workspace = createBrowserWorkspace([a, b], {
        confirmClose: async () => ({ closed: false, canceled: true }),
      })
      const req: GenerateRequest = { text: DOC_A, mode: 'full', format: '{}', sections: [] }
      const first = a.generate(req, () => {}, new AbortController().signal)
      await vi.waitFor(() => expect(a.getState().generating).toBe(true))

      const canceled = await workspace.closeDocument(a.id)
      expect(canceled.canceled).toBe(true)
      expect(workspace.document(a.id)).toBe(a)
      expect(a.getState().generating).toBe(true)

      a.setPath('/elsewhere/a.md')
      agentA.finish(viewFile('Stale path'))
      await expect(first).rejects.toMatchObject({
        failure: { kind: 'agent-error', message: expect.stringContaining('path changed') },
      })
      expect(a.keptView()).toBeNull()

      const agentClose = scripted()
      const closing = createBrowserHost(DOC_A, 'close.md', {
        generator: agentClose.generator,
        view: viewFile('Keep me'),
      })
      const closingWs = createBrowserWorkspace([closing, b])
      const run = closing.generate(req, () => {}, new AbortController().signal)
      await vi.waitFor(() => expect(closing.getState().generating).toBe(true))
      const closed = await closingWs.closeDocument(closing.id)
      expect(closed.closed).toBe(true)
      agentClose.finish(viewFile('Too late'))
      await expect(run).rejects.toMatchObject({ failure: { kind: 'cancelled' } })
      expect(closing.keptView()).toBeNull()
    },
  )
})

describe('one settings store across documents', () => {
  it('shows in an idle mounted panel a model another document changed, without remounting it', async () => {
    const a = createBrowserHost(DOC_A, 'a.md', { generator: scripted().generator })
    const b = createBrowserHost(DOC_B, 'b.md', { generator: scripted().generator })
    const workspace = createBrowserWorkspace([a, b])
    const app = await mountWorkspace(workspace)
    try {
      await openPaneGenUI(app.container, a)
      await openPaneGenUI(app.container, b)
      const paneA = paneOf(app.container, a)
      const triggerBefore = modelTrigger(paneA)
      expect(triggerBefore.textContent).toContain('gemini-3.8-flash')

      // B picks a model while A sits idle in the background.
      await chooseModel(paneOf(app.container, b), 'claude-opus-5-5')

      await vi.waitFor(() => {
        if (!modelTrigger(paneA).textContent?.includes('claude-opus-5-5')) {
          throw new Error('the idle panel still shows the old model')
        }
      })
      // The same panel and footer control, never remounted.
      expect(paneOf(app.container, a)).toBe(paneA)
      expect(modelTrigger(paneA)).toBe(triggerBefore)
      expect(await a.getSettings()).toMatchObject({ model: 'claude-opus-5-5', agentCommand: 'agy' })
    } finally {
      await app.unmount()
    }
  })

  it('keeps a command saved in one document when a model is picked in another', async () => {
    const agentB = scripted()
    const a = createBrowserHost(DOC_A, 'a.md', { generator: scripted().generator })
    const b = createBrowserHost(DOC_B, 'b.md', { generator: agentB.generator })
    const workspace = createBrowserWorkspace([a, b])
    const app = await mountWorkspace(workspace)
    try {
      await openPaneGenUI(app.container, a)
      await openPaneGenUI(app.container, b)
      const paneA = paneOf(app.container, a)
      const paneB = paneOf(app.container, b)

      // B fails on a missing agent and saves the command that does exist.
      flushSync(() => generateButton(paneB).click())
      agentB.fail({
        kind: 'agent-not-found',
        message: 'Could not find the agent command "agy".',
        hint: 'Install the agent, or enter the command or path that runs it.',
      })
      const alert = await vi.waitFor(() => {
        const found = paneB.querySelector<HTMLElement>('[role="alert"]')
        if (!found?.textContent?.includes('Could not find the agent command')) {
          throw new Error('the failure is not shown')
        }
        return found
      })
      const locate = [...alert.querySelectorAll<HTMLButtonElement>('button')].find(
        (button) => button.textContent === 'Locate agent…',
      )
      if (!locate) throw new Error('Locate agent… missing')
      flushSync(() => locate.click())
      saveAgentCommand(alert, '/opt/bin/agy')
      await vi.waitFor(async () => {
        if ((await b.getSettings()).agentCommand !== '/opt/bin/agy') {
          throw new Error('the command was not saved')
        }
      })

      // A's stale panel must not write B's command back to the default.
      await chooseModel(paneA, 'claude-opus-5-5')
      await vi.waitFor(async () => {
        if ((await a.getSettings()).model !== 'claude-opus-5-5') {
          throw new Error('the model was not saved')
        }
      })
      expect(await b.getSettings()).toEqual({
        model: 'claude-opus-5-5',
        agentCommand: '/opt/bin/agy',
        models: [...DEFAULT_MODELS],
      })
    } finally {
      await app.unmount()
    }
  })

  it('keeps both fields when two documents write different settings at once', async () => {
    const a = createBrowserHost(DOC_A, 'a.md')
    const b = createBrowserHost(DOC_B, 'b.md')
    createBrowserWorkspace([a, b])
    await Promise.all([
      a.setSettings({ model: 'claude-opus-5-5' }),
      b.setSettings({ agentCommand: '/opt/bin/agy' }),
    ])
    const want = {
      model: 'claude-opus-5-5',
      agentCommand: '/opt/bin/agy',
      models: [...DEFAULT_MODELS],
    }
    expect(await a.getSettings()).toEqual(want)
    expect(await b.getSettings()).toEqual(want)
  })
})

const ISOLATED_A = `# Overview\n\nA first body.\n\n# Metrics\n\nA second body.\n`
const ISOLATED_B = `# Overview\n\nB first body.\n\n# Metrics\n\nB second body.\n`

/** A view of `markdown` whose component ids match the other document's view. */
function isolatedView(markdown: string): string {
  const sections = extractSections(markdown)
  const sources: Record<string, { heading: string; occurrence: number; hash: string }> = {}
  for (const heading of ['Overview', 'Metrics']) {
    const section = sections.find(
      (candidate) => candidate.heading.text === heading && candidate.occurrence === 1,
    )
    if (!section) throw new Error(`no section ${heading}`)
    sources[heading.toLowerCase()] = { heading, occurrence: 1, hash: section.hash }
  }
  return JSON.stringify({
    version: 2,
    sources,
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
              title: 'Overview',
              items: ['One point.'],
              sources: ['overview'],
            },
            {
              id: 'metrics',
              component: 'KeyPoints',
              title: 'Metrics',
              items: ['One point.'],
              sources: ['metrics'],
            },
          ],
        },
      },
    ],
  })
}

describe('GenUI scroll/highlight isolation', () => {
  it("scrolls and highlights the clicked document's card when another mounted view has the same component ids", async () => {
    const a = createBrowserHost(ISOLATED_A, 'a.md', { view: isolatedView(ISOLATED_A) })
    const b = createBrowserHost(ISOLATED_B, 'b.md', { view: isolatedView(ISOLATED_B) })
    const workspace = createBrowserWorkspace([a, b])
    const app = await mountWorkspace(workspace)
    try {
      await openPaneGenUI(app.container, a)
      await openPaneGenUI(app.container, b)
      await workspace.activate(b.id)

      const paneA = paneOf(app.container, a)
      const paneB = paneOf(app.container, b)
      await vi.waitFor(() => expect(paneA.hidden).toBe(true))
      expect(paneA.querySelector('[data-testid="genui-view"]')).not.toBeNull()

      const editedB = ISOLATED_B.replace('B second body.', 'B second body, edited.')
      b.reload(editedB)
      const item = await vi.waitFor(
        () => {
          const found = paneB.querySelector<HTMLButtonElement>('[data-testid="changed-item-metrics"]')
          if (!found) throw new Error('changed item missing')
          return found
        },
        { timeout: 3000 },
      )

      const cardA = paneA.querySelector<HTMLElement>(
        '[data-testid="genui-view"] [data-component-id="metrics"]',
      )
      const cardB = paneB.querySelector<HTMLElement>(
        '[data-testid="genui-view"] [data-component-id="metrics"]',
      )
      if (!cardA || !cardB) throw new Error('metrics cards missing')
      const scrollA = vi.spyOn(cardA, 'scrollIntoView')
      const scrollB = vi.spyOn(cardB, 'scrollIntoView')

      flushSync(() => item.click())
      await vi.waitFor(() => {
        expect(scrollB).toHaveBeenCalled()
        expect(cardB.getAttribute('data-highlighted')).toBe('true')
      })
      expect(scrollA).not.toHaveBeenCalled()
      expect(cardA.getAttribute('data-highlighted')).toBeNull()

      // Jumping to a card is navigation only: both documents keep their bytes.
      expect((await a.load()).text).toBe(ISOLATED_A)
      expect((await b.load()).text).toBe(editedB)
    } finally {
      await app.unmount()
    }
  })
})
