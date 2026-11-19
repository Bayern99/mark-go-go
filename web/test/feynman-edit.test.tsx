import { createRoot } from 'react-dom/client'
import { expect, it, vi } from 'vitest'
import { userEvent } from 'vitest/browser'
import { App } from '../src/App'
import { createBrowserHost } from '../src/host/browserHost'
import { createFeynmanPlugin } from '../src/plugins/feynman'

const prompt = vi.hoisted(() => vi.fn<() => Promise<string>>())
vi.mock('../src/host/generated/mygo', async importOriginal => {
  const native = await importOriginal<typeof import('../src/host/generated/mygo')>()
  return {
    ...native,
    events: { ...native.events, researchUpdate: { on: () => () => {} } },
    ResearchHost: {
      connect: async () => {}, disconnect: async () => {},
      artifacts: async () => [], prompt,
    },
  }
})

it('reviews a proposal against the same document after it is renamed during research', async () => {
  const host = createBrowserHost('# Original\n', 'draft.md')
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  let finish!: (text: string) => void
  prompt.mockReturnValue(new Promise(resolve => { finish = resolve }))
  function button(label: string) {
    return Array.from(container.querySelectorAll('button')).find(b => b.textContent === label)
  }
  try {
    const plugin = createFeynmanPlugin('/demo', host.id, async () => { throw new Error('Unexpected file open') })
    root.render(<App host={host} plugins={[plugin]} />)
    await vi.waitFor(() => expect(button('Feynman')).toBeTruthy())
    button('Feynman')!.click()
    await vi.waitFor(() => expect(container.querySelector('textarea[aria-label="Research prompt"]')).toBeTruthy())
    await userEvent.fill(container.querySelector('textarea[aria-label="Research prompt"]')!, 'Revise this draft')
    container.querySelector<HTMLInputElement>('.plugin-panel input[type=checkbox]')!.click()
    button('Send')!.click()
    await vi.waitFor(() => expect(prompt).toHaveBeenCalled())
    host.setPath('/demo/renamed.md')
    finish('# Revised\n')
    await vi.waitFor(() => expect(button('Compare current text')).toBeTruthy())
    expect(button('Apply in Markdown')).toBeUndefined()
    button('Compare current text')!.click()
    await vi.waitFor(() => expect(button('Apply in Markdown')).toBeTruthy())
    button('Apply in Markdown')!.click()
    await vi.waitFor(() => expect(container.querySelector('.cm-content')?.textContent).toContain('# Revised'))
    await vi.waitFor(async () => expect((await host.load()).text).toBe('# Revised\n'))
  } finally {
    finish('')
    root.unmount()
    container.remove()
  }
})
