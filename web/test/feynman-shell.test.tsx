import '../src/styles.css'
import { createRoot } from 'react-dom/client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { cdp, page, userEvent } from 'vitest/browser'
import { App } from '../src/App'
import { createBrowserHost, createBrowserWorkspace } from '../src/host/browserHost'
import type {
  ResearchArtifact,
  ResearchCommand,
  ResearchConversation,
  ResearchForkMessage,
  ResearchTree,
  ResearchUpdate,
  ReviewUpdateEvent,
} from '../src/host/generated/mygo'
import { actionGroup, actionLabel, actionMeta } from '../src/plugins/feynman/actions'
import { WorkspaceResearch } from '../src/plugins/feynman'
import './helpers/enable-feynman'

const mocks = vi.hoisted(() => ({
  onResearchEvent: undefined as ((e: ResearchUpdate) => void) | undefined,
  onReviewEvent: undefined as ((e: ReviewUpdateEvent) => void) | undefined,
  steer: vi.fn(async () => {}),
  followUp: vi.fn(async () => {}),
  clearQueue: vi.fn(async () => ({ steering: [], followUp: [] })),
  abort: vi.fn(async () => {}),
  cancel: vi.fn(async () => {}),
  setModel: vi.fn(async (_root: string, provider: string, modelId: string) => ({
    id: modelId,
    provider,
    name: `${provider}/${modelId}`,
  })),
  setThinkingLevel: vi.fn(async () => {}),
  setSessionName: vi.fn(async () => {}),
  clone: vi.fn(async () => true),
  compact: vi.fn(async () => ({ summary: 'Earlier research', tokensBefore: 1000 })),
  forkMessages: vi.fn<() => Promise<ResearchForkMessage[]>>(async () => [
    { entryId: 'entry-1', text: 'First prompt about attention' },
    { entryId: 'entry-2', text: 'Second prompt about scaling' },
  ]),
  fork: vi.fn(async () => ({ cancelled: false, text: 'forked draft', sessionId: 'sess-fork' })),
  tree: vi.fn<() => Promise<ResearchTree>>(async () => ({
    leafId: 'node-2',
    tree: [
      {
        entry: { id: 'node-1', type: 'message', role: 'user', text: 'Root research question' },
        label: 'Main',
        children: [
          {
            entry: { id: 'node-2', type: 'message', role: 'assistant', text: 'Current leaf answer' },
            children: [],
          },
        ],
      },
    ],
  })),
  sessions: vi.fn<() => Promise<ResearchConversation[]>>(async () => [
    { id: 'sess-1', title: 'Initial research', updatedAt: '2026-11-19T10:00:00Z', current: true },
    { id: 'sess-2', title: 'Previous survey', updatedAt: '2026-11-18T10:00:00Z', current: false },
  ]),
  selectSession: vi.fn(async () => {}),
  deleteSession: vi.fn(async () => {}),
  prompt: vi.fn<() => Promise<string>>(async () => ''),
  artifacts: vi.fn<() => Promise<ResearchArtifact[]>>(async () => []),
  recoverArtifact: vi.fn(async (_root: string, rel: string) => `/ws/${rel}`),
  answer: vi.fn(async () => {}),
  answerConfirm: vi.fn(async () => {}),
  answerText: vi.fn(async () => {}),
  sessionStats: vi.fn(async () => ({
    sessionId: 'sess-1',
    userMessages: 1,
    assistantMessages: 1,
    toolCalls: 2,
    toolResults: 2,
    totalMessages: 4,
    tokens: { input: 1000, output: 250, cacheRead: 100, cacheWrite: 0, total: 1350 },
    cost: 0.042,
    contextUsage: { tokens: 1350, contextWindow: 200000, percent: 0.675 },
  })),
}))

