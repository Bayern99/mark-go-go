import '../src/styles.css'
import { createRoot } from 'react-dom/client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { page, userEvent } from 'vitest/browser'
import type {
  CLIAlphaAnnotateResult,
  CLIAlphaPaperResult,
  CLIAlphaSearchResult,
  CLIAlphaStatusResult,
  CLICommandResult,
  CLIDoctorResult,
  CLIMCPListResult,
  CLIModelListResult,
  CLIPackageListResult,
  CLISearchStatusResult,
  CLIStatusResult,
  CLITerminalLaunchResult,
  ResearchFocusEvent,
  ResearchModel,
  ResearchUpdate,
} from '../src/host/generated/mygo'
import { WorkspaceResearch } from '../src/plugins/feynman'
import './helpers/enable-feynman'

const mocks = vi.hoisted(() => ({
  onResearchEvent: undefined as ((e: ResearchUpdate) => void) | undefined,
  onFocusEvent: undefined as ((e: ResearchFocusEvent) => void) | undefined,
  disconnect: vi.fn(async () => {}),
  prompt: vi.fn(async () => 'Chat reply'),
  artifacts: vi.fn(async () => []),
  sessionStats: vi.fn(async () => ({
    sessionId: 'sess-1',
    userMessages: 1,
    assistantMessages: 1,
    toolCalls: 0,
    toolResults: 0,
    totalMessages: 2,
    tokens: { input: 100, output: 50, cacheRead: 0, cacheWrite: 0, total: 150 },
    cost: 0.01,
  })),
  cliStatus: vi.fn<() => Promise<CLIStatusResult>>(async () => ({
    command: 'feynman status',
    parsed: true,
    rawOutput: 'Model: openai-codex/gpt-6.1-terra',
    exitCode: 0,
    sections: [
      {
        title: 'Core',
        entries: [
          { key: 'Model', value: 'openai-codex/gpt-6.1-terra' },
          { key: 'alphaXiv', value: 'not configured' },
        ],
      },
    ],
    model: 'openai-codex/gpt-6.1-terra',
    modelValid: 'yes',
    alphaStatus: 'not configured',
    webAccess: 'pi-web-access (Auto)',
    serviceTier: 'not set',
    previewStatus: 'configured',
    piRuntime: 'ready',
  })),
  cliDoctor: vi.fn<() => Promise<CLIDoctorResult>>(async () => ({
    command: 'feynman doctor',
    parsed: true,
    rawOutput: 'pi runtime: ok',
    exitCode: 0,
    entries: [
      { key: 'pi runtime', value: 'ok' },
      { key: 'default model', value: 'openai-codex/gpt-6.1-terra' },
    ],
    issues: [],
    healthy: true,
  })),
  cliSetupPreview: vi.fn<() => Promise<CLICommandResult>>(async () => ({
    command: 'feynman setup preview',
    parsed: true,
    rawOutput: 'Preview ready',
    exitCode: 0,
    summary: 'Preview ready',
  })),
  cliModelList: vi.fn<() => Promise<CLIModelListResult>>(async () => ({
    command: 'feynman model list',
    parsed: true,
    rawOutput: 'openai-codex/gpt-6.1-terra (current)',
    exitCode: 0,
    currentModel: 'openai-codex/gpt-6.1-terra',
    recommendedModel: 'openai-codex/gpt-6.1-terra',
    tier: 'not set',
    models: [
      {
        id: 'gpt-6.1-terra',
        provider: 'openai-codex',
        name: 'openai-codex/gpt-6.1-terra',
        current: true,
        recommended: true,
      },
      {
        id: 'claude-sonnet-4',
        provider: 'anthropic',
        name: 'anthropic/claude-sonnet-4',
        current: false,
        recommended: false,
      },
    ],
  })),
  cliModelAction: vi.fn(async (_root: string, action: string, value: string): Promise<CLICommandResult> => ({
    command: `feynman model ${action} ${value}`.trim(),
    parsed: true,
    rawOutput: `Updated model ${action}: ${value}`,
    exitCode: 0,
    summary: `Updated model ${action}: ${value}`,
  })),
  cliSearchStatus: vi.fn<() => Promise<CLISearchStatusResult>>(async () => ({
    command: 'feynman search status',
    parsed: true,
    rawOutput: 'Search route: Auto',
    exitCode: 0,
    managedBy: 'pi-web-access',
    searchRoute: 'Auto',
    requestRoute: 'auto',
    savedApiKeys: 'none',
    configured: false,
    entries: [{ key: 'Search route', value: 'Auto' }],
  })),
  cliSearchAction: vi.fn(
    async (_root: string, action: string, provider: string): Promise<CLICommandResult> => ({
      command: `feynman search ${action} ${provider}`.trim(),
      parsed: true,
      rawOutput: `Web search ${action} ${provider}`.trim(),
      exitCode: 0,
      summary: `Web search ${action} ${provider}`.trim(),
    }),
  ),
  climcpList: vi.fn<() => Promise<CLIMCPListResult>>(async () => ({
    command: 'feynman mcp list --json',
    parsed: true,
    rawOutput: '{"servers":[]}',
    exitCode: 0,
    servers: [
      {
        name: 'zotero',
        scope: 'project',
        transport: 'stdio',
        command: 'zotero-mcp',
        exposure: 'direct',
        status: 'connected',
        tools: ['search_library'],
      },
    ],
    errors: [],
  })),
  climcpAdd: vi.fn(async (): Promise<CLICommandResult> => ({
    command: 'feynman mcp add',
    parsed: true,
    rawOutput: 'Added MCP server',
    exitCode: 0,
    summary: 'Added MCP server',
  })),
  climcpRemove: vi.fn(async (_root: string, name: string, _local: boolean): Promise<CLICommandResult> => ({
    command: `feynman mcp remove ${name}`,
    parsed: true,
    rawOutput: `Removed ${name}`,
    exitCode: 0,
    summary: `Removed ${name}`,
  })),
  cliPackagesList: vi.fn<() => Promise<CLIPackageListResult>>(async () => ({
    command: 'feynman packages list',
    parsed: true,
    rawOutput: 'pi-subagents',
    exitCode: 0,
    packages: [
      { name: 'pi-subagents', kind: 'core', installed: true },
      { name: 'memory', description: 'Research memory', kind: 'optional', installed: false },
    ],
  })),
  cliPackageAction: vi.fn(async (_root: string, action: string, target: string): Promise<CLICommandResult> => ({
    command: `feynman ${action} ${target}`.trim(),
    parsed: true,
    rawOutput: `${action} ${target}`.trim(),
    exitCode: 0,
    summary: `${action} ${target}`.trim(),
  })),
  cliAlphaStatus: vi.fn<() => Promise<CLIAlphaStatusResult>>(async () => ({
    command: 'feynman alpha status',
    parsed: true,
    rawOutput: 'alphaXiv not logged in',
    exitCode: 0,
    loggedIn: false,
    statusText: 'alphaXiv not logged in',
  })),
  cliAlphaLogout: vi.fn(async (): Promise<CLICommandResult> => ({
    command: 'feynman alpha logout',
    parsed: true,
    rawOutput: 'Logged out',
    exitCode: 0,
    summary: 'Logged out',
  })),
  cliAlphaSearch: vi.fn<() => Promise<CLIAlphaSearchResult>>(async () => ({
    command: 'feynman alpha search --mode semantic attention',
    parsed: true,
    rawOutput: 'Sparse Attention at Scale',
    exitCode: 0,
    query: 'attention',
    mode: 'semantic',
    papers: [
      {
        id: '2501.01234',
        title: 'Sparse Attention at Scale',
        authors: 'A. Researcher',
        url: 'https://alphaxiv.org/abs/2501.01234',
        summary: 'Sub-quadratic routing.',
      },
    ],
  })),
  cliAlphaGet: vi.fn<() => Promise<CLIAlphaPaperResult>>(async () => ({
    command: 'feynman alpha get 2501.01234',
    parsed: true,
    rawOutput: '# Sparse Attention at Scale\nDetails.',
    exitCode: 0,
    paperId: '2501.01234',
    title: 'Sparse Attention at Scale',
    content: '# Sparse Attention at Scale\nDetails.',
    annotation: 'Check theorem 2',
    fullText: false,
  })),
  cliAlphaAsk: vi.fn(async (): Promise<CLICommandResult> => ({
    command: 'feynman alpha ask 2501.01234',
    parsed: true,
    rawOutput: 'O(n log n) complexity.',
    exitCode: 0,
    summary: 'O(n log n) complexity.',
  })),
  cliAlphaCode: vi.fn(async (): Promise<CLICommandResult> => ({
    command: 'feynman alpha code',
    parsed: true,
    rawOutput: 'def sparse_attention(): pass',
    exitCode: 0,
    summary: 'def sparse_attention(): pass',
  })),
  cliAlphaAnnotate: vi.fn<() => Promise<CLIAlphaAnnotateResult>>(async () => ({
    command: 'feynman alpha annotate --list',
    parsed: true,
    rawOutput: '2501.01234: Check theorem 2',
    exitCode: 0,
    annotations: [{ paperId: '2501.01234', note: 'Check theorem 2' }],
  })),
  launchTerminalWizard: vi.fn(
    async (_root: string, wizard: string, target: string): Promise<CLITerminalLaunchResult> => ({
      wizard,
      command:
        wizard === 'model_login'
          ? target
            ? `feynman model login ${target}`
            : 'feynman model login'
          : wizard === 'alpha_login'
            ? 'feynman alpha login'
            : 'feynman setup',
      launched: true,
    }),
  ),
  watchFocus: vi.fn(async () => {}),
  refreshModels: vi.fn<() => Promise<ResearchModel[]>>(async () => [
    { id: 'gpt-6.1-terra', provider: 'openai-codex', name: 'GPT-6.1 Terra' },
    { id: 'claude-sonnet-4', provider: 'anthropic', name: 'Claude Sonnet 4' },
  ]),
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
      researchFocus: {
        on(fn: (e: ResearchFocusEvent) => void) {
          mocks.onFocusEvent = fn
          return () => {
            mocks.onFocusEvent = undefined
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
          models: [{ id: 'gpt-6.1-terra', provider: 'openai-codex', name: 'GPT-6.1 Terra' }],
          thinkingLevels: ['low', 'medium'],
          thinkingLevel: 'medium',
          state: {
            model: { id: 'gpt-6.1-terra', provider: 'openai-codex', name: 'GPT-6.1 Terra' },
            thinkingLevel: 'medium',
            isStreaming: false,
            isCompacting: false,
            sessionId: 'sess-1',
            autoCompactionEnabled: true,
            messageCount: 0,
            pendingMessageCount: 0,
          },
        })
        mocks.onResearchEvent?.({ root, kind: 'commands', commands: [] })
      },
      disconnect: mocks.disconnect,
      prompt: mocks.prompt,
      artifacts: mocks.artifacts,
      sessionStats: mocks.sessionStats,
      cliStatus: mocks.cliStatus,
      cliDoctor: mocks.cliDoctor,
      cliSetupPreview: mocks.cliSetupPreview,
      cliModelList: mocks.cliModelList,
      cliModelAction: mocks.cliModelAction,
      cliSearchStatus: mocks.cliSearchStatus,
      cliSearchAction: mocks.cliSearchAction,
      climcpList: mocks.climcpList,
      climcpAdd: mocks.climcpAdd,
      climcpRemove: mocks.climcpRemove,
      cliPackagesList: mocks.cliPackagesList,
      cliPackageAction: mocks.cliPackageAction,
      cliAlphaStatus: mocks.cliAlphaStatus,
      cliAlphaLogout: mocks.cliAlphaLogout,
      cliAlphaSearch: mocks.cliAlphaSearch,
      cliAlphaGet: mocks.cliAlphaGet,
      cliAlphaAsk: mocks.cliAlphaAsk,
      cliAlphaCode: mocks.cliAlphaCode,
      cliAlphaAnnotate: mocks.cliAlphaAnnotate,
      launchTerminalWizard: mocks.launchTerminalWizard,
      watchFocus: mocks.watchFocus,
      refreshModels: mocks.refreshModels,
    },
  }
})

