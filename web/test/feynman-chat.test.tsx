import { createRoot } from 'react-dom/client'
import { expect, it, vi } from 'vitest'
import { userEvent } from 'vitest/browser'
import { App } from '../src/App'
import { createBrowserHost, createBrowserWorkspace } from '../src/host/browserHost'
import type { ResearchUpdate } from '../src/host/generated/mygo'
import './helpers/enable-feynman'

const adapter = vi.hoisted(() => ({
  send: vi.fn(async () => ''), openCitation: vi.fn(async () => {}),
  answer: vi.fn(async () => {}), answerText: vi.fn(async () => {}),
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
        { name: 'skill:paper-writing', description: 'Write a paper from research notes' },
      ] }),
      disconnect: async () => {}, artifacts: async () => [], prompt: adapter.send, openCitation: adapter.openCitation,
      answer: adapter.answer, answerText: adapter.answerText,
    },
  }
})

it('returns extension form input and keeps a draft when an extension suggests a workflow', async () => {
  const host = createBrowserHost('# Paper\n', 'draft.md')
  const container = document.body.appendChild(document.createElement('div'))
  const root = createRoot(container)
  const emit = (update: Omit<ResearchUpdate, 'root'>) => adapter.onEvent?.({ root: '/forms', ...update })
  try {
    root.render(<App workspace={createBrowserWorkspace([host], {root: '/forms'})} />)
    const composer = await vi.waitFor(() => {
      const field = container.querySelector<HTMLTextAreaElement>('[aria-label="Research prompt"]')
      expect(field).not.toBeNull()
      return field!
    })
    await userEvent.fill(composer, 'compare two reviews')
    emit({kind: 'editor', requestId: 'check', text: 'Review scope', prefill: 'First paragraph'})
    const editor = await vi.waitFor(() => {
      const field = container.querySelector<HTMLTextAreaElement>('textarea[aria-label="Review scope"]')
      expect(field?.value).toBe('First paragraph')
      return field!
    })
    await userEvent.fill(editor, 'Title\nMethods')
    await userEvent.click(editor.form!.querySelector<HTMLButtonElement>('button')!)
    expect(adapter.answerText).toHaveBeenCalledWith('/forms', 'check', 'Title\nMethods', false)
    emit({kind: 'permissionDone', requestId: 'check'})
    emit({kind: 'editorText', text: '/lit'})
    await vi.waitFor(() => expect(container.querySelector('.fy-suggestion')).not.toBeNull())
    expect(composer.value).toBe('compare two reviews')
    await userEvent.click(container.querySelector<HTMLButtonElement>('.fy-suggestion button')!)
    expect(composer.value).toBe('compare two reviews')
    expect(container.querySelector('[aria-label="Remove Literature review"]')).not.toBeNull()
    emit({kind: 'input', requestId: 'follow-up', text: 'Source title'})
    const cancel = await vi.waitFor(() => {
      const field = container.querySelector<HTMLInputElement>('input[aria-label="Source title"]')
      expect(field).not.toBeNull()
      return field!.form!.querySelector<HTMLButtonElement>('button[type=button]')!
    })
    await userEvent.click(cancel)
    expect(adapter.answerText).toHaveBeenCalledWith('/forms', 'follow-up', '', true)
    emit({kind: 'permission', requestId: 'catalog', text: 'Tools', options: [{id: 'choice-0', name: 'web_search — Search the web'}]})
    const select = await vi.waitFor(() => {
      const field = container.querySelector<HTMLSelectElement>('[aria-label="Feynman choices"]')
      expect(field).not.toBeNull()
      return field!
    })
    await userEvent.selectOptions(select, 'choice-0')
    await userEvent.click(select.form!.querySelector<HTMLButtonElement>('button[value=use]')!)
    expect(adapter.answer).toHaveBeenCalledWith('/forms', 'catalog', 'choice-0')
    expect(composer.value).toBe('compare two reviews Use the web_search tool.')
  } finally {
    root.unmount()
    container.remove()
  }
})

it('chooses an exact slash workflow, sends with Enter and opens citations from the streamed response', async () => {
  const host = createBrowserHost('# Source\n', 'paper.md')
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const workspace = createBrowserWorkspace([host], { root: '/papers' })
  const emit = (update: Omit<ResearchUpdate, 'root'>) => adapter.onEvent?.({ root: '/papers', ...update })
  try {
    root.render(<App workspace={workspace} />)
    await vi.waitFor(() => expect(container.querySelector('.fy-launchers button')).not.toBeNull())
    const field = container.querySelector<HTMLTextAreaElement>('[aria-label="Research prompt"]')!
    await userEvent.fill(field, 'score reading')
    container.querySelector<HTMLButtonElement>('[aria-label="Research actions"]')!.click()
    const filter = await vi.waitFor(() => {
      const input = container.querySelector<HTMLInputElement>('[aria-label="Find a research action"]')
      expect(input).not.toBeNull()
      return input!
    })
    await userEvent.fill(filter, 'paper writing')
    await userEvent.keyboard('{Enter}')
    expect(field.value).toBe('score reading')
    const remove = container.querySelector<HTMLButtonElement>('[aria-label="Remove Paper writing"]')!
    await userEvent.click(remove)
    expect(field.value).toBe('score reading')
    await userEvent.fill(field, '/lit')
    await vi.waitFor(() => expect(container.querySelector('[role="option"][aria-selected=true]')?.textContent).toContain('/lit'))
    await userEvent.keyboard('{Enter}')
    expect(field.value).toBe('')
    expect(container.querySelector('[aria-label="Remove Literature review"]')).not.toBeNull()
    expect(adapter.send).not.toHaveBeenCalled()
    await userEvent.fill(field, 'score reading')
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