vi.mock('../src/host/generated/mygo', async (original) => {
  const native = await original<typeof import('../src/host/generated/mygo')>()
  return {
    ...native,
    events: {
      ...native.events,
      researchUpdate: {
        on(fn: (e: ResearchUpdate) => void) {
          mocks.onResearchEvent = fn
          return () => {
            mocks.onResearchEvent = undefined
          }
        },
      },
      reviewUpdated: {
        on(fn: (e: ReviewUpdateEvent) => void) {
          mocks.onReviewEvent = fn
          return () => {
            mocks.onReviewEvent = undefined
          }
        },
      },
    },
    ResearchHost: {
      ...native.ResearchHost,
      connect: async (root: string) => {
        mocks.onResearchEvent?.({
          root,
          kind: 'config',
          models: [
            { id: 'claude-sonnet-4', provider: 'anthropic', name: 'Claude Sonnet 4', contextWindow: 200000 },
            { id: 'gpt-5', provider: 'openai', name: 'GPT-5', contextWindow: 128000 },
          ],
          thinkingLevels: ['low', 'medium', 'high'],
          thinkingLevel: 'medium',
          state: {
            model: { id: 'claude-sonnet-4', provider: 'anthropic', name: 'Claude Sonnet 4', contextWindow: 200000 },
            thinkingLevel: 'medium',
            isStreaming: false,
            isCompacting: false,
            sessionId: 'sess-1',
            sessionName: 'Initial research',
            autoCompactionEnabled: true,
            messageCount: 2,
            pendingMessageCount: 0,
          },
          config: [
            {
              id: 'model',
              name: 'Model',
              type: 'select',
              category: 'model',
              currentValue: 'anthropic/claude-sonnet-4',
              options: [
                { value: 'anthropic/claude-sonnet-4', name: 'Claude Sonnet 4' },
                { value: 'openai/gpt-5', name: 'GPT-5' },
              ],
            },
            {
              id: 'thinking_level',
              name: 'Thinking',
              type: 'select',
              category: 'thinking',
              currentValue: 'medium',
              options: [
                { value: 'low', name: 'Low' },
                { value: 'medium', name: 'Medium' },
                { value: 'high', name: 'High' },
              ],
            },
          ],
        })
        mocks.onResearchEvent?.({
          root,
          kind: 'commands',
          commands: [
            {
              name: 'custom-alpha',
              description: 'Run custom alpha workflow',
              source: 'prompt',
              sourceInfo: { source: 'prompt', scope: 'project', origin: 'top-level' },
            },
            {
              name: 'literature-map',
              description: 'Map citations',
              source: 'skill',
              sourceInfo: { source: 'skill', scope: 'user', origin: 'package' },
            },
            {
              name: 'websearch',
              description: 'Search the web',
              source: 'extension',
              sourceInfo: { source: 'extension', scope: 'project', origin: 'top-level' },
            },
          ],
        })
        mocks.onResearchEvent?.({
          root,
          kind: 'usage',
          used: 1350,
          size: 200000,
          stats: {
            sessionId: 'sess-1',
            userMessages: 1,
            assistantMessages: 1,
            toolCalls: 2,
            toolResults: 2,
            totalMessages: 4,
            tokens: { input: 1000, output: 250, cacheRead: 100, cacheWrite: 0, total: 1350 },
            cost: 0.042,
            contextUsage: { tokens: 1350, contextWindow: 200000, percent: 0.675 },
          },
        })
      },
      disconnect: async () => {},
      artifacts: mocks.artifacts,
      recoverArtifact: mocks.recoverArtifact,
      prompt: mocks.prompt,
      steer: mocks.steer,
      followUp: mocks.followUp,
      clearQueue: mocks.clearQueue,
      abort: mocks.abort,
      cancel: mocks.cancel,
      setModel: mocks.setModel,
      setThinkingLevel: mocks.setThinkingLevel,
      setSessionName: mocks.setSessionName,
      clone: mocks.clone,
      compact: mocks.compact,
      forkMessages: mocks.forkMessages,
      fork: mocks.fork,
      tree: mocks.tree,
      sessions: mocks.sessions,
      selectSession: mocks.selectSession,
      deleteSession: mocks.deleteSession,
      sessionStats: mocks.sessionStats,
      answer: mocks.answer,
      answerConfirm: mocks.answerConfirm,
      answerText: mocks.answerText,
    },
  }
})