describe('Feynman Settings & alphaXiv Portrait UI', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
  })

  it('renders all 6 English Portrait settings sections without native select/meter and keeps Chat live', async () => {
    const container = document.body.appendChild(document.createElement('div'))
    container.style.height = '760px'
    container.style.width = '420px'
    const root = createRoot(container)
    const openDoc = vi.fn(async (path: string) => ({
      id: 'doc-1',
      snapshot: async () => ({ path, text: '# Paper', version: 1 }),
    }))
    try {
      root.render(
        <WorkspaceResearch root="/ws-settings" documentID="doc-1" open={openDoc} openFolder={() => {}} />,
      )
      await expect.element(page.getByRole('radio', { name: /Chat/ })).toBeVisible()
      await expect.element(page.getByTestId('feynman-settings-toggle')).toBeVisible()

      await userEvent.click(page.getByTestId('feynman-settings-toggle'))

      await expect.element(page.getByRole('heading', { name: 'Feynman Settings', level: 3 })).toBeVisible()
      await expect.element(page.getByText(/links existing Feynman files/)).toBeVisible()
      const transport = container.querySelector<HTMLElement>('[role="radiogroup"][aria-label="MCP transport"] [aria-checked="true"]')!
      transport.focus()
      await userEvent.keyboard('{ArrowRight}')
      await expect.element(page.getByRole('radio', { name: 'STDIO' })).toHaveFocus()
      await userEvent.keyboard('{ArrowLeft}')
      await expect.element(page.getByRole('radio', { name: 'HTTP' })).toHaveFocus()
      for (const heading of [
        'Status & Doctor',
        'Models & Tier',
        'alphaXiv',
        'Web Search',
        'MCP Servers',
        'Packages & Updates',
      ]) {
        await expect.element(page.getByRole('heading', { name: heading, level: 4 })).toBeVisible()
      }

      expect(container.querySelectorAll('select, meter').length).toBe(0)

      await userEvent.fill(page.getByLabelText('Web search provider'), 'exa')
      await userEvent.fill(page.getByLabelText('Web search API key'), 'exa-test-key')
      await userEvent.click(page.getByRole('button', { name: 'Save Web Search' }))
      expect(mocks.cliSearchAction).toHaveBeenCalledWith('/ws-settings', 'set', 'exa', 'exa-test-key')

      await userEvent.fill(page.getByLabelText('MCP server name'), 'papers-mcp')
      await userEvent.fill(page.getByLabelText('MCP server URL'), 'http://127.0.0.1:9001/mcp')
      await userEvent.click(page.getByRole('button', { name: 'Add MCP Server' }))
      expect(mocks.climcpAdd).toHaveBeenCalledWith(
        '/ws-settings',
        expect.objectContaining({
          name: 'papers-mcp',
          transport: 'http',
          url: 'http://127.0.0.1:9001/mcp',
        }),
      )

      await userEvent.click(page.getByRole('button', { name: 'Remove zotero (project)' }))
      expect(mocks.climcpRemove).toHaveBeenCalledWith('/ws-settings', 'zotero', true)
      await userEvent.click(page.getByRole('button', { name: 'Set default' }))
      expect(mocks.cliModelAction).toHaveBeenCalledWith('/ws-settings', 'set', 'anthropic/claude-sonnet-4')

      await userEvent.fill(page.getByLabelText('Custom service tier'), 'priority')
      await userEvent.click(page.getByRole('button', { name: 'Apply tier' }))
      expect(mocks.cliModelAction).toHaveBeenCalledWith('/ws-settings', 'tier', 'priority')

      await userEvent.click(page.getByRole('button', { name: 'Install' }))
      expect(mocks.cliPackageAction).toHaveBeenCalledWith('/ws-settings', 'install', 'memory')

      await userEvent.click(page.getByRole('button', { name: 'Update All' }))
      expect(mocks.cliPackageAction).toHaveBeenCalledWith('/ws-settings', 'update', '')

      await userEvent.click(page.getByRole('button', { name: 'Verify Preview' }))
      expect(mocks.cliSetupPreview).toHaveBeenCalledWith('/ws-settings')

      expect(mocks.disconnect).not.toHaveBeenCalled()

      await userEvent.click(page.getByRole('button', { name: 'Back to Chat' }))
      await expect.element(page.getByLabelText('Connected')).toBeVisible()
    } finally {
      root.unmount()
      container.remove()
    }
  })

  it('clears a saved search key separately from resetting the search route', async () => {
    const placeholders = new Set<string>()
    let requestRoute = 'auto'
    const status = mocks.cliSearchStatus.getMockImplementation()!
    const action = mocks.cliSearchAction.getMockImplementation()!
    mocks.cliSearchStatus.mockImplementation(async () => {
      const savedApiKeys = [...placeholders].join(', ') || 'none'
      return {
        ...await status(), requestRoute, savedApiKeys,
        entries: [{ key: 'Saved API keys', value: savedApiKeys }],
      }
    })
    mocks.cliSearchAction.mockImplementation(async (root, kind, provider) => {
      if (kind === 'set') { placeholders.add(provider); requestRoute = provider }
      if (kind === 'clear') requestRoute = 'auto'
      if (kind === 'clear-key') placeholders.delete(provider)
      return action(root, kind, provider)
    })
    const container = document.body.appendChild(document.createElement('div'))
    const root = createRoot(container)
    try {
      root.render(<WorkspaceResearch root="/ws-search-key" documentID="doc-1" open={vi.fn()} openFolder={() => {}} />)
      await userEvent.click(page.getByTestId('feynman-settings-toggle'))
      const search = page.getByRole('region', { name: 'Web Search' })
      await userEvent.fill(search.getByLabelText('Web search provider'), 'exa')
      await userEvent.fill(search.getByLabelText('Web search API key'), 'fake-exa-key')
      await userEvent.click(search.getByRole('button', { name: 'Save Web Search' }))
      await expect.element(search.getByText('exa', { exact: true }).first()).toBeVisible()
      await userEvent.click(search.getByRole('button', { name: 'Reset to Auto' }))
      await expect.element(search.getByLabelText('Web search provider')).toHaveValue('auto')
      await expect.element(search.getByText('exa', { exact: true }).first()).toBeVisible()
      await userEvent.click(search.getByRole('button', { name: 'Clear exa API key' }))
      expect(mocks.cliSearchAction).toHaveBeenCalledWith('/ws-search-key', 'clear-key', 'exa', '')
      await expect.element(search.getByText('none', { exact: true })).toBeVisible()
      await expect.element(search.getByRole('button', { name: 'Clear exa API key' })).not.toBeInTheDocument()
    } finally {
      mocks.cliSearchStatus.mockImplementation(status)
      mocks.cliSearchAction.mockImplementation(action)
      root.unmount()
      container.remove()
    }
  })

  it('degrades gracefully to raw CLI output when parsed is false', async () => {
    mocks.cliStatus.mockResolvedValueOnce({
      command: 'feynman status',
      parsed: false,
      rawOutput: 'RAW UNRECOGNIZED STATUS OUTPUT',
      exitCode: 0,
      sections: [],
    })
    const container = document.body.appendChild(document.createElement('div'))
    container.style.height = '760px'
    const root = createRoot(container)
    const openDoc = vi.fn(async (path: string) => ({
      id: 'doc-1',
      snapshot: async () => ({ path, text: '', version: 1 }),
    }))
    try {
      root.render(
        <WorkspaceResearch root="/ws-raw" documentID="doc-1" open={openDoc} openFolder={() => {}} />,
      )
      await userEvent.click(page.getByTestId('feynman-settings-toggle'))
      await expect.element(page.getByText('RAW UNRECOGNIZED STATUS OUTPUT')).toBeVisible()
      expect(container.querySelector('[data-testid="cli-raw-fallback"]')).not.toBeNull()
    } finally {
      root.unmount()
      container.remove()
    }
  })

  it('supports Terminal wizard handoff and focus-triggered refresh of status and models', async () => {
    const container = document.body.appendChild(document.createElement('div'))
    container.style.height = '760px'
    const root = createRoot(container)
    const openDoc = vi.fn(async (path: string) => ({
      id: 'doc-1',
      snapshot: async () => ({ path, text: '', version: 1 }),
    }))
    try {
      root.render(
        <WorkspaceResearch root="/ws-focus" documentID="doc-1" open={openDoc} openFolder={() => {}} />,
      )
      await userEvent.click(page.getByTestId('feynman-settings-toggle'))
      await expect.poll(() => mocks.cliStatus.mock.calls.length).toBeGreaterThanOrEqual(1)

      await userEvent.click(page.getByRole('button', { name: 'Sign in in Terminal (Models)' }))
      expect(mocks.launchTerminalWizard).toHaveBeenCalledWith('/ws-focus', 'model_login', '')

      await userEvent.click(page.getByRole('button', { name: 'Sign in to alphaXiv in Terminal' }))
      expect(mocks.launchTerminalWizard).toHaveBeenCalledWith('/ws-focus', 'alpha_login', '')

      await userEvent.click(page.getByRole('button', { name: 'Run Setup in Terminal' }))
      expect(mocks.launchTerminalWizard).toHaveBeenCalledWith('/ws-focus', 'setup', '')

      const statusCallsBefore = mocks.cliStatus.mock.calls.length
      const modelCallsBefore = mocks.cliModelList.mock.calls.length

      mocks.onFocusEvent?.({ root: '/ws-focus' })
      await expect.poll(() => mocks.cliStatus.mock.calls.length).toBeGreaterThan(statusCallsBefore)
      await expect.poll(() => mocks.cliModelList.mock.calls.length).toBeGreaterThan(modelCallsBefore)
      expect(mocks.refreshModels).toHaveBeenCalledWith('/ws-focus')

      const afterMyGoFocus = mocks.cliStatus.mock.calls.length
      window.dispatchEvent(new Event('focus'))
      await expect.poll(() => mocks.cliStatus.mock.calls.length).toBeGreaterThan(afterMyGoFocus)
    } finally {
      root.unmount()
      container.remove()
    }
  })

  it('provides alphaXiv search, inspect, ask, code, and annotate from both Settings and Chat', async () => {
    const container = document.body.appendChild(document.createElement('div'))
    container.style.height = '760px'
    const root = createRoot(container)
    const openDoc = vi.fn(async (path: string) => ({
      id: 'doc-1',
      snapshot: async () => ({ path, text: '', version: 1 }),
    }))
    try {
      root.render(
        <WorkspaceResearch root="/ws-alpha" documentID="doc-1" open={openDoc} openFolder={() => {}} />,
      )
      await expect.element(page.getByTestId('fy-alphaxiv-trigger')).toBeVisible()

      await userEvent.click(page.getByTestId('fy-alphaxiv-trigger'))
      await expect.element(page.getByRole('searchbox', { name: 'alphaXiv search query' })).toHaveFocus()
      await userEvent.click(page.getByRole('button', { name: 'Back to Chat' }))
      await expect.element(page.getByRole('radio', { name: /Chat/ })).toHaveFocus()
      await userEvent.click(page.getByTestId('fy-alphaxiv-trigger'))
      await userEvent.fill(page.getByLabelText('alphaXiv search query'), 'sparse attention')
      await userEvent.click(page.getByRole('button', { name: 'Search alphaXiv' }))
      expect(mocks.cliAlphaSearch).toHaveBeenCalledWith('/ws-alpha', 'sparse attention', 'semantic')
      await expect.element(page.getByText('Sparse Attention at Scale')).toBeVisible()

      await userEvent.click(page.getByRole('button', { name: 'Inspect paper' }))
      expect(mocks.cliAlphaGet).toHaveBeenCalledWith('/ws-alpha', '2501.01234', false)

      mocks.cliAlphaSearch.mockResolvedValueOnce({
        command: 'feynman alpha search --mode both',
        parsed: true,
        rawOutput: '{}',
        exitCode: 0,
        query: 'sparse attention',
        mode: 'both',
        papers: [],
        groups: [
          { mode: 'semantic', papers: [{ id: '2501.01234', title: 'Semantic match' }] },
          { mode: 'keyword', papers: [{ id: '2502.01234', title: 'Keyword match' }] },
        ],
      })
      await userEvent.click(page.getByRole('button', { name: 'both', exact: true }))
      await userEvent.click(page.getByRole('button', { name: 'Search alphaXiv' }))
      await expect.element(page.getByText('Semantic match')).toBeVisible()
      await expect.element(page.getByText('Keyword match')).toBeVisible()

      await userEvent.fill(page.getByLabelText('Ask about paper'), 'What is the complexity?')
      await userEvent.click(page.getByRole('button', { name: 'Ask paper' }))
      expect(mocks.cliAlphaAsk).toHaveBeenCalledWith('/ws-alpha', '2501.01234', 'What is the complexity?')
      await expect.element(page.getByText('O(n log n) complexity.')).toBeVisible()

      await userEvent.fill(page.getByLabelText('GitHub repository URL'), 'https://github.com/example/sparse-attn')
      await userEvent.click(page.getByRole('button', { name: 'Inspect code' }))
      expect(mocks.cliAlphaCode).toHaveBeenCalledWith('/ws-alpha', 'https://github.com/example/sparse-attn', '')

      await userEvent.fill(page.getByLabelText('Paper annotation'), 'Verify theorem 2')
      await userEvent.click(page.getByRole('button', { name: 'Save note' }))
      expect(mocks.cliAlphaAnnotate).toHaveBeenCalledWith('/ws-alpha', 'set', '2501.01234', 'Verify theorem 2')

      await userEvent.click(page.getByRole('button', { name: 'Insert to Chat' }).first())
      const textarea = container.querySelector<HTMLTextAreaElement>('[aria-label="Research prompt"]')!
      expect(textarea.value).toContain('2501.01234')

      let resolvePrompt!: (v: string) => void
      mocks.prompt.mockImplementationOnce(
        () =>
          new Promise<string>((resolve) => {
            resolvePrompt = resolve
          }),
      )
      await userEvent.fill(page.getByLabelText('Research prompt'), 'Analyze theorem 2')
      await userEvent.click(page.getByRole('button', { name: 'Send' }))
      await expect.element(page.getByLabelText('Working')).toBeVisible()

      await userEvent.click(page.getByTestId('feynman-settings-toggle'))
      await userEvent.click(page.getByRole('button', { name: 'Run Doctor' }))
      expect(mocks.cliDoctor).toHaveBeenCalledWith('/ws-alpha')
      expect(mocks.disconnect).not.toHaveBeenCalled()

      await userEvent.click(page.getByRole('button', { name: 'Back to Chat' }))
      await expect.element(page.getByLabelText('Working')).toBeVisible()
      const refreshCalls = mocks.refreshModels.mock.calls.length
      const statusCalls = mocks.cliStatus.mock.calls.length
      mocks.cliStatus.mockResolvedValueOnce({
        command: 'feynman status',
        parsed: false,
        rawOutput: 'REFRESHED STATUS WHILE SETTINGS WAS CLOSED',
        exitCode: 0,
        sections: [],
      })
      mocks.onFocusEvent?.({ root: '/ws-alpha' })
      await expect.poll(() => mocks.refreshModels.mock.calls.length).toBeGreaterThan(refreshCalls)
      await expect.poll(() => mocks.cliStatus.mock.calls.length).toBeGreaterThan(statusCalls)
      resolvePrompt('Done')
      await expect.element(page.getByLabelText('Connected')).toBeVisible()
      mocks.cliStatus.mockImplementationOnce(() => new Promise(() => {}))
      await userEvent.click(page.getByTestId('feynman-settings-toggle'))
      await expect.element(page.getByText('REFRESHED STATUS WHILE SETTINGS WAS CLOSED')).toBeVisible()
    } finally {
      root.unmount()
      container.remove()
    }
  })
})
