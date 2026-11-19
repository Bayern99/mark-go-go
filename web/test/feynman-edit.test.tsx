import '../src/styles.css'
import { createRoot } from 'react-dom/client'
import { expect, it, vi } from 'vitest'
import { userEvent } from 'vitest/browser'
import { applyNativeEdit, nativeEditCapabilities } from '../src/editor/nativeEdit'
import { App } from '../src/App'
import type { ResearchArtifact } from '../src/host/generated/mygo'
import { createBrowserHost, createBrowserWorkspace } from '../src/host/browserHost'
import { computeMyersHunks, reconcileProposal } from '../src/plugins/DocumentDiff'
import './helpers/enable-feynman'

const prompt = vi.hoisted(() => vi.fn<() => Promise<string>>())
const artifacts = vi.hoisted(() => vi.fn<() => Promise<ResearchArtifact[]>>().mockResolvedValue([]))
const readArtifact = vi.hoisted(() => vi.fn<(root: string, rel: string) => Promise<string>>())
vi.mock('../src/host/generated/mygo', async importOriginal => {
  const native = await importOriginal<typeof import('../src/host/generated/mygo')>()
  return {
    ...native,
    events: {
      ...native.events,
      researchUpdate: { on: () => () => {} },
      reviewUpdated: { on: () => () => {} },
    },
    ResearchHost: {
      connect: async () => {}, disconnect: async () => {},
      artifacts, readArtifact, prompt,
    },
  }
})

it('computes minimal Myers hunks with 1-based line numbers and 3 lines of context', () => {
  const before = [
    'Line 1',
    'Line 2',
    'Line 3',
    'Line 4',
    'Line 5 old',
    'Line 6',
    'Line 7',
    'Line 8',
    'Line 9',
    'Line 10 old',
    'Line 11',
  ].join('\n') + '\n'
  const after = [
    'Line 1',
    'Line 2',
    'Line 3',
    'Line 4',
    'Line 5 new',
    'Line 6',
    'Line 7',
    'Line 8',
    'Line 9',
    'Line 10 new',
    'Line 11',
  ].join('\n') + '\n'

  const hunks = computeMyersHunks(before, after)
  expect(hunks).toHaveLength(2)
  expect(hunks[0].id).toBe('hunk-1')
  expect(hunks[0].oldStart).toBe(2)
  expect(hunks[0].oldLines).toBe(7)
  expect(hunks[0].changeOldLine).toBe(5)
  expect(hunks[0].lines.filter(l => l.kind === 'context')).toHaveLength(6)
  expect(hunks[0].lines.find(l => l.kind === 'delete')).toMatchObject({ oldLine: 5, text: 'Line 5 old' })
  expect(hunks[0].lines.find(l => l.kind === 'insert')).toMatchObject({ newLine: 5, text: 'Line 5 new' })

  expect(hunks[1].id).toBe('hunk-2')
  expect(hunks[1].changeOldLine).toBe(10)
})