beforeEach(() => {
  vi.clearAllMocks()
  mocks.prompt.mockResolvedValue('')
  mocks.artifacts.mockResolvedValue([])
})

describe('Dynamic commands without static tables', () => {
  it('groups strictly by source and sourceInfo and formats labels dynamically', () => {
    const promptCmd: ResearchCommand = {
      name: 'lit',
      source: 'prompt',
      sourceInfo: { scope: 'project', origin: 'top-level' },
    }
    const skillCmd: ResearchCommand = {
      name: 'skill:paper-writing',
      source: 'skill',
      sourceInfo: { source: 'skill', scope: 'user', origin: 'package' },
    }
    const extCmd: ResearchCommand = {
      name: 'subagents',
      sourceInfo: { source: 'extension', scope: 'temporary' },
    }

    expect(actionGroup(promptCmd)).toBe('Workflows')
    expect(actionGroup(skillCmd)).toBe('Skills')
    expect(actionGroup(extCmd)).toBe('Extensions')
    expect(actionLabel(promptCmd.name)).toBe('Lit')
    expect(actionLabel(skillCmd.name)).toBe('Paper writing')
    expect(actionMeta(promptCmd)).toContain('project')
    expect(actionMeta(skillCmd)).toContain('user')
  })
})

describe('Portrait Chat | Review right-pane shell', () => {
  it('renders two-segment .seg + MovingCapsule header, Portrait pickers, tabular-nums stats, and no native select/meter/terminal', async () => {
    const host = createBrowserHost('# Paper\n\nBody.\n', 'paper.md')
    const workspace = createBrowserWorkspace([host], { root: '/ws', name: 'ws', kind: 'folder' })
    const container = document.body.appendChild(document.createElement('div'))
    container.style.height = '100%'
    const root = createRoot(container)
    try {
      root.render(<App workspace={workspace} />)
      await expect.element(page.getByRole('radio', { name: /Chat/ })).toBeVisible()
      await expect.element(page.getByRole('radio', { name: /Review/ })).toBeVisible()

      // Two-segment .seg + MovingCapsule, no terminal tab
      const seg = container.querySelector('.fy-tabs.seg')
      expect(seg).not.toBeNull()
      expect(seg?.querySelector('.moving-capsule')).not.toBeNull()
      expect(seg?.querySelectorAll('[role="radio"]')).toHaveLength(2)
      expect(container.textContent).not.toContain('CLI')

      // No native <select> or <meter> inside the panel
      const panel = container.querySelector('.fy-panel')!
      expect(panel.querySelectorAll('select')).toHaveLength(0)
      expect(panel.querySelectorAll('meter')).toHaveLength(0)

      // Glass overlay styling on Actions command palette
      await userEvent.click(page.getByRole('button', { name: 'Research actions' }))
      const palette = container.querySelector<HTMLElement>('.fy-command-palette')!
      expect(palette).not.toBeNull()
      expect(palette.textContent).toContain('Workflows')
      expect(palette.textContent).toContain('Skills')
      expect(palette.textContent).toContain('Extensions')
      expect(getComputedStyle(palette).backdropFilter).not.toBe('none')
      await userEvent.click(page.getByRole('button', { name: 'Close actions' }))

      // Visible tabular-nums token/cost stats
      const statsEl = container.querySelector('[data-testid="fy-session-stats"]')
      expect(statsEl).not.toBeNull()
      expect(getComputedStyle(statsEl!).fontVariantNumeric).toContain('tabular-nums')
      expect(statsEl?.textContent).toContain('1,350')
      expect(statsEl?.textContent).toContain('$0.04')

      // Portrait model & thinking pickers
      await userEvent.click(page.getByRole('button', { name: /Model/ }))
      await expect.element(page.getByRole('button', { name: /GPT-5/ })).toBeVisible()
      await userEvent.click(page.getByRole('button', { name: /GPT-5/ }))
      expect(mocks.setModel).toHaveBeenCalledWith('/ws', 'openai', 'gpt-5')

      await userEvent.click(page.getByRole('button', { name: /Thinking/ }))
      await expect.element(page.getByRole('button', { name: 'High' })).toBeVisible()
      await userEvent.click(page.getByRole('button', { name: 'High' }))
      expect(mocks.setThinkingLevel).toHaveBeenCalledWith('/ws', 'high')
    } finally {
      root.unmount()
      container.remove()
    }
  })

  it('supports mid-turn steer, follow_up, clear_queue, abort, and animated working spinner', async () => {
    let finishPrompt!: (value: string) => void
    mocks.prompt.mockImplementation(() => new Promise<string>((resolve) => { finishPrompt = resolve }))

    const host = createBrowserHost('# Paper\n', 'paper.md')
    const workspace = createBrowserWorkspace([host], { root: '/ws', name: 'ws', kind: 'folder' })
    const container = document.body.appendChild(document.createElement('div'))
    container.style.height = '100%'
    const root = createRoot(container)
    try {
      root.render(<App workspace={workspace} />)
      const composer = await vi.waitFor(() => {
        const el = container.querySelector<HTMLTextAreaElement>('[aria-label="Research prompt"]')
        expect(el).not.toBeNull()
        return el!
      })

      await userEvent.fill(composer, 'Initial prompt')
      await userEvent.click(page.getByRole('button', { name: 'Send' }))

      // Animated working spinner visible
      const spinner = await vi.waitFor(() => {
        const el = container.querySelector('.fy-working .ph-spinner-gap')
        expect(el).not.toBeNull()
        return el!
      })
      expect(getComputedStyle(spinner).animationName).not.toBe('none')

      // Steer & Follow up mid-turn
      await userEvent.fill(composer, 'Focus on table 2')
      await userEvent.click(page.getByTestId('fy-steer'))
      expect(mocks.steer).toHaveBeenCalledWith('/ws', 'Focus on table 2', [])

      await userEvent.fill(composer, 'Then summarize findings')
      await userEvent.click(page.getByTestId('fy-follow-up'))
      expect(mocks.followUp).toHaveBeenCalledWith('/ws', 'Then summarize findings', [])
      await expect.poll(() => container.querySelector('[data-testid="fy-queue-bar"] .fy-queue-summary')?.textContent).toBe('1 follow-up')

      // Queue update from Go backend + Clear queue
      mocks.onResearchEvent?.({
        root: '/ws',
        kind: 'queue',
        queue: { steering: ['Focus on table 2'], followUp: ['Then summarize findings'] },
      })
      await expect.poll(() => container.querySelector('[data-testid="fy-queue-bar"] .fy-queue-summary')?.textContent).toBe('1 steering · 1 follow-up')
      await userEvent.click(page.getByTestId('fy-clear-queue'))
      expect(mocks.clearQueue).toHaveBeenCalledWith('/ws')
      await vi.waitFor(() => expect(container.querySelector('[data-testid="fy-queue-bar"]')).toBeNull())

      // Abort turn
      await userEvent.click(page.getByRole('button', { name: 'Abort research' }))
      expect(mocks.abort).toHaveBeenCalledWith('/ws')
      finishPrompt('')
    } finally {
      finishPrompt?.('')
      root.unmount()
      container.remove()
    }
  })

  it('lets Chat clone a conversation and compact its context', async () => {
    const host = createBrowserHost('# Paper\n', 'paper.md')
    const workspace = createBrowserWorkspace([host], { root: '/ws', name: 'ws', kind: 'folder' })
    const container = document.body.appendChild(document.createElement('div'))
    const root = createRoot(container)
    try {
      root.render(<App workspace={workspace} />)
      const history = page.getByRole('button', { name: 'Conversation history' })
      await userEvent.click(history)
      const clone = page.getByRole('button', { name: 'Clone conversation' })
      await expect.element(clone).toBeEnabled()
      await userEvent.click(clone)
      await expect.poll(() => mocks.clone.mock.calls.length).toBe(1)
      expect(mocks.clone).toHaveBeenCalledWith('/ws')
      await userEvent.click(history)
      await userEvent.click(page.getByRole('button', { name: 'Compact conversation' }))
      await expect.poll(() => mocks.compact.mock.calls.length).toBe(1)
      expect(mocks.compact).toHaveBeenCalledWith('/ws', '')
    } finally {
      root.unmount()
      container.remove()
    }
  })

  it('supports session fork, tree, rename, switch, and delete in SessionMenu', async () => {
    const host = createBrowserHost('# Paper\n', 'paper.md')
    const workspace = createBrowserWorkspace([host], { root: '/ws', name: 'ws', kind: 'folder' })
    const container = document.body.appendChild(document.createElement('div'))
    container.style.height = '100%'
    const root = createRoot(container)
    try {
      root.render(<App workspace={workspace} />)
      await expect.element(page.getByRole('button', { name: 'Conversation history' })).toBeEnabled()
      await userEvent.click(page.getByRole('button', { name: 'Conversation history' }))

      // Rename
      await userEvent.click(page.getByRole('button', { name: 'Rename conversation' }))
      const nameInput = page.getByLabelText('Conversation name')
      await userEvent.fill(nameInput, 'Renamed study')
      await userEvent.click(page.getByRole('button', { name: 'Save' }))
      expect(mocks.setSessionName).toHaveBeenCalledWith('/ws', 'Renamed study')
      await expect.poll(() => container.querySelector('.fy-heading-title small')?.textContent).toContain('Renamed study')

      // Tree viewer
      await userEvent.click(page.getByRole('button', { name: 'Conversation tree' }))
      await expect.poll(() => document.querySelector('[aria-label="Conversation tree"]')?.textContent).toContain('Current leaf answer')
      await expect.poll(() => document.querySelector('[aria-label="Conversation tree"]')?.textContent).toContain('Current')
      await userEvent.click(page.getByRole('button', { name: 'Tree' }))

      // Fork
      await userEvent.click(page.getByRole('button', { name: 'Fork conversation' }))
      await expect.element(page.getByRole('button', { name: 'First prompt about attention' })).toBeVisible()
      await userEvent.click(page.getByRole('button', { name: 'First prompt about attention' }))
      expect(mocks.fork).toHaveBeenCalledWith('/ws', 'entry-1')
      await expect.poll(() => container.querySelector<HTMLTextAreaElement>('[aria-label="Research prompt"]')?.value).toBe('forked draft')

      // Clear draft so switching is allowed, then switch & delete
      await userEvent.fill(page.getByLabelText('Research prompt'), '')
      await userEvent.click(page.getByRole('button', { name: 'Conversation history' }))
      await userEvent.click(page.getByRole('button', { name: 'Delete Previous survey' }))
      await userEvent.click(page.getByRole('button', { name: 'Delete', exact: true }))
      expect(mocks.deleteSession).toHaveBeenCalledWith('/ws', 'sess-2')
    } finally {
      root.unmount()
      container.remove()
    }
  })

  it('handles all 9 extension_ui_request types, code copy morph, safe images, and reduced-transparency fallback', async () => {
    const sessionCdp = cdp()
    const host = createBrowserHost('# Paper\n', 'paper.md')
    const workspace = createBrowserWorkspace([host], { root: '/ws', name: 'ws', kind: 'folder' })
    const container = document.body.appendChild(document.createElement('div'))
    container.style.height = '100%'
    const root = createRoot(container)
    const emit = (update: Omit<ResearchUpdate, 'root'>) => mocks.onResearchEvent?.({ root: '/ws', ...update })
    try {
      root.render(<App workspace={workspace} />)
      await expect.element(page.getByRole('radio', { name: /Chat/ })).toBeVisible()

      // 1. setTitle
      emit({ kind: 'setTitle', uiMethod: 'setTitle', title: 'Custom Title', text: 'Custom Title' })
      await expect.poll(() => container.querySelector('.fy-heading-title small')?.textContent).toContain('Custom Title')

      // 2. notify
      emit({ kind: 'notify', uiMethod: 'notify', message: 'Index synced', notifyType: 'warning' })
      await expect.poll(() => container.querySelector('.fy-notice[data-tone="warning"]')?.textContent).toContain('Index synced')

      // 3. setStatus
      emit({ kind: 'setStatus', uiMethod: 'setStatus', statusKey: 'sync', text: 'Syncing 2/2' })
      await expect.element(page.getByTestId('fy-ext-statuses')).toHaveTextContent('Syncing 2/2')

      // 4. setWidget
      emit({ kind: 'setWidget', uiMethod: 'setWidget', widgetKey: 'w1', widgetLines: ['Widget summary'], widgetPlacement: 'aboveEditor' })
      await expect.poll(() => container.querySelector('[data-widget-key="w1"]')?.textContent).toBe('Widget summary')

      // 5. set_editor_text
      emit({ kind: 'set_editor_text', uiMethod: 'set_editor_text', text: 'Pre-filled draft' })
      await expect.poll(() => container.querySelector<HTMLTextAreaElement>('[aria-label="Research prompt"]')?.value).toBe('Pre-filled draft')

      // 6. confirm
      emit({ kind: 'confirm', uiMethod: 'confirm', requestId: 'c1', text: 'Proceed with edit?', options: [{ id: 'yes', name: 'Confirm', kind: 'confirm' }, { id: 'no', name: 'Cancel', kind: 'cancel' }] })
      await userEvent.click(page.getByRole('button', { name: 'Confirm' }))
      expect(mocks.answerConfirm).toHaveBeenCalledWith('/ws', 'c1', true, false)
      emit({ kind: 'permissionDone', requestId: 'c1' })

      // 7. select (>4 options uses Portrait listbox, no <select>)
      emit({
        kind: 'select',
        uiMethod: 'select',
        requestId: 's1',
        text: 'Pick target',
        options: ['a', 'b', 'c', 'd', 'e'].map((id) => ({ id, name: `Option ${id}` })),
      })
      expect(container.querySelector('.fy-panel select')).toBeNull()
      await userEvent.click(page.getByRole('option', { name: 'Option c' }))
      await userEvent.click(page.getByRole('button', { name: 'Confirm' }))
      expect(mocks.answer).toHaveBeenCalledWith('/ws', 's1', 'c')
      emit({ kind: 'permissionDone', requestId: 's1' })

      // 8. input & 9. editor
      emit({ kind: 'input', uiMethod: 'input', requestId: 'i1', text: 'Enter label' })
      await userEvent.fill(page.getByLabelText('Enter label'), 'v1')
      await userEvent.click(page.getByRole('button', { name: 'Submit' }))
      expect(mocks.answerText).toHaveBeenCalledWith('/ws', 'i1', 'v1', false)
      emit({ kind: 'permissionDone', requestId: 'i1' })

      emit({ kind: 'editor', uiMethod: 'editor', requestId: 'e1', text: 'Edit abstract', prefill: 'Draft' })
      await userEvent.fill(page.getByLabelText('Edit abstract'), 'Updated')
      await userEvent.click(page.getByRole('button', { name: 'Submit' }))
      expect(mocks.answerText).toHaveBeenCalledWith('/ws', 'e1', 'Updated', false)
      emit({ kind: 'permissionDone', requestId: 'e1' })

      // Code block & safe image rendering in chat
      emit({ kind: 'user', text: 'Show code and plot' })
      emit({
        kind: 'assistant',
        text: '```ts\nconst x = 1\n```\n\n![Plot](https://example.com/plot.png)',
      })
      await expect.poll(() => container.querySelector('.fy-chat-image')?.getAttribute('src')).toBe('https://example.com/plot.png')
      await userEvent.click(page.getByRole('button', { name: 'Copy code' }))
      await expect.poll(() => container.querySelector('.fy-code-block .mgo-copy-morph')).not.toBeNull()

      // Reduced transparency fallback on glass overlay
      await sessionCdp.send('Emulation.setEmulatedMedia', {
        features: [{ name: 'prefers-reduced-transparency', value: 'reduce' }],
      })
      await userEvent.click(page.getByRole('button', { name: 'Research actions' }))
      const palette = container.querySelector<HTMLElement>('.fy-command-palette')!
      expect(getComputedStyle(palette).backdropFilter).toBe('none')
    } finally {
      await sessionCdp.send('Emulation.setEmulatedMedia', { features: [] })
      root.unmount()
      container.remove()
    }
  })

  it('displays Review tab pending badge, recovery Restore action, and settings slot hook', async () => {
    mocks.artifacts.mockResolvedValue([
      { path: 'outputs/lost.md', target: '', proposal: false, missing: true },
    ])
    const container = document.body.appendChild(document.createElement('div'))
    container.style.height = '100%'
    const root = createRoot(container)
    const openDoc = vi.fn(async (path: string) => ({
      id: 'doc-1',
      snapshot: async () => ({ path, text: '# Restored', version: 1 }),
    }))
    try {
      root.render(
        <WorkspaceResearch
          root="/ws"
          documentID="doc-1"
          open={openDoc}
          openFolder={() => {}}
          renderSettings={({ onClose }) => (
            <div data-testid="feynman-settings-stub">
              <button type="button" onClick={onClose}>
                Back to research
              </button>
            </div>
          )}
        />,
      )
      await expect.element(page.getByRole('radio', { name: /Review/ })).toBeVisible()

      // Pending proposal via reviewUpdated shows tabular-nums badge
      mocks.onReviewEvent?.({
        workspaceRoot: '/ws',
        revision: 1,
        proposals: [
          {
            id: 'prop-1',
            workspaceRoot: '/ws',
            documentId: 'doc-1',
            path: '/ws/paper.md',
            source: 'mcp',
            summary: 'Fix citation',
            baseText: '# Paper\n',
            proposedText: '# Paper (Revised)\n',
            liveText: '# Paper\n',
            mergedText: '# Paper (Revised)\n',
            hunks: [],
            createdAt: Date.now(),
          },
        ],
      })
      const badge = await vi.waitFor(() => {
        const el = container.querySelector('[data-testid="review-pending-badge"]')
        expect(el).not.toBeNull()
        return el!
      })
      expect(badge.textContent).toBe('1')
      expect(getComputedStyle(badge).fontVariantNumeric).toContain('tabular-nums')

      // Switch to Review tab and verify proposal + Restore missing file
      await userEvent.click(page.getByRole('radio', { name: /Review/ }))
      await expect.element(page.getByText('Fix citation')).toBeVisible()
      await userEvent.click(page.getByRole('button', { name: /Restore lost\.md/ }))
      expect(mocks.recoverArtifact).toHaveBeenCalledWith('/ws', 'outputs/lost.md')
      expect(openDoc).toHaveBeenCalledWith('/ws/outputs/lost.md')

      // Settings slot hook works in-pane
      await userEvent.click(page.getByTestId('feynman-settings-toggle'))
      await expect.element(page.getByTestId('feynman-settings-stub')).toBeVisible()
      await userEvent.click(page.getByRole('button', { name: 'Back to research' }))
      await expect.element(page.getByRole('radio', { name: /Review/ })).toBeVisible()
    } finally {
      root.unmount()
      container.remove()
    }
  })
})
