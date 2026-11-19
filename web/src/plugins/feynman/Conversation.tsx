import { memo, useCallback, useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import remarkMath from 'remark-math'
import rehypeKatex from 'rehype-katex'
import { ScrollArea } from '../../components/ScrollArea'
import { ResearchHost, type ResearchArtifact, type ResearchTool } from '../../host/generated/mygo'
import { DocumentDiff } from '../DocumentDiff'
import { actionGroup, actionLabel, activeAction } from './actions'
import type { ConversationItem, ResearchSession } from './session'

export function ResearchIcon({ name, className }: { name: string; className?: string }) {
  return <i aria-hidden="true" className={`ph ph-${name}${className ? ' ' + className : ''}`} />
}

function safeImageSrc(src?: string): string | null {
  if (!src) return null
  const trimmed = src.trim()
  if (/^data:image\/(?:png|jpeg|gif|webp);base64,[A-Za-z0-9+/=]+$/i.test(trimmed)) return trimmed
  if (/^(?:https?:\/\/|blob:)/i.test(trimmed)) return trimmed
  return null
}

function CodeBlock({ children }: { children?: React.ReactNode }) {
  const pre = useRef<HTMLPreElement>(null)
  const [copied, setCopied] = useState(false)
  useEffect(() => {
    if (!copied) return
    const timer = setTimeout(() => setCopied(false), 1400)
    return () => clearTimeout(timer)
  }, [copied])
  return (
    <div className="fy-code-block">
      <pre ref={pre}>{children}</pre>
      <button
        type="button"
        className="fy-icon"
        aria-label="Copy code"
        title="Copy code"
        onClick={() =>
          void (navigator.clipboard?.writeText(pre.current?.textContent ?? '') ?? Promise.reject())
            .then(() => setCopied(true))
            .catch(() => {
              if (!pre.current) return
              const range = document.createRange()
              range.selectNodeContents(pre.current)
              window.getSelection()?.removeAllRanges()
              window.getSelection()?.addRange(range)
              if (document.execCommand?.('copy')) setCopied(true)
            })
        }
      >
        <span key={copied ? 'check' : 'copy'} className={copied ? 'mgo-copy-morph' : undefined}>
          <ResearchIcon name={copied ? 'check' : 'copy'} />
        </span>
      </button>
    </div>
  )
}

const RichMessage = memo(function RichMessage({
  text,
  followLink,
}: {
  text: string
  followLink: (url: string) => void
}) {
  const prefix = useId() + '-note-'
  return (
    <div className="fy-markdown">
      <Markdown
        remarkPlugins={[remarkGfm, remarkMath]}
        remarkRehypeOptions={{ clobberPrefix: prefix }}
        rehypePlugins={[[rehypeKatex, { output: 'mathml', trust: false }]]}
        skipHtml
        components={{
          a: ({ href, children }) =>
            href ? (
              <a
                href={href}
                onClick={(e) => {
                  e.preventDefault()
                  if (href.startsWith('#')) {
                    e.currentTarget
                      .closest('.fy-markdown')
                      ?.querySelector('#' + CSS.escape(href.slice(1)))
                      ?.scrollIntoView({ block: 'nearest' })
                  } else followLink(href)
                }}
              >
                {children}
              </a>
            ) : (
              <span>{children}</span>
            ),
          img: ({ src, alt }) => {
            const resolved = safeImageSrc(src)
            return resolved ? (
              <img className="fy-chat-image" src={resolved} alt={alt ?? ''} loading="lazy" />
            ) : (
              <span>{alt}</span>
            )
          },
          pre: ({ children }) => <CodeBlock>{children}</CodeBlock>,
          table: ({ children }) => (
            <div className="fy-table">
              <table>{children}</table>
            </div>
          ),
        }}
      >
        {text}
      </Markdown>
    </div>
  )
})

function locationName(path: string) {
  return path.replace(/^.*\/research\/[0-9a-f]{64}\/work\//, '')
}

function toolTitle(tool: ResearchTool) {
  const title = tool.title || tool.kind || 'Tool'
  const file = tool.locations?.[0]?.path.split('/').at(-1)
  return file && /^(?:write|read|edit)$/i.test(title) ? title + ' · ' + file : title
}

function Step({
  tool,
  files,
  open,
}: {
  tool: ResearchTool
  files: ResearchArtifact[]
  open: (file: ResearchArtifact) => void
}) {
  const status = tool.status ?? 'pending'
  const running = status === 'pending' || status === 'in_progress'
  return (
    <details className="fy-step">
      <summary>
        <ResearchIcon
          name={status === 'completed' ? 'check' : status === 'failed' ? 'warning-circle' : 'spinner-gap'}
          className={running ? 'fy-spin' : undefined}
        />
        <span>{toolTitle(tool)}</span>
        <small>{status.replaceAll('_', ' ')}</small>
      </summary>
      {tool.input && (
        <div>
          <b>Input</b>
          <pre>{tool.input}</pre>
        </div>
      )}
      {tool.output && (
        <div>
          <b>Result</b>
          <pre>{tool.output}</pre>
        </div>
      )}
      {tool.terminalDelta && <pre aria-label="Command output">{tool.terminalDelta}</pre>}
      {tool.exitCode !== undefined && <small>Exit {tool.exitCode}</small>}
      {tool.locations?.map((loc, i) => {
        const path = locationName(loc.path)
        const file = files.find((f) => f.path === path || f.target === loc.path)
        return file ? (
          <button type="button" key={i} className="fy-link fy-path" onClick={() => open(file)}>
            {path}
          </button>
        ) : (
          <div className="fy-path" key={i} title={loc.path}>
            {path}
            {loc.line ? ' · ' + loc.line : ''}
          </div>
        )
      })}
      {tool.images?.map((src, i) => (
        <img key={i} src={src} alt="Tool result" />
      ))}
      {tool.diffs?.map((diff, i) => (
        <div key={i}>
          <b>{diff.path}</b>
          <DocumentDiff before={diff.oldText} after={diff.newText} />
        </div>
      ))}
    </details>
  )
}

export function Conversation({
  session,
  choose,
  open,
  onOpenReview,
}: {
  session: ResearchSession
  choose: (name: string) => void
  open: (file: ResearchArtifact) => Promise<void>
  onOpenReview: () => void
}) {
  const [viewport, setViewport] = useState<HTMLElement | null>(null)
  const pinned = useRef(true)
  const [atEnd, setAtEnd] = useState(true)
  const [copied, setCopied] = useState<number>()
  useEffect(() => {
    if (copied === undefined) return
    const timer = setTimeout(() => setCopied(undefined), 1400)
    return () => clearTimeout(timer)
  }, [copied])
  const revision = session.snapshot()
  const firstPrompt = session.messages.findIndex((message) => message.role === 'user')
  useLayoutEffect(() => {
    if (pinned.current && viewport) viewport.scrollTop = firstPrompt >= 0 ? viewport.scrollHeight : 0
  }, [viewport, revision, firstPrompt])
  function report(e: unknown) {
    session.error = String(e)
    session.changed()
  }
  const openFile = useRef(open)
  openFile.current = open
  const followLink = useCallback(
    (href: string) => {
      if (/^https?:\/\//i.test(href)) {
        void ResearchHost.openCitation(session.root, href).catch(report)
        return
      }
      const path = href.split('#')[0].replace(/^\.\//, '')
      const file = session.files.find((f) => f.path === path || f.target === path)
      if (file) void openFile.current(file).catch(report)
      else report('This link does not point to a published research file or web source.')
    },
    [session],
  )
  const renderMessage = (msg: ConversationItem, i: number) => {
    if (msg.role === 'tool') return null
    if (msg.role === 'plan') {
      return (
        <section key={i} className="fy-plan">
          <h3>Research plan</h3>
          <ol>
            {msg.steps.map((step, n) => (
              <li key={n} data-status={step.status}>
                <ResearchIcon name={step.status === 'completed' ? 'check-circle' : 'circle'} />
                {step.content}
              </li>
            ))}
          </ol>
        </section>
      )
    }
    if (msg.role === 'thought') {
      return (
        <details key={i} className="fy-thought">
          <summary>Thinking</summary>
          <p>{msg.text}</p>
        </details>
      )
    }
    if (msg.role === 'notice') {
      const tone = msg.notifyType || 'info'
      return (
        <p key={i} className="fy-notice" data-tone={tone} role={tone === 'error' ? 'alert' : 'status'}>
          <ResearchIcon
            name={tone === 'error' ? 'warning-circle' : tone === 'warning' ? 'warning' : 'info'}
          />
          <span>{msg.text}</span>
        </p>
      )
    }
    if (msg.role === 'user') {
      const action = activeAction(msg.text, session.commands)
      return (
        <div key={i} className="fy-prompt">
          {action ? (
            <>
              <strong>{actionLabel(action.command.name)}</strong>
              {action.argument}
            </>
          ) : (
            msg.text
          )}
        </div>
      )
    }
    return (
      <article className="fy-response" key={i}>
        <div className="fy-author">
          <ResearchIcon name="atom" /> Feynman
          <button
            type="button"
            className="fy-icon"
            title="Copy response"
            aria-label="Copy response"
            onClick={() =>
              void navigator.clipboard
                .writeText(msg.text)
                .then(() => setCopied(i))
                .catch(report)
            }
          >
            <span key={copied === i ? 'check' : 'copy'} className={copied === i ? 'mgo-copy-morph' : undefined}>
              <ResearchIcon name={copied === i ? 'check' : 'copy'} />
            </span>
          </button>
        </div>
        <RichMessage text={msg.text} followLink={followLink} />
        {msg.image && <img className="fy-chat-image" src={msg.image} alt="Research result" />}
      </article>
    )
  }

  const content = []
  for (let i = 0; i < session.messages.length; i++) {
    const msg = session.messages[i]
    if ((firstPrompt < 0 || i < firstPrompt) && msg.role === 'assistant') {
      content.push(
        <details className="fy-thought" key={i}>
          <summary>Session details</summary>
          <RichMessage text={msg.text} followLink={followLink} />
        </details>,
      )
      continue
    }
    if (msg.role !== 'tool') {
      content.push(renderMessage(msg, i))
      continue
    }
    const tools: ResearchTool[] = [msg.tool]
    const start = i
    while (session.messages[i + 1]?.role === 'tool') {
      const next = session.messages[++i]
      if (next.role === 'tool') tools.push(next.tool)
    }
    const running = tools.some((tool) => tool.status === 'pending' || tool.status === 'in_progress')
    if (tools.length === 1) {
      content.push(
        <Step
          key={start}
          tool={tools[0]}
          files={session.files}
          open={(file) => {
            void open(file).catch(report)
          }}
        />,
      )
      continue
    }
    content.push(
      <details className="fy-activity" key={start} open={running}>
        <summary>
          <ResearchIcon name={running ? 'spinner-gap' : 'list-checks'} className={running ? 'fy-spin' : undefined} />
          <span>{tools.length} tool calls</span>
          <ResearchIcon name="caret-down" />
        </summary>
        {tools.map((tool) => (
          <Step
            key={tool.toolCallId}
            tool={tool}
            files={session.files}
            open={(file) => {
              void open(file).catch(report)
            }}
          />
        ))}
      </details>,
    )
  }

  const quickWorkflows = session.commands
    .filter((c) => actionGroup(c) === 'Workflows')
    .slice(0, 4)

  return (
    <div className="fy-thread">
      <ScrollArea
        className="fy-transcript"
        contentClassName="research-conversation"
        aria-label="Conversation"
        onViewport={setViewport}
        onViewportScroll={() => {
          if (!viewport) return
          pinned.current = viewport.scrollHeight - viewport.clientHeight - viewport.scrollTop < 24
          setAtEnd(pinned.current)
        }}
      >
        {firstPrompt < 0 && quickWorkflows.length > 0 && (
          <div className="fy-start">
            <h3>Start with a workflow</h3>
            <div className="fy-launchers">
              {quickWorkflows.map((cmd) => (
                <button type="button" key={cmd.name} onClick={() => choose(cmd.name)}>
                  <ResearchIcon name="sparkle" />
                  <span>{actionLabel(cmd.name)}</span>
                </button>
              ))}
            </div>
          </div>
        )}
        {content}
        {session.activity.size > 0 && (
          <details className="fy-activity">
            <summary>
              <ResearchIcon name="pulse" />
              <span>Extension activity</span>
              <ResearchIcon name="caret-down" />
            </summary>
            {Array.from(session.activity, ([key, value]) => (
              <pre key={key}>{value}</pre>
            ))}
          </details>
        )}
        {session.status === 'working' && (
          <p className="fy-working" role="status">
            <ResearchIcon name="spinner-gap" className="fy-spin" />
            <span>{session.progress || 'Working…'}</span>
          </p>
        )}
        {session.proposals.length > 0 && (
          <button type="button" className="fy-review-callout" onClick={onOpenReview}>
            <ResearchIcon name="git-diff" />
            <span>
              {session.proposals.length} pending change {session.proposals.length === 1 ? 'proposal' : 'proposals'}
            </span>
            <ResearchIcon name="arrow-right" />
          </button>
        )}
      </ScrollArea>
      {!atEnd && (
        <button
          type="button"
          className="fy-jump pill"
          onClick={() => {
            pinned.current = true
            if (viewport) viewport.scrollTop = viewport.scrollHeight
          }}
        >
          <ResearchIcon name="arrow-down" />
          Latest
        </button>
      )}
    </div>
  )
}
