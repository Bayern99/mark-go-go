import { useState } from 'react'
import {
  ResearchHost,
  type CLIAlphaAnnotateResult,
  type CLIAlphaPaper,
  type CLIAlphaPaperResult,
  type CLIAlphaSearchResult,
  type CLICommandResult,
} from '../../host/generated/mygo'
import { ResearchIcon } from './Conversation'

const ALPHA_MODES = ['semantic', 'keyword', 'both', 'agentic', 'all'] as const

export function CLIRawFallback({
  command,
  rawOutput,
  error,
}: {
  command?: string
  rawOutput: string
  error?: string
}) {
  if (!rawOutput && !error) return null
  return (
    <div className="fy-cli-raw-card" data-testid="cli-raw-fallback">
      <div className="fy-cli-raw-head">
        <span>Raw CLI output</span>
        {command && <code>{command}</code>}
      </div>
      {error && error !== rawOutput && <p className="fy-cli-inline-error">{error}</p>}
      <pre className="fy-cli-raw">{rawOutput || error}</pre>
    </div>
  )
}

export function AlphaXivPanel({
  root,
  embedded = false,
  onClose,
  onInsertToChat,
}: {
  root: string
  embedded?: boolean
  onClose?: () => void
  onInsertToChat?: (text: string) => void
}) {
  const [query, setQuery] = useState('')
  const [mode, setMode] = useState<(typeof ALPHA_MODES)[number]>('semantic')
  const [searchResult, setSearchResult] = useState<CLIAlphaSearchResult | null>(null)
  const [paperId, setPaperId] = useState('')
  const [fullText, setFullText] = useState(false)
  const [paperResult, setPaperResult] = useState<CLIAlphaPaperResult | null>(null)
  const [question, setQuestion] = useState('')
  const [askResult, setAskResult] = useState<CLICommandResult | null>(null)
  const [repoUrl, setRepoUrl] = useState('')
  const [repoPath, setRepoPath] = useState('')
  const [codeResult, setCodeResult] = useState<CLICommandResult | null>(null)
  const [noteInput, setNoteInput] = useState('')
  const [annotateResult, setAnnotateResult] = useState<CLIAlphaAnnotateResult | null>(null)
  const [busyAction, setBusyAction] = useState('')
  const [error, setError] = useState('')
  const searchGroups = searchResult?.groups ?? [{ mode: '', papers: searchResult?.papers ?? [] }]

  async function runAction(name: string, task: () => Promise<void>) {
    setBusyAction(name)
    setError('')
    try {
      await task()
    } catch (err) {
      setError(String(err))
    } finally {
      setBusyAction('')
    }
  }

  async function handleSearch(e: React.FormEvent) {
    e.preventDefault()
    if (!query.trim()) return
    await runAction('search', async () => {
      const res = await ResearchHost.cliAlphaSearch(root, query.trim(), mode)
      setSearchResult(res)
      const firstPaper = res.groups?.find(group => group.papers.length > 0)?.papers[0] ?? res.papers?.[0]
      if (firstPaper?.id && !paperId) {
        setPaperId(firstPaper.id)
      }
    })
  }

  async function handleInspect(targetId = paperId, useFullText = fullText) {
    const id = targetId.trim()
    if (!id) return
    setPaperId(id)
    await runAction('get', async () => {
      const res = await ResearchHost.cliAlphaGet(root, id, useFullText)
      setPaperResult(res)
      if (res.annotation) setNoteInput(res.annotation)
    })
  }

  async function handleAsk(e: React.FormEvent) {
    e.preventDefault()
    if (!paperId.trim() || !question.trim()) return
    await runAction('ask', async () => {
      const res = await ResearchHost.cliAlphaAsk(root, paperId.trim(), question.trim())
      setAskResult(res)
    })
  }

  async function handleCode(e: React.FormEvent) {
    e.preventDefault()
    if (!repoUrl.trim()) return
    await runAction('code', async () => {
      const res = await ResearchHost.cliAlphaCode(root, repoUrl.trim(), repoPath.trim())
      setCodeResult(res)
    })
  }

  async function handleAnnotate(action: 'list' | 'get' | 'set' | 'clear') {
    await runAction(`annotate-${action}`, async () => {
      const res = await ResearchHost.cliAlphaAnnotate(
        root,
        action,
        paperId.trim(),
        action === 'set' ? noteInput.trim() : '',
      )
      setAnnotateResult(res)
      if (action === 'get' && res.note) setNoteInput(res.note)
      if (action === 'clear') setNoteInput('')
    })
  }

  function formatPaperForChat(paper: CLIAlphaPaper) {
    const lines = [`alphaXiv paper ${paper.id || ''}: ${paper.title}`.trim()]
    if (paper.authors) lines.push(`Authors: ${paper.authors}`)
    if (paper.url) lines.push(`URL: ${paper.url}`)
    if (paper.summary) lines.push(paper.summary)
    return lines.join('\n')
  }

  const content = (
    <div className="fy-alpha-body">
      {error && (
        <p className="fy-cli-inline-error" role="alert">
          {error}
        </p>
      )}

      <form className="fy-cli-form" onSubmit={(e) => void handleSearch(e)}>
        <div className="fy-cli-mode-row" role="group" aria-label="alphaXiv search mode">
          {ALPHA_MODES.map((m) => (
            <button
              key={m}
              type="button"
              className={`pill xs${mode === m ? ' primary' : ' quiet'}`}
              aria-pressed={mode === m}
              onClick={() => setMode(m)}
            >
              {m}
            </button>
          ))}
        </div>
        <div className="fy-cli-row">
          <input
            type="search"
            className="fy-cli-input"
            aria-label="alphaXiv search query"
            autoFocus={!embedded}
            placeholder="Search alphaXiv papers (e.g. sparse attention)…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <button
            type="submit"
            className="pill primary"
            disabled={!query.trim() || busyAction !== ''}
          >
            {busyAction === 'search' && <ResearchIcon name="spinner-gap" className="fy-spin" />}
            Search alphaXiv
          </button>
        </div>
      </form>

      {searchResult && (
        <div className="fy-alpha-results">
          {searchResult.parsed ? (
            searchGroups.map((group) => (
              <div key={group.mode} className="fy-alpha-results">
                {group.mode && <strong>{group.mode}</strong>}
                {group.papers.length === 0 && <p className="fy-muted">No matching alphaXiv papers.</p>}
                {group.papers.map((paper) => (
                <article key={paper.id || paper.title} className="fy-alpha-card">
                  <div className="fy-alpha-card-head">
                    <strong>{paper.title}</strong>
                    {paper.id && <code className="meta">{paper.id}</code>}
                  </div>
                  {(paper.authors || paper.published) && (
                    <small className="fy-muted">
                      {[paper.authors, paper.published].filter(Boolean).join(' · ')}
                    </small>
                  )}
                  {paper.summary && <p className="fy-alpha-summary">{paper.summary}</p>}
                  <div className="fy-cli-actions">
                    {paper.id && (
                      <button
                        type="button"
                        className="pill xs"
                        disabled={busyAction !== ''}
                        onClick={() => void handleInspect(paper.id, fullText)}
                      >
                        Inspect paper
                      </button>
                    )}
                    {onInsertToChat && (
                      <button
                        type="button"
                        className="pill xs quiet"
                        onClick={() => onInsertToChat(formatPaperForChat(paper))}
                      >
                        Cite in Chat
                      </button>
                    )}
                  </div>
                </article>
                ))}
              </div>
            ))
          ) : (
            <CLIRawFallback
              command={searchResult.command}
              rawOutput={searchResult.rawOutput}
              error={searchResult.error}
            />
          )}
        </div>
      )}

      <div className="fy-cli-subcard">
        <div className="fy-cli-subhead">
          <span>Paper inspect &amp; ask</span>
          <label className="fy-cli-check">
            <input
              type="checkbox"
              checked={fullText}
              onChange={(e) => setFullText(e.target.checked)}
            />
            <span>Full text</span>
          </label>
        </div>
        <div className="fy-cli-row">
          <input
            type="text"
            className="fy-cli-input"
            aria-label="Paper ID or URL"
            placeholder="arXiv/alphaXiv ID or URL (e.g. 2501.01234)"
            value={paperId}
            onChange={(e) => setPaperId(e.target.value)}
          />
          <button
            type="button"
            className="pill"
            disabled={!paperId.trim() || busyAction !== ''}
            onClick={() => void handleInspect(paperId, fullText)}
          >
            {busyAction === 'get' && <ResearchIcon name="spinner-gap" className="fy-spin" />}
            Get
          </button>
        </div>

        {paperResult && (
          <div className="fy-alpha-detail">
            {paperResult.parsed ? (
              <>
                {paperResult.title && <strong>{paperResult.title}</strong>}
                {paperResult.annotation && (
                  <p className="fy-alpha-note-pill">Note: {paperResult.annotation}</p>
                )}
                <pre className="fy-cli-raw">{paperResult.content || paperResult.rawOutput}</pre>
                {onInsertToChat && (
                  <button
                    type="button"
                    className="pill xs"
                    onClick={() =>
                      onInsertToChat(
                        `Paper ${paperResult.paperId}${paperResult.title ? ` (${paperResult.title})` : ''}:\n${(paperResult.content || paperResult.rawOutput).slice(0, 1200)}`,
                      )
                    }
                  >
                    Insert to Chat
                  </button>
                )}
              </>
            ) : (
              <CLIRawFallback
                command={paperResult.command}
                rawOutput={paperResult.rawOutput}
                error={paperResult.error}
              />
            )}
          </div>
        )}

        <form className="fy-cli-row" onSubmit={(e) => void handleAsk(e)}>
          <input
            type="text"
            className="fy-cli-input"
            aria-label="Ask about paper"
            placeholder="Ask a question about this paper…"
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
          />
          <button
            type="submit"
            className="pill"
            disabled={!paperId.trim() || !question.trim() || busyAction !== ''}
          >
            {busyAction === 'ask' && <ResearchIcon name="spinner-gap" className="fy-spin" />}
            Ask paper
          </button>
        </form>

        {askResult && (
          <div className="fy-alpha-detail">
            <pre className="fy-cli-raw">{askResult.rawOutput || askResult.error}</pre>
            {onInsertToChat && askResult.rawOutput && (
              <button
                type="button"
                className="pill xs"
                onClick={() =>
                  onInsertToChat(`alphaXiv Q&A (${paperId}): ${question}\n${askResult.rawOutput}`)
                }
              >
                Insert to Chat
              </button>
            )}
          </div>
        )}
      </div>

      <form className="fy-cli-subcard" onSubmit={(e) => void handleCode(e)}>
        <div className="fy-cli-subhead">
          <span>Paper repository lookup</span>
        </div>
        <div className="fy-cli-stack">
          <input
            type="url"
            className="fy-cli-input"
            aria-label="GitHub repository URL"
            placeholder="https://github.com/owner/repo"
            value={repoUrl}
            onChange={(e) => setRepoUrl(e.target.value)}
          />
          <div className="fy-cli-row">
            <input
              type="text"
              className="fy-cli-input"
              aria-label="Repository path"
              placeholder="Optional file or directory path"
              value={repoPath}
              onChange={(e) => setRepoPath(e.target.value)}
            />
            <button
              type="submit"
              className="pill"
              disabled={!repoUrl.trim() || busyAction !== ''}
            >
              {busyAction === 'code' && <ResearchIcon name="spinner-gap" className="fy-spin" />}
              Inspect code
            </button>
          </div>
        </div>
        {codeResult && (
          <div className="fy-alpha-detail">
            <pre className="fy-cli-raw">{codeResult.rawOutput || codeResult.error}</pre>
            {onInsertToChat && codeResult.rawOutput && (
              <button
                type="button"
                className="pill xs"
                onClick={() =>
                  onInsertToChat(
                    `Repository ${repoUrl}${repoPath ? ` (${repoPath})` : ''}:\n\`\`\`\n${codeResult.rawOutput.slice(0, 1200)}\n\`\`\``,
                  )
                }
              >
                Insert to Chat
              </button>
            )}
          </div>
        )}
      </form>

      <div className="fy-cli-subcard">
        <div className="fy-cli-subhead">
          <span>Local annotations</span>
          <button
            type="button"
            className="pill xs quiet"
            disabled={busyAction !== ''}
            onClick={() => void handleAnnotate('list')}
          >
            List all
          </button>
        </div>
        <div className="fy-cli-stack">
          <input
            type="text"
            className="fy-cli-input"
            aria-label="Paper annotation"
            placeholder="Add or update local note for the selected paper…"
            value={noteInput}
            onChange={(e) => setNoteInput(e.target.value)}
          />
          <div className="fy-cli-actions">
            <button
              type="button"
              className="pill xs"
              disabled={!paperId.trim() || !noteInput.trim() || busyAction !== ''}
              onClick={() => void handleAnnotate('set')}
            >
              Save note
            </button>
            <button
              type="button"
              className="pill xs quiet"
              disabled={!paperId.trim() || busyAction !== ''}
              onClick={() => void handleAnnotate('get')}
            >
              Read note
            </button>
            <button
              type="button"
              className="pill xs quiet"
              disabled={!paperId.trim() || busyAction !== ''}
              onClick={() => void handleAnnotate('clear')}
            >
              Clear note
            </button>
          </div>
        </div>
        {annotateResult && (
          <div className="fy-alpha-detail">
            {annotateResult.parsed ? (
              annotateResult.annotations.length > 0 ? (
                <ul className="fy-cli-kv-list">
                  {annotateResult.annotations.map((ann) => (
                    <li key={ann.paperId}>
                      <code>{ann.paperId}</code>
                      <span className="fy-alpha-note">{ann.note}</span>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="fy-muted">{annotateResult.note || annotateResult.rawOutput}</p>
              )
            ) : (
              <CLIRawFallback
                command={annotateResult.command}
                rawOutput={annotateResult.rawOutput}
                error={annotateResult.error}
              />
            )}
          </div>
        )}
      </div>
    </div>
  )

  if (embedded) return content

  return (
    <div className="fy-settings-view">
      <div className="fy-settings-topbar">
        {onClose && (
          <button type="button" className="pill quiet" onClick={onClose}>
            <ResearchIcon name="arrow-left" />
            Back to Chat
          </button>
        )}
        <h3>alphaXiv Research</h3>
      </div>
      <div className="fy-settings-scroll">{content}</div>
    </div>
  )
}
