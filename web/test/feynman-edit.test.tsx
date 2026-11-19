import { createRoot } from 'react-dom/client'
import { expect, it, vi } from 'vitest'
import { userEvent } from 'vitest/browser'
import { applyNativeEdit, nativeEditCapabilities } from '../src/editor/nativeEdit'
import { App } from '../src/App'
import type { ResearchArtifact } from '../src/host/generated/mygo'
import { createBrowserHost, createBrowserWorkspace } from '../src/host/browserHost'
import './helpers/enable-feynman'

const prompt = vi.hoisted(() => vi.fn<() => Promise<string>>())
const artifacts = vi.hoisted(() => vi.fn<() => Promise<ResearchArtifact[]>>().mockResolvedValue([]))
const readArtifact = vi.hoisted(() => vi.fn<() => Promise<string>>())
vi.mock('../src/host/generated/mygo', async importOriginal => {
  const native = await importOriginal<typeof import('../src/host/generated/mygo')>()
  return {
    ...native,
    events: { ...native.events, researchUpdate: { on: () => () => {} } },
    ResearchHost: {
      connect: async () => {}, disconnect: async () => {},
      artifacts, readArtifact, prompt,
    },
  }
})

it('keeps the pending draft and opens a different file and its comparison with one review action', async () => {
  const draft = createBrowserHost('# Draft\n', 'draft.md')
  const paper = createBrowserHost('# Paper\n', 'paper.md')
  const workspace = createBrowserWorkspace([draft, paper], { root: '/demo' })
  const container = document.body.appendChild(document.createElement('div'))
  const root = createRoot(container)
  const button = (text: string) => Array.from(container.querySelectorAll('button')).find(b => b.textContent === text)!
  const files = () => container.querySelector<HTMLButtonElement>('.fy-tabs button:nth-child(2)')!
  const comparison = () => container.querySelector('[aria-label="Proposed document edit"]')
  artifacts.mockResolvedValue([{ path: 'paper.md', target: '/demo/paper.md', proposal: true }])
  prompt.mockResolvedValue('# Revised draft\n')
  readArtifact.mockResolvedValue('# Revised paper\n')
  try {
    root.render(<App workspace={workspace} />)
    await vi.waitFor(() => expect(button('Review paper.md')).toBeTruthy())
    await userEvent.click(container.querySelector<HTMLInputElement>('[aria-label="Edit this document"]')!)
    await userEvent.fill(container.querySelector<HTMLTextAreaElement>('[aria-label="Research prompt"]')!, 'Revise the draft')
    await userEvent.click(button('Send'))
    await vi.waitFor(() => expect(comparison()?.textContent).toContain('Revised draft'))

    await workspace.activate(paper.id)
    await userEvent.click(files())
    expect(button('Review paper.md').disabled).toBe(true)
    expect(readArtifact).not.toHaveBeenCalled()
    await userEvent.click(button('Review pending edit · draft.md'))
    await vi.waitFor(() => expect(comparison()?.textContent).toContain('Revised draft'))
    expect((await workspace.snapshot()).activeId).toBe(draft.id)
    await userEvent.click(button('Dismiss'))

    await userEvent.click(files())
    await userEvent.click(button('Review paper.md'))
    await vi.waitFor(() => expect(comparison()?.textContent).toContain('Revised paper'))
    expect((await workspace.snapshot()).activeId).toBe(paper.id)
    expect((await paper.load()).text).toBe('# Paper\n')
    expect((await draft.load()).text).toBe('# Draft\n')
    await userEvent.click(button('Apply in Markdown'))
    await vi.waitFor(async () => expect((await paper.load()).text).toBe('# Revised paper\n'))
  } finally {
    artifacts.mockResolvedValue([])
    root.unmount()
    container.remove()
  }
})

