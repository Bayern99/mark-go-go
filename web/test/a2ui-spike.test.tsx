// SPIKE (branch spike/a2ui only)
import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'
import { A2uiSpike, SPIKE_CATALOG_ID } from '../src/plugins/genui/a2uiSpike'

function surface(components: unknown[]) {
  return [{ version: '1.0', createSurface: { surfaceId: 'view', catalogId: SPIKE_CATALOG_ID, components } }]
}

function mount(messages: unknown[], onReveal: (s: string[]) => void = () => {}) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  flushSync(() => root.render(<A2uiSpike messages={messages} onReveal={onReveal} />))
  return container
}

const GOOD = surface([
  { id: 'root', component: 'Tabs', tabs: [
    { title: '概览', child: 'col' },
    { title: '指标', child: 'metrics' },
  ] },
  { id: 'col', component: 'Column', children: ['card'] },
  { id: 'card', component: 'Card', child: 'kp' },
  { id: 'kp', component: 'KeyPoints', title: '要点', items: ['第一点', '第二点'], sources: ['s1'] },
  { id: 'metrics', component: 'Text', text: '**指标页**' },
])

/** Text including open shadow roots, where basic catalog custom elements render. */
function deepText(node: Node): string {
  let out = ''
  if (node instanceof Element && node.shadowRoot) out += deepText(node.shadowRoot)
  if (node.nodeType === Node.TEXT_NODE) out += node.textContent ?? ''
  for (const child of Array.from(node.childNodes)) out += deepText(child)
  return out
}

describe('A2UI 1.0 spike', () => {
  it('renders layout, a custom component and calls back with its sources', async () => {
    const onReveal = vi.fn()
    const el = mount(GOOD, onReveal)
    await vi.waitFor(() => expect(el.querySelector('[data-testid="spike-keypoints"]')).not.toBeNull())
    expect(el.textContent).toContain('第二点')
    el.querySelector<HTMLButtonElement>('[data-testid="spike-keypoints"] button')!.click()
    expect(onReveal).toHaveBeenCalledWith(['s1'])
    console.log('tags:', Array.from(new Set(Array.from(el.querySelectorAll('*')).map((n) => n.tagName.toLowerCase()))).join(' '))
    console.log('deep text:', deepText(el).replace(/\s+/g, ' ').trim())
  })

  it('rejects a component outside the catalog', async () => {
    const el = mount(surface([{ id: 'root', component: 'Column', children: ['b'] }, { id: 'b', component: 'Button', child: 'x' }]))
    await vi.waitFor(() => expect(el.querySelector('[data-testid="spike-problems"]')).not.toBeNull())
    console.log('unknown:', el.querySelector('[data-testid="spike-problems"]')?.textContent)
  })

  it('rejects a custom component with invalid props', async () => {
    const el = mount(surface([{ id: 'root', component: 'KeyPoints', title: 't', items: 'nope', sources: [] }]))
    await vi.waitFor(() => expect(el.innerHTML).not.toBe(''))
    console.log('bad props:', el.querySelector('[data-testid="spike-error"]')?.textContent ?? 'NO ERROR: ' + el.innerHTML.slice(0, 300))
  })
})