it('manages a multi-file queue of Change proposals without blocking other files', async () => {
  const draft = createBrowserHost('# Draft\n', 'draft.md')
  const paper = createBrowserHost('# Paper\n', 'paper.md')
  const workspace = createBrowserWorkspace([draft, paper], { root: '/demo' })
  const container = document.body.appendChild(document.createElement('div'))
  const root = createRoot(container)
  const button = (text: string) => Array.from(container.querySelectorAll('button')).find(b => b.textContent?.trim() === text)!
  const files = () => container.querySelector<HTMLButtonElement>('.fy-tabs button:nth-of-type(2)')!
  const comparison = () => container.querySelector('[aria-label="Proposed document edit"]')
  artifacts.mockResolvedValue([{ path: 'paper.md', target: '/demo/paper.md', proposal: true }])
  prompt.mockResolvedValue('# Revised draft\n')
  readArtifact.mockResolvedValue('# Revised paper\n')
  try {
    root.render(<App workspace={workspace} />)
    await vi.waitFor(() => expect(files()).toBeTruthy())
    await userEvent.click(files())
    await vi.waitFor(() => expect(button('Review paper.md')).toBeTruthy())
    await userEvent.click(container.querySelector<HTMLButtonElement>('.fy-tabs button:nth-of-type(1)')!)
    await userEvent.click(container.querySelector<HTMLInputElement>('[aria-label="Edit this document"]')!)
    await userEvent.fill(container.querySelector<HTMLTextAreaElement>('[aria-label="Research prompt"]')!, 'Revise the draft')
    await userEvent.click(button('Send'))
    await vi.waitFor(() => expect(comparison()?.textContent).toContain('Revised draft'))

    // Multi-file queue: reviewing paper.md is NOT blocked while draft.md proposal is pending,
    // and Edit document in Chat remains enabled so additional edits can be queued.
    await userEvent.click(container.querySelector<HTMLButtonElement>('.fy-tabs button:nth-of-type(1)')!)
    const editToggle = container.querySelector<HTMLInputElement>('[aria-label="Edit this document"]')!
    expect(editToggle.disabled).toBe(false)
    await userEvent.fill(container.querySelector<HTMLTextAreaElement>('[aria-label="Research prompt"]')!, 'Queue another edit')
    expect(button('Send').disabled).toBe(false)
    await userEvent.fill(container.querySelector<HTMLTextAreaElement>('[aria-label="Research prompt"]')!, '')
    await workspace.activate(paper.id)
    await userEvent.click(files())
    expect(button('Review paper.md').disabled).toBe(false)
    await userEvent.click(button('Review paper.md'))
    await vi.waitFor(() => expect(comparison()?.textContent).toContain('Revised paper'))
    expect((await workspace.snapshot()).activeId).toBe(paper.id)

    // Both draft.md and paper.md are queued; switch back to draft.md via the multi-file queue switcher and dismiss it.
    const draftQueueItem = Array.from(container.querySelectorAll<HTMLButtonElement>('[data-proposal-file]')).find(
      b => b.dataset.proposalFile?.endsWith('draft.md'),
    )!
    expect(draftQueueItem).toBeTruthy()
    await userEvent.click(draftQueueItem)
    await vi.waitFor(async () => expect((await workspace.snapshot()).activeId).toBe(draft.id))
    await vi.waitFor(() => expect(comparison()?.textContent).toContain('Revised draft'))
    await userEvent.click(button('Dismiss'))

    // paper.md's proposal remains in the queue; switch to it and apply in Markdown Preview.
    await vi.waitFor(() => expect(comparison()?.textContent).toContain('Revised paper'))
    if ((await workspace.snapshot()).activeId !== paper.id) {
      await userEvent.click(button('Open document paper.md'))
    }
    await vi.waitFor(async () => expect((await workspace.snapshot()).activeId).toBe(paper.id))
    await userEvent.click(button('Apply in Markdown'))
    await vi.waitFor(async () => expect((await paper.load()).text).toBe('# Revised paper\n'))
    expect((await draft.load()).text).toBe('# Draft\n')
    // Stays in Markdown Preview mode (not forced into CodeMirror source mode).
    expect(container.querySelector('.mgo-source-root')?.hasAttribute('hidden')).toBe(true)
  } finally {
    artifacts.mockResolvedValue([])
    root.unmount()
    container.remove()
  }
})

