import { createRoot } from 'react-dom/client'
import { expect, it, vi } from 'vitest'
import { userEvent } from 'vitest/browser'
import { App } from '../src/App'
import { createBrowserHost } from '../src/host/browserHost'
import type { ResearchUpdate } from '../src/host/generated/mygo'
import { createFeynmanPlugin } from '../src/plugins/feynman'

const adapter = vi.hoisted(() => ({
  send: vi.fn(async () => ''), openCitation: vi.fn(async () => {}),
  onEvent: undefined as ((e: ResearchUpdate) => void) | undefined,
}))
vi.mock('../src/host/generated/mygo', async original => {
  const native = await original<typeof import('../src/host/generated/mygo')>()
  return { ...native,
    events: { ...native.events, researchUpdate: { on(fn: (e: ResearchUpdate) => void) {
      adapter.onEvent = fn
      return () => { adapter.onEvent = undefined }
    } } },
    ResearchHost: {
      connect: async (root: string) => adapter.onEvent?.({ root, kind: 'commands', commands: [
        { name: 'audit', description: 'Audit literature' },
        { name: 'lit', description: 'Review literature', input: {hint: 'topic'} },
      ] }),
      disconnect: async () => {}, artifacts: async () => [], prompt: adapter.send, openCitation: adapter.openCitation,
    },
  }
})

it('chooses an exact slash workflow, sends with Enter and opens citations from the streamed response', async () => {
  const host = createBrowserHost('# Source\n', 'paper.md')
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const plugin = createFeynmanPlugin('/papers', host.id, async () => {})
  const emit = (update: Omit<ResearchUpdate, 'root'>) => adapter.onEvent?.({ root: '/papers', ...update })
  try {
    root.render(<App host={host} plugins={[plugin]} />)
    await vi.waitFor(() => expect(container.querySelector('[data-testid="right-tab-feynman.panel"]')).not.toBeNull())
    container.querySelector<HTMLButtonElement>('[data-testid="right-tab-feynman.panel"]')!.click()
    await vi.waitFor(() => expect(container.querySelector('.fy-launchers button')).not.toBeNull())
    const field = container.querySelector<HTMLTextAreaElement>('[aria-label="Research prompt"]')!
    await userEvent.fill(field, '/lit')
    await vi.waitFor(() => expect(container.querySelector('[role="option"][aria-selected=true]')?.textContent).toContain('/lit'))
    await userEvent.keyboard('{Enter}')
    expect(field.value).toBe('/lit ')
    expect(adapter.send).not.toHaveBeenCalled()
    await userEvent.fill(field, '/lit score reading')
    await userEvent.keyboard('{Enter}')
    await vi.waitFor(() => expect(adapter.send).toHaveBeenCalledWith('/papers', '/lit score reading', '', ''))
    emit({ kind: 'user', text: '/lit score reading' })
    emit({ kind: 'tool', tool: {toolCallId:'read-1',title:'Read score',status:'in_progress'} })
    emit({ kind: 'tool', tool: {toolCallId:'read-1',status:'completed',output:'Bibliography verified'} })
    emit({ kind: 'assistant', text: '## Findings\n\nSee [the paper](https://doi.org/10.1234/example). Use $x^2$.' })
    await vi.waitFor(() => expect(container.querySelector('.fy-markdown h2')?.textContent).toBe('Findings'))
    expect(container.querySelector('.fy-markdown math')).not.toBeNull()
    expect(container.querySelectorAll('.fy-step')).toHaveLength(1)
    expect(container.querySelector('.fy-step')?.textContent).toContain('Bibliography verified')
    container.querySelector<HTMLAnchorElement>('.fy-markdown a')!.click()
    expect(adapter.openCitation).toHaveBeenCalledWith('/papers', 'https://doi.org/10.1234/example')
  } finally {
    root.unmount()
    container.remove()
  }
})