it('reviews a proposal against the same document after it is renamed during research', async () => {
  const host = createBrowserHost('# Original\n', 'draft.md')
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  let finish!: (text: string) => void
  prompt.mockClear().mockReturnValue(new Promise(resolve => { finish = resolve }))
  function button(label: string) {
    return Array.from(container.querySelectorAll('button')).find(b => b.textContent === label)
  }
  try {
    const second = createBrowserHost('# Reading notes\n', 'notes.md')
    const workspace = createBrowserWorkspace([host, second], { root: '/demo' })
    root.render(<App workspace={workspace} />)
    await vi.waitFor(() => expect(container.querySelector('textarea[aria-label="Research prompt"]')).toBeTruthy())
    const composer = container.querySelector<HTMLTextAreaElement>('textarea[aria-label="Research prompt"]')!
    await userEvent.fill(composer, 'Revise this draft')
    expect(nativeEditCapabilities(host.id).canUndo).toBe(true)
    expect(applyNativeEdit(host.id, 'undo')).toBe(true)
    await vi.waitFor(() => expect(composer.value).toBe(''))
    expect(nativeEditCapabilities(host.id).canRedo).toBe(true)
    expect(applyNativeEdit(host.id, 'redo')).toBe(true)
    await vi.waitFor(() => expect(composer.value).toBe('Revise this draft'))
    await userEvent.click(container.querySelector<HTMLButtonElement>('[data-testid="left-tab-outline.panel"]')!)
    await workspace.activate(second.id)
    await vi.waitFor(() => expect(container.querySelector('.outline-link')?.textContent).toBe('Reading notes'))
    expect(container.querySelector('[aria-label="Research prompt"]')).toBe(composer)
    expect(composer.value).toBe('Revise this draft')
    await workspace.activate(host.id)
    await vi.waitFor(() => expect(container.querySelector('.outline-link')?.textContent).toBe('Original'))
    container.querySelector<HTMLButtonElement>('[data-testid="view-mode-genui"]')!.click()
    const settings = await vi.waitFor(() => {
      const trigger = container.querySelector<HTMLButtonElement>('[aria-label="GenUI settings"]')
      if (!trigger) throw new Error('GenUI settings not ready')
      return trigger
    })
    settings.click()
    await vi.waitFor(() => expect(container.querySelector('[data-testid="center-pane"] [data-testid="genui-panel"]')).not.toBeNull())
    expect(container.querySelector('aside[aria-label="Feynman"] [role=tablist]')).toBeNull()
    expect(container.querySelector('[data-testid="right-pane"] textarea[aria-label="Research prompt"]')).toBe(composer)
    expect(composer.value).toBe('Revise this draft')
    settings.click()
    container.querySelector<HTMLInputElement>('.plugin-panel input[type=checkbox]')!.click()
    button('Send')!.click()
    await vi.waitFor(() => expect(prompt).toHaveBeenCalled())
    host.setPath('/demo/renamed.md')
    second.setPath('/demo/draft.md')
    await workspace.activate(second.id)
    finish('# Revised\n')
    await vi.waitFor(() => expect(button('Open document draft.md')).toBeTruthy())
    expect(button('Compare current text')).toBeUndefined()
    expect(button('Apply in Markdown')).toBeUndefined()
    button('Open document draft.md')!.click()
    await vi.waitFor(() => expect(button('Compare current text')).toBeTruthy())
    expect((await workspace.snapshot()).activeId).toBe(host.id)
    expect(button('Apply in Markdown')).toBeUndefined()
    button('Compare current text')!.click()
    await vi.waitFor(() => expect(button('Apply in Markdown')).toBeTruthy())
    button('Apply in Markdown')!.click()
    await vi.waitFor(() => expect(container.querySelector('.cm-content')?.textContent).toContain('# Revised'))
    await vi.waitFor(async () => expect((await host.load()).text).toBe('# Revised\n'))
    expect((await second.load()).text).toBe('# Reading notes\n')
  } finally {
    finish('')
    root.unmount()
    container.remove()
  }
})