it('supports selective per-hunk Accept/Reject, concurrent non-overlapping merge vs conflict detection, and in-mode Undo', async () => {
  const initialText = '# Study\n\nParagraph 1 original.\n\nParagraph 2 original.\n\nParagraph 3 original.\n'
  const proposedText = '# Study\n\nParagraph 1 by agent.\n\nParagraph 2 original.\n\nParagraph 3 by agent.\n'
  const host = createBrowserHost(initialText, 'study.md')
  const workspace = createBrowserWorkspace([host], { root: '/demo' })
  const container = document.body.appendChild(document.createElement('div'))
  const root = createRoot(container)
  const button = (text: string) => Array.from(container.querySelectorAll('button')).find(b => b.textContent?.trim() === text)
  const comparison = () => container.querySelector('[aria-label="Proposed document edit"]')
  prompt.mockResolvedValue(proposedText)

  try {
    root.render(<App workspace={workspace} />)
    await vi.waitFor(() => expect(container.querySelector('textarea[aria-label="Research prompt"]')).toBeTruthy())
    await userEvent.click(container.querySelector<HTMLInputElement>('[aria-label="Edit this document"]')!)
    await userEvent.fill(container.querySelector<HTMLTextAreaElement>('[aria-label="Research prompt"]')!, 'Update paragraphs 1 and 3')
    await userEvent.click(button('Send')!)

    await vi.waitFor(() => expect(container.querySelectorAll('.diff-hunk')).toHaveLength(2))

    // User edits unrelated Paragraph 2 while proposal is pending (non-overlapping concurrent edit).
    const p2Node = Array.from(container.querySelectorAll('.ProseMirror p')).find(p => p.textContent === 'Paragraph 2 original.')!
    await userEvent.click(p2Node)
    await userEvent.keyboard(' Edited live.')
    await vi.waitFor(async () => expect((await host.load()).text).toContain('Paragraph 2 original. Edited live.'))

    // Both hunks remain conflict-free because Paragraph 2 does not overlap Paragraph 1 or Paragraph 3.
    const hunksAfterUnrelated = container.querySelectorAll<HTMLElement>('.diff-hunk')
    expect(hunksAfterUnrelated[0].dataset.conflict).toBe('false')
    expect(hunksAfterUnrelated[1].dataset.conflict).toBe('false')

    // Selectively Accept hunk-1 and Reject hunk-2.
    await userEvent.click(container.querySelector<HTMLButtonElement>('button[aria-label="Accept hunk-1"]')!)
    await userEvent.click(container.querySelector<HTMLButtonElement>('button[aria-label="Reject hunk-2"]')!)
    await userEvent.click(button('Apply in Markdown')!)

    const expectedMerged = '# Study\n\nParagraph 1 by agent.\n\nParagraph 2 original. Edited live.\n\nParagraph 3 original.\n'
    await vi.waitFor(async () => expect((await host.load()).text).toBe(expectedMerged))
    expect(comparison()).toBeNull()
    // Verified still in Markdown Preview mode.
    expect(container.querySelector('.mgo-source-root')?.hasAttribute('hidden')).toBe(true)

    // Single-step Undo in Markdown Preview reverts the applied proposal cleanly back to the user's live edit.
    expect(applyNativeEdit(host.id, 'undo')).toBe(true)
    const expectedAfterUndo = '# Study\n\nParagraph 1 original.\n\nParagraph 2 original. Edited live.\n\nParagraph 3 original.\n'
    await vi.waitFor(async () => expect((await host.load()).text).toBe(expectedAfterUndo))

    // Verify overlapping concurrent edit flags Conflict on the colliding hunk only.
    const reconciled = reconcileProposal(
      initialText,
      proposedText,
      '# Study\n\nParagraph 1 user collision.\n\nParagraph 2 original.\n\nParagraph 3 original.\n',
    )
    expect(reconciled.hasConflict).toBe(true)
    expect(reconciled.hunks[0].conflict).toBe(true)
    expect(reconciled.hunks[1].conflict).toBe(false)
    expect(reconciled.mergedText).toBe(
      '# Study\n\nParagraph 1 user collision.\n\nParagraph 2 original.\n\nParagraph 3 by agent.\n',
    )
  } finally {
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
    return Array.from(container.querySelectorAll('button')).find(b => b.textContent?.trim() === label)
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
    await vi.waitFor(() => expect(container.querySelector('.ProseMirror')?.textContent).toContain('Revised'))
    await vi.waitFor(async () => expect((await host.load()).text).toBe('# Revised\n'))
    expect((await second.load()).text).toBe('# Reading notes\n')
  } finally {
    finish('')
    root.unmount()
    container.remove()
  }
})
