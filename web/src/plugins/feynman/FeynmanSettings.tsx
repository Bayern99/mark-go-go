import { useCallback, useEffect, useRef, useState } from 'react'
import { Popover } from 'radix-ui'
import { ScrollArea } from '../../components/ScrollArea'
import { radioKeys } from '../../components/radioKeys'
import {
  events,
  ResearchHost,
  type CLIAlphaStatusResult,
  type CLICommandResult,
  type CLIDoctorResult,
  type CLIMCPListResult,
  type CLIMCPServer,
  type CLIModelListResult,
  type CLIPackageListResult,
  type CLISearchStatusResult,
} from '../../host/generated/mygo'
import { AlphaXivPanel, CLIRawFallback } from './AlphaXivPanel'
import { ResearchIcon } from './Conversation'
import type { FeynmanSettingsSlotProps } from './index'

const TIER_PRESETS = ['default', 'auto', 'flex', 'priority']
const SEARCH_PROVIDERS = [
  'auto',
  'all',
  'exa',
  'brave',
  'tavily',
  'perplexity',
  'openai',
  'gemini',
  'kagi',
  'duckduckgo',
  'searxng',
  'firecrawl',
  'jina',
]
const MCP_EXPOSURES = ['codemode', 'deferred', 'direct', 'hidden']

function OptionPopover({
  label,
  value,
  options,
  onSelect,
}: {
  label: string
  value: string
  options: string[]
  onSelect: (next: string) => void
}) {
  const [open, setOpen] = useState(false)
  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>
        <button type="button" className="fy-config fy-cli-picker" aria-label={label}>
          <span>{value || label}</span>
          <ResearchIcon name="caret-down" />
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content className="fy-popover" side="bottom" align="start" sideOffset={5} collisionPadding={8}>
          <div className="fy-options" role="group" aria-label={label}>
            {options.map((opt) => (
              <button
                key={opt}
                type="button"
                aria-pressed={opt === value}
                onClick={() => {
                  setOpen(false)
                  onSelect(opt)
                }}
              >
                <span>{opt}</span>
                {opt === value && <ResearchIcon name="check" />}
              </button>
            ))}
          </div>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}

export function FeynmanSettings({ root, session, cliStatus: statusRes, refreshCLIStatus, onClose, onInsertToChat }: FeynmanSettingsSlotProps) {
  const [doctorRes, setDoctorRes] = useState<CLIDoctorResult | null>(null)
  const [previewRes, setPreviewRes] = useState<CLICommandResult | null>(null)
  const [modelsRes, setModelsRes] = useState<CLIModelListResult | null>(null)
  const [alphaStatus, setAlphaStatus] = useState<CLIAlphaStatusResult | null>(null)
  const [searchStatus, setSearchStatus] = useState<CLISearchStatusResult | null>(null)
  const [mcpList, setMCPList] = useState<CLIMCPListResult | null>(null)
  const [packagesRes, setPackagesRes] = useState<CLIPackageListResult | null>(null)

  const [loginProvider, setLoginProvider] = useState('')
  const [tierInput, setTierInput] = useState('')
  const [searchProvider, setSearchProvider] = useState('auto')
  const [searchApiKey, setSearchApiKey] = useState('')

  const [mcpName, setMCPName] = useState('')
  const [mcpTransport, setMCPTransport] = useState<'http' | 'stdio'>('http')
  const [mcpUrl, setMCPUrl] = useState('')
  const [mcpHeaders, setMCPHeaders] = useState('')
  const [mcpBearerEnv, setMCPBearerEnv] = useState('')
  const [mcpCommand, setMCPCommand] = useState('')
  const [mcpArgs, setMCPArgs] = useState('')
  const [mcpEnv, setMCPEnv] = useState('')
  const [mcpCwd, setMCPCwd] = useState('')
  const [mcpExposure, setMCPExposure] = useState('direct')
  const [mcpDesc, setMCPDesc] = useState('')
  const [mcpLocal, setMCPLocal] = useState(false)

  const [loading, setLoading] = useState(false)
  const [busyOp, setBusyOp] = useState('')
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const refreshingFocus = useRef(false)

  const loadCore = useCallback(async () => {
    const [, models, alpha] = await Promise.all([
      refreshCLIStatus(),
      ResearchHost.cliModelList?.(root),
      ResearchHost.cliAlphaStatus?.(root),
    ])
    if (models) {
      setModelsRes(models)
      if (models.tier && models.tier !== 'not set') setTierInput(models.tier)
    }
    if (alpha) setAlphaStatus(alpha)
  }, [refreshCLIStatus, root])

  const loadAll = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const [, doc, search, mcp, pkgs] = await Promise.all([
        loadCore(),
        ResearchHost.cliDoctor?.(root),
        ResearchHost.cliSearchStatus?.(root),
        ResearchHost.climcpList?.(root),
        ResearchHost.cliPackagesList?.(root),
      ])
      if (doc) setDoctorRes(doc)
      if (search) {
        setSearchStatus(search)
        if (search.requestRoute) setSearchProvider(search.requestRoute)
      }
      if (mcp) setMCPList(mcp)
      if (pkgs) setPackagesRes(pkgs)
    } catch (err) {
      setError(String(err))
    } finally {
      setLoading(false)
    }
  }, [loadCore, root])

  useEffect(() => {
    if (!root) return
    void loadAll()
    void ResearchHost.watchFocus?.(root)?.catch?.(() => {})
  }, [loadAll, root])

  const refreshOnFocus = useCallback(async () => {
    if (!root || refreshingFocus.current) return
    refreshingFocus.current = true
    try {
      await loadCore()
      if (!session || session.status === 'ready' || session.status === 'working') {
        await ResearchHost.refreshModels?.(root)?.catch?.(() => {})
      }
    } catch {
      // Best-effort refresh on focus regain.
    } finally {
      refreshingFocus.current = false
    }
  }, [loadCore, root, session])

  useEffect(() => {
    if (!root) return
    const onWindowFocus = () => {
      void refreshOnFocus()
    }
    window.addEventListener('focus', onWindowFocus)
    let offFocusEvent: (() => void) | undefined
    try {
      offFocusEvent = events.researchFocus?.on((ev) => {
        if (ev.root === root) void refreshOnFocus()
      })
    } catch {
      // Standalone test mocks may omit researchFocus event.
    }
    return () => {
      window.removeEventListener('focus', onWindowFocus)
      offFocusEvent?.()
    }
  }, [refreshOnFocus, root])

  async function runOp(name: string, fn: () => Promise<void>) {
    setBusyOp(name)
    setError('')
    setNotice('')
    try {
      await fn()
    } catch (err) {
      setError(String(err))
    } finally {
      setBusyOp('')
    }
  }

  async function launchWizard(wizard: 'model_login' | 'alpha_login' | 'setup', target = '') {
    await runOp(`wizard-${wizard}`, async () => {
      const res = await ResearchHost.launchTerminalWizard(root, wizard, target)
      setNotice(`Launched "${res.command}" in Terminal. Status and models refresh automatically on window focus.`)
    })
  }

  async function handleModelAction(action: 'set' | 'tier' | 'logout', value: string) {
    await runOp(`model-${action}`, async () => {
      const res = await ResearchHost.cliModelAction(root, action, value)
      setNotice(res.summary || res.rawOutput)
      await loadCore()
      if (session?.status === 'ready') {
        await ResearchHost.refreshModels?.(root)?.catch?.(() => {})
      }
    })
  }

  async function handleSaveSearch(e: React.FormEvent) {
    e.preventDefault()
    if (!searchProvider.trim()) return
    await runOp('search-set', async () => {
      const res = await ResearchHost.cliSearchAction(root, 'set', searchProvider.trim(), searchApiKey.trim())
      setNotice(res.summary || res.rawOutput)
      setSearchApiKey('')
      const next = await ResearchHost.cliSearchStatus(root)
      setSearchStatus(next)
    })
  }

  async function handleClearSearch() {
    await runOp('search-clear', async () => {
      const res = await ResearchHost.cliSearchAction(root, 'clear', '', '')
      setNotice(res.summary || res.rawOutput)
      const next = await ResearchHost.cliSearchStatus(root)
      setSearchStatus(next)
      if (next.requestRoute) setSearchProvider(next.requestRoute)
    })
  }

  async function handleAddMCP(e: React.FormEvent) {
    e.preventDefault()
    if (!mcpName.trim()) return
    await runOp('mcp-add', async () => {
      const splitList = (val: string) =>
        val
          .split(/[\n,]/)
          .map((item) => item.trim())
          .filter(Boolean)
      const res = await ResearchHost.climcpAdd(root, {
        name: mcpName.trim(),
        transport: mcpTransport,
        url: mcpTransport === 'http' ? mcpUrl.trim() : undefined,
        headers: mcpTransport === 'http' ? splitList(mcpHeaders) : undefined,
        bearerTokenEnvVar: mcpTransport === 'http' ? mcpBearerEnv.trim() : undefined,
        command: mcpTransport === 'stdio' ? mcpCommand.trim() : undefined,
        args: mcpTransport === 'stdio' ? mcpArgs.trim().split(/\s+/).filter(Boolean) : undefined,
        env: mcpTransport === 'stdio' ? splitList(mcpEnv) : undefined,
        cwd: mcpTransport === 'stdio' ? mcpCwd.trim() : undefined,
        exposure: mcpExposure,
        description: mcpDesc.trim() || undefined,
        local: mcpLocal,
      })
      setNotice(res.summary || res.rawOutput)
      setMCPName('')
      setMCPUrl('')
      setMCPHeaders('')
      setMCPBearerEnv('')
      setMCPCommand('')
      setMCPArgs('')
      setMCPEnv('')
      setMCPCwd('')
      setMCPDesc('')
      setMCPList(await ResearchHost.climcpList(root))
    })
  }

  async function handleRemoveMCP(server: CLIMCPServer) {
    await runOp(`mcp-remove-${server.name}`, async () => {
      const res = await ResearchHost.climcpRemove(root, server.name, server.scope === 'project')
      setNotice(res.summary || res.rawOutput)
      setMCPList(await ResearchHost.climcpList(root))
    })
  }

  async function handlePackageAction(action: 'install' | 'remove' | 'update', target: string) {
    await runOp(`pkg-${action}-${target}`, async () => {
      const res = await ResearchHost.cliPackageAction(root, action, target)
      setNotice(res.summary || res.rawOutput)
      setPackagesRes(await ResearchHost.cliPackagesList(root))
    })
  }

  return (
    <div className="fy-settings-view" data-testid="feynman-settings-view">
      <div className="fy-settings-topbar">
        <button type="button" className="pill quiet" onClick={onClose}>
          <ResearchIcon name="arrow-left" />
          Back to Chat
        </button>
        <h3>Feynman Settings</h3>
        <button
          type="button"
          className="fy-icon"
          aria-label="Refresh settings"
          title="Refresh settings"
          disabled={loading}
          onClick={() => void loadAll()}
        >
          <ResearchIcon name={loading ? 'spinner-gap' : 'arrow-clockwise'} className={loading ? 'fy-spin' : undefined} />
        </button>
      </div>

      {notice && (
        <p className="fy-notice" role="status">
          <ResearchIcon name="info" />
          <span>{notice}</span>
        </p>
      )}
      {error && (
        <p className="fy-notice" data-tone="error" role="alert">
          <ResearchIcon name="warning-circle" />
          <span>{error}</span>
        </p>
      )}

      <ScrollArea className="fy-settings-scroll" contentClassName="fy-settings-content">
        <section className="fy-settings-card" aria-labelledby="fy-sec-status">
          <div className="fy-settings-card-head">
            <h4 id="fy-sec-status">Status &amp; Doctor</h4>
            <div className="fy-cli-actions">
              <button
                type="button"
                className="pill xs"
                disabled={busyOp !== ''}
                onClick={() =>
                  void runOp('doctor', async () => {
                    const [, doc] = await Promise.all([
                      refreshCLIStatus(),
                      ResearchHost.cliDoctor(root),
                    ])
                    setDoctorRes(doc)
                  })
                }
              >
                {busyOp === 'doctor' && <ResearchIcon name="spinner-gap" className="fy-spin" />}
                Run Doctor
              </button>
              <button
                type="button"
                className="pill xs quiet"
                disabled={busyOp !== ''}
                onClick={() =>
                  void runOp('setup-preview', async () => {
                    setPreviewRes(await ResearchHost.cliSetupPreview(root))
                  })
                }
              >
                Verify Preview
              </button>
              <button
                type="button"
                className="pill xs"
                disabled={busyOp !== ''}
                onClick={() => void launchWizard('setup')}
              >
                Run Setup in Terminal
              </button>
            </div>
          </div>

          {statusRes && (
            statusRes.parsed ? (
              <div className="fy-cli-sections">
                {statusRes.sections.map((sec) => (
                  <div key={sec.title} className="fy-cli-subcard">
                    <div className="fy-cli-subhead">
                      <span>{sec.title}</span>
                    </div>
                    <ul className="fy-cli-kv-list">
                      {sec.entries.map((entry) => (
                        <li key={`${sec.title}-${entry.key}`}>
                          <span className="fy-cli-kv-key">{entry.key}</span>
                          <span className="fy-cli-kv-val">{entry.value}</span>
                        </li>
                      ))}
                    </ul>
                  </div>
                ))}
              </div>
            ) : (
              <CLIRawFallback
                command={statusRes.command}
                rawOutput={statusRes.rawOutput}
                error={statusRes.error}
              />
            )
          )}

          {doctorRes && (
            doctorRes.parsed ? (
              <div className="fy-cli-subcard">
                <div className="fy-cli-subhead">
                  <span>Doctor diagnostics</span>
                  <span className={`fy-cli-pill${doctorRes.healthy ? ' ok' : ' warn'}`}>
                    {doctorRes.healthy ? 'Healthy' : `${doctorRes.issues.length} issue(s)`}
                  </span>
                </div>
                <ul className="fy-cli-kv-list">
                  {doctorRes.entries.slice(0, 12).map((entry) => (
                    <li key={entry.key}>
                      <span className="fy-cli-kv-key">{entry.key}</span>
                      <span className="fy-cli-kv-val">{entry.value}</span>
                    </li>
                  ))}
                </ul>
              </div>
            ) : (
              <CLIRawFallback
                command={doctorRes.command}
                rawOutput={doctorRes.rawOutput}
                error={doctorRes.error}
              />
            )
          )}

          {previewRes && (
            <CLIRawFallback
              command={previewRes.command}
              rawOutput={previewRes.rawOutput}
              error={previewRes.error}
            />
          )}
        </section>

        <section className="fy-settings-card" aria-labelledby="fy-sec-models">
          <div className="fy-settings-card-head">
            <h4 id="fy-sec-models">Models &amp; Tier</h4>
            <button
              type="button"
              className="pill xs"
              disabled={busyOp !== ''}
              onClick={() => void launchWizard('model_login', loginProvider)}
            >
              Sign in in Terminal (Models)
            </button>
          </div>
          <p className="meta">Start setup and sign-in here. Mark Go Go uses a separate Feynman profile and does not import your existing CLI login.</p>

          <div className="fy-cli-row">
            <input
              type="text"
              className="fy-cli-input"
              aria-label="Model provider for Terminal login"
              placeholder="Optional provider (e.g. anthropic, openai)"
              value={loginProvider}
              onChange={(e) => setLoginProvider(e.target.value)}
            />
          </div>

          <div className="fy-cli-subcard">
            <div className="fy-cli-subhead">
              <span>Service tier</span>
              {modelsRes?.tier && <code className="meta">{modelsRes.tier}</code>}
            </div>
            <div className="fy-cli-row">
              <OptionPopover
                label="Service tier"
                value={tierInput || 'default'}
                options={TIER_PRESETS}
                onSelect={(val) => {
                  setTierInput(val)
                  void handleModelAction('tier', val)
                }}
              />
              <input
                type="text"
                className="fy-cli-input"
                aria-label="Custom service tier"
                placeholder="Tier value"
                value={tierInput}
                onChange={(e) => setTierInput(e.target.value)}
              />
              <button
                type="button"
                className="pill xs"
                disabled={!tierInput.trim() || busyOp !== ''}
                onClick={() => void handleModelAction('tier', tierInput)}
              >
                Apply tier
              </button>
            </div>
          </div>

          {modelsRes && (
            modelsRes.parsed ? (
              modelsRes.models.length === 0 ? (
                <p className="fy-muted">No authenticated models found. Use Sign in in Terminal.</p>
              ) : (
                <div className="fy-cli-list">
                  {modelsRes.models.map((m) => (
                    <div key={m.name} className="fy-cli-item">
                      <div className="fy-cli-item-copy">
                        <strong>{m.name}</strong>
                        <small>
                          {[m.provider, m.current ? 'current' : '', m.recommended ? 'recommended' : '']
                            .filter(Boolean)
                            .join(' · ')}
                        </small>
                      </div>
                      <div className="fy-cli-actions">
                        <button
                          type="button"
                          className={`pill xs${m.current ? ' primary' : ''}`}
                          disabled={m.current || busyOp !== ''}
                          onClick={() => void handleModelAction('set', m.name)}
                        >
                          {m.current ? 'Default' : 'Set default'}
                        </button>
                        <button
                          type="button"
                          className="pill xs quiet"
                          disabled={busyOp !== ''}
                          onClick={() => void handleModelAction('logout', m.provider)}
                        >
                          Log out
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )
            ) : (
              <CLIRawFallback
                command={modelsRes.command}
                rawOutput={modelsRes.rawOutput}
                error={modelsRes.error}
              />
            )
          )}
        </section>

        <section className="fy-settings-card" aria-labelledby="fy-sec-alpha">
          <div className="fy-settings-card-head">
            <h4 id="fy-sec-alpha">alphaXiv</h4>
            <div className="fy-cli-actions">
              <button
                type="button"
                className="pill xs"
                disabled={busyOp !== ''}
                onClick={() => void launchWizard('alpha_login')}
              >
                Sign in to alphaXiv in Terminal
              </button>
              {alphaStatus?.loggedIn && (
                <button
                  type="button"
                  className="pill xs quiet"
                  disabled={busyOp !== ''}
                  onClick={() =>
                    void runOp('alpha-logout', async () => {
                      const res = await ResearchHost.cliAlphaLogout(root)
                      setNotice(res.summary || res.rawOutput)
                      setAlphaStatus(await ResearchHost.cliAlphaStatus(root))
                    })
                  }
                >
                  Log out
                </button>
              )}
            </div>
          </div>

          {alphaStatus && (
            alphaStatus.parsed ? (
              <p className="fy-muted">
                {alphaStatus.loggedIn
                  ? `Signed in${alphaStatus.account ? ` as ${alphaStatus.account}` : ''}`
                  : alphaStatus.statusText || 'alphaXiv not logged in'}
              </p>
            ) : (
              <CLIRawFallback
                command={alphaStatus.command}
                rawOutput={alphaStatus.rawOutput}
                error={alphaStatus.error}
              />
            )
          )}

          <AlphaXivPanel
            root={root}
            embedded
            onInsertToChat={(text) => {
              onInsertToChat?.(text)
              onClose()
            }}
          />
        </section>

        <section className="fy-settings-card" aria-labelledby="fy-sec-search">
          <div className="fy-settings-card-head">
            <h4 id="fy-sec-search">Web Search</h4>
            <button
              type="button"
              className="pill xs quiet"
              disabled={busyOp !== ''}
              onClick={() => void handleClearSearch()}
            >
              Reset to Auto
            </button>
          </div>

          {searchStatus && (
            searchStatus.parsed ? (
              <ul className="fy-cli-kv-list">
                {searchStatus.entries.map((entry) => (
                  <li key={entry.key}>
                    <span className="fy-cli-kv-key">{entry.key}</span>
                    <span className="fy-cli-kv-val">{entry.value}</span>
                  </li>
                ))}
              </ul>
            ) : (
              <CLIRawFallback
                command={searchStatus.command}
                rawOutput={searchStatus.rawOutput}
                error={searchStatus.error}
              />
            )
          )}

          <form className="fy-cli-form" onSubmit={(e) => void handleSaveSearch(e)}>
            <div className="fy-cli-row">
              <OptionPopover
                label="Choose search provider"
                value={searchProvider}
                options={SEARCH_PROVIDERS}
                onSelect={setSearchProvider}
              />
              <input
                type="text"
                className="fy-cli-input"
                aria-label="Web search provider"
                placeholder="Provider (e.g. auto, exa, brave, tavily)"
                value={searchProvider}
                onChange={(e) => setSearchProvider(e.target.value)}
              />
            </div>
            <div className="fy-cli-row">
              <input
                type="password"
                className="fy-cli-input"
                aria-label="Web search API key"
                placeholder="Optional API key"
                autoComplete="off"
                value={searchApiKey}
                onChange={(e) => setSearchApiKey(e.target.value)}
              />
              <button
                type="submit"
                className="pill primary"
                disabled={!searchProvider.trim() || busyOp !== ''}
              >
                {busyOp === 'search-set' && <ResearchIcon name="spinner-gap" className="fy-spin" />}
                Save Web Search
              </button>
            </div>
          </form>
        </section>

        <section className="fy-settings-card" aria-labelledby="fy-sec-mcp">
          <div className="fy-settings-card-head">
            <h4 id="fy-sec-mcp">MCP Servers</h4>
            <button
              type="button"
              className="pill xs quiet"
              disabled={busyOp !== ''}
              onClick={() =>
                void runOp('mcp-list', async () => {
                  setMCPList(await ResearchHost.climcpList(root))
                })
              }
            >
              Refresh
            </button>
          </div>

          {mcpList && (
            mcpList.parsed ? (
              mcpList.servers.length === 0 ? (
                <p className="fy-muted">No user MCP servers configured.</p>
              ) : (
                <div className="fy-cli-list">
                  {mcpList.servers.map((srv) => (
                    <div key={`${srv.scope}:${srv.name}`} className="fy-cli-item">
                      <div className="fy-cli-item-copy">
                        <strong>{srv.name}</strong>
                        <small>
                          {[srv.scope, srv.transport, srv.url || srv.command, srv.exposure, srv.status]
                            .filter(Boolean)
                            .join(' · ')}
                        </small>
                        {srv.error && <small className="fy-cli-inline-error">{srv.error}</small>}
                      </div>
                      <button
                        type="button"
                        className="pill xs quiet"
                        disabled={busyOp !== ''}
                        aria-label={`Remove ${srv.name}${srv.scope ? ` (${srv.scope})` : ''}`}
                        onClick={() => void handleRemoveMCP(srv)}
                      >
                        Remove
                      </button>
                    </div>
                  ))}
                </div>
              )
            ) : (
              <CLIRawFallback
                command={mcpList.command}
                rawOutput={mcpList.rawOutput}
                error={mcpList.error}
              />
            )
          )}

          <form className="fy-cli-form" onSubmit={(e) => void handleAddMCP(e)}>
            <div className="fy-cli-row">
              <input
                type="text"
                className="fy-cli-input"
                aria-label="MCP server name"
                placeholder="Server name (e.g. zotero)"
                value={mcpName}
                onChange={(e) => setMCPName(e.target.value)}
              />
              <div className="seg" role="radiogroup" aria-label="MCP transport" onKeyDown={radioKeys}>
                {(['http', 'stdio'] as const).map((t) => (
                  <button
                    key={t}
                    type="button"
                    role="radio"
                    aria-checked={mcpTransport === t}
                    tabIndex={mcpTransport === t ? 0 : -1}
                    className={`pill xs${mcpTransport === t ? ' primary' : ' quiet'}`}
                    onClick={() => setMCPTransport(t)}
                  >
                    {t.toUpperCase()}
                  </button>
                ))}
              </div>
            </div>

            {mcpTransport === 'http' ? (
              <>
                <input
                  type="url"
                  className="fy-cli-input"
                  aria-label="MCP server URL"
                  placeholder="https://example.com/mcp or http://127.0.0.1:8123/mcp"
                  value={mcpUrl}
                  onChange={(e) => setMCPUrl(e.target.value)}
                />
                <div className="fy-cli-row">
                  <input
                    type="password"
                    className="fy-cli-input"
                    aria-label="MCP headers"
                    placeholder="Headers (KEY=VALUE, comma separated)"
                    autoComplete="off"
                    value={mcpHeaders}
                    onChange={(e) => setMCPHeaders(e.target.value)}
                  />
                  <input
                    type="text"
                    className="fy-cli-input"
                    aria-label="Bearer token env var"
                    placeholder="Bearer env var"
                    value={mcpBearerEnv}
                    onChange={(e) => setMCPBearerEnv(e.target.value)}
                  />
                </div>
              </>
            ) : (
              <>
                <div className="fy-cli-row">
                  <input
                    type="text"
                    className="fy-cli-input"
                    aria-label="MCP command"
                    placeholder="Command (e.g. npx)"
                    value={mcpCommand}
                    onChange={(e) => setMCPCommand(e.target.value)}
                  />
                  <input
                    type="text"
                    className="fy-cli-input"
                    aria-label="MCP command arguments"
                    placeholder="Arguments (space separated)"
                    value={mcpArgs}
                    onChange={(e) => setMCPArgs(e.target.value)}
                  />
                </div>
                <div className="fy-cli-row">
                  <input
                    type="password"
                    className="fy-cli-input"
                    aria-label="MCP environment variables"
                    placeholder="Env (KEY=VALUE, comma separated)"
                    autoComplete="off"
                    value={mcpEnv}
                    onChange={(e) => setMCPEnv(e.target.value)}
                  />
                  <input
                    type="text"
                    className="fy-cli-input"
                    aria-label="MCP working directory"
                    placeholder="Working directory"
                    value={mcpCwd}
                    onChange={(e) => setMCPCwd(e.target.value)}
                  />
                </div>
              </>
            )}

            <div className="fy-cli-row">
              <OptionPopover
                label="MCP exposure mode"
                value={mcpExposure}
                options={MCP_EXPOSURES}
                onSelect={setMCPExposure}
              />
              <input
                type="text"
                className="fy-cli-input"
                aria-label="MCP server description"
                placeholder="Optional description"
                value={mcpDesc}
                onChange={(e) => setMCPDesc(e.target.value)}
              />
            </div>

            <div className="fy-cli-row">
              <label className="fy-cli-check">
                <input
                  type="checkbox"
                  checked={mcpLocal}
                  onChange={(e) => setMCPLocal(e.target.checked)}
                />
                <span>Project-local (.pi/mcp.json)</span>
              </label>
              <button
                type="submit"
                className="pill primary"
                disabled={
                  !mcpName.trim() ||
                  (mcpTransport === 'http' ? !mcpUrl.trim() : !mcpCommand.trim()) ||
                  busyOp !== ''
                }
              >
                {busyOp === 'mcp-add' && <ResearchIcon name="spinner-gap" className="fy-spin" />}
                Add MCP Server
              </button>
            </div>
          </form>
        </section>

        <section className="fy-settings-card" aria-labelledby="fy-sec-packages">
          <div className="fy-settings-card-head">
            <h4 id="fy-sec-packages">Packages &amp; Updates</h4>
            <button
              type="button"
              className="pill xs primary"
              disabled={busyOp !== ''}
              onClick={() => void handlePackageAction('update', '')}
            >
              {busyOp === 'pkg-update-' && <ResearchIcon name="spinner-gap" className="fy-spin" />}
              Update All
            </button>
          </div>

          {packagesRes && (
            packagesRes.parsed ? (
              <div className="fy-cli-list">
                {packagesRes.packages.map((pkg) => (
                  <div key={pkg.name} className="fy-cli-item">
                    <div className="fy-cli-item-copy">
                      <strong>{pkg.name}</strong>
                      <small>
                        {[pkg.kind, pkg.description].filter(Boolean).join(' · ')}
                      </small>
                    </div>
                    {pkg.kind === 'optional' && (
                      <div className="fy-cli-actions">
                        <button
                          type="button"
                          className="pill xs"
                          disabled={busyOp !== ''}
                          onClick={() => void handlePackageAction('install', pkg.name)}
                        >
                          Install
                        </button>
                        <button
                          type="button"
                          className="pill xs quiet"
                          disabled={busyOp !== ''}
                          onClick={() => void handlePackageAction('remove', pkg.name)}
                        >
                          Remove
                        </button>
                        <button
                          type="button"
                          className="pill xs quiet"
                          disabled={busyOp !== ''}
                          onClick={() => void handlePackageAction('update', pkg.name)}
                        >
                          Update
                        </button>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            ) : (
              <CLIRawFallback
                command={packagesRes.command}
                rawOutput={packagesRes.rawOutput}
                error={packagesRes.error}
              />
            )
          )}
        </section>
      </ScrollArea>
    </div>
  )
}
