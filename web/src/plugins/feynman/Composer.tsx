import { useEffect, useId, useRef, useState, type RefObject } from 'react'
import { Popover } from 'radix-ui'
import type { DocumentSnapshot } from '../../core'
import type { ResearchConfig, ResearchModel } from '../../host/generated/mygo'
import { ResearchIcon } from './Conversation'
import { actionGroup, actionLabel, actionMeta, activeAction } from './actions'
import type { ResearchSession } from './session'

const numberFmt = new Intl.NumberFormat()

function formatCost(cost: number): string {
  if (cost <= 0) return '$0.00'
  if (cost < 0.01) return `$${cost.toFixed(4)}`
  return `$${cost.toFixed(2)}`
}

export function useCommand(session: ResearchSession, input: RefObject<HTMLTextAreaElement | null>) {
  return (name: string) => {
    const args =
      activeAction(session.draft, session.commands)?.argument ??
      (/^\/[\w:-]*$/.test(session.draft) ? '' : session.draft)
    session.draft = '/' + name.replace(/^\//, '') + ' ' + args
    session.changed()
    input.current?.focus()
  }
}

interface PickerItem {
  value: string
  label: string
  detail?: string
}

function PortraitPicker({
  icon,
  label,
  currentValue,
  items,
  disabled,
  onSelect,
}: {
  icon: string
  label: string
  currentValue: string
  items: PickerItem[]
  disabled: boolean
  onSelect: (value: string) => void
}) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const current = items.find((item) => item.value === currentValue)
  const matches = items.filter((item) =>
    `${item.label} ${item.value} ${item.detail ?? ''}`.toLowerCase().includes(query.trim().toLowerCase()),
  )
  return (
    <Popover.Root
      open={open}
      onOpenChange={(value) => {
        setOpen(value)
        setQuery('')
      }}
    >
      <Popover.Trigger asChild>
        <button
          type="button"
          className="fy-config"
          aria-label={label}
          title={`${label}: ${current?.label || currentValue || 'Default'}`}
          disabled={disabled}
        >
          <ResearchIcon name={icon} />
          <span>{current?.label || currentValue || label}</span>
          <ResearchIcon name="caret-down" />
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          className="fy-popover"
          side="top"
          align="start"
          sideOffset={7}
          collisionPadding={8}
          aria-label={label}
        >
          {items.length > 4 && (
            <label className="fy-filter">
              <ResearchIcon name="magnifying-glass" />
              <input
                autoFocus
                aria-label={`Find ${label}`}
                placeholder={`Find ${label.toLowerCase()}…`}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'ArrowDown') {
                    e.preventDefault()
                    e.currentTarget
                      .closest('.fy-popover')
                      ?.querySelector<HTMLButtonElement>('.fy-options button')
                      ?.focus()
                  }
                }}
              />
            </label>
          )}
          <div
            className="fy-options"
            role="group"
            aria-label={label}
            onKeyDown={(e) => {
              if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return
              const choices = Array.from(e.currentTarget.querySelectorAll('button'))
              if (!choices.length) return
              e.preventDefault()
              const next = choices.indexOf(e.target as HTMLButtonElement) + (e.key === 'ArrowDown' ? 1 : -1)
              choices[(next + choices.length) % choices.length].focus()
            }}
          >
            {matches.map((item) => (
              <button
                key={item.value}
                type="button"
                aria-pressed={item.value === currentValue}
                onClick={() => {
                  setOpen(false)
                  onSelect(item.value)
                }}
              >
                <span className="fy-option-copy">
                  <span>{item.label}</span>
                  {item.detail && <small>{item.detail}</small>}
                </span>
                {item.value === currentValue && <ResearchIcon name="check" />}
              </button>
            ))}
            {!matches.length && <p className="fy-options-empty">No matching options.</p>}
          </div>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}

function modelKey(model: ResearchModel): string {
  return model.provider ? `${model.provider}/${model.id}` : model.id
}

function ConfigControls({ session }: { session: ResearchSession }) {
  const disabled = session.configuring || session.status !== 'ready'
  const modelItems: PickerItem[] = session.models.length
    ? session.models.map((m) => ({
        value: modelKey(m),
        label: m.name || m.id,
        detail: m.provider,
      }))
    : (session.config.find((c) => c.id === 'model' || c.category === 'model')?.options ?? []).map((o) => ({
        value: o.value,
        label: o.name,
        detail: o.description,
      }))
  const currentModel = session.sessionState?.model
    ? modelKey(session.sessionState.model)
    : session.config.find((c) => c.id === 'model' || c.category === 'model')?.currentValue ?? ''

  const thinkingItems: PickerItem[] = session.thinkingLevels.length
    ? session.thinkingLevels.map((level) => ({
        value: level,
        label: level.charAt(0).toUpperCase() + level.slice(1),
      }))
    : (session.config.find((c) => c.category === 'thinking' || c.id === 'thinking_level')?.options ?? []).map(
        (o) => ({ value: o.value, label: o.name }),
      )
  const currentThinking =
    session.thinkingLevel ||
    session.sessionState?.thinkingLevel ||
    session.config.find((c) => c.category === 'thinking' || c.id === 'thinking_level')?.currentValue ||
    ''

  const extraConfigs = session.config.filter(
    (c) => c.id !== 'model' && c.category !== 'model' && c.category !== 'thinking' && c.id !== 'thinking_level',
  )

  const usedTokens = session.stats?.contextUsage?.tokens ?? session.usage?.used ?? session.stats?.tokens.total
  const maxTokens = session.stats?.contextUsage?.contextWindow ?? session.usage?.size
  const totalTokens = session.stats?.tokens.total ?? usedTokens
  const cost = session.stats?.cost
  const pct = maxTokens && usedTokens !== undefined ? Math.min(100, Math.round((usedTokens / maxTokens) * 100)) : 0

  if (!modelItems.length && !thinkingItems.length && !extraConfigs.length && !session.stats && !session.usage) {
    return null
  }

  return (
    <div className="fy-configs">
      {modelItems.length > 0 && (
        <PortraitPicker
          icon="cube"
          label="Model"
          currentValue={currentModel}
          items={modelItems}
          disabled={disabled}
          onSelect={(value) => {
            const matched = session.models.find((m) => modelKey(m) === value)
            if (matched) {
              void session.setModel(matched.provider, matched.id)
            } else {
              const slash = value.indexOf('/')
              if (slash > 0) void session.setModel(value.slice(0, slash), value.slice(slash + 1))
              else void session.configure('model', value)
            }
          }}
        />
      )}
      {thinkingItems.length > 0 && (
        <PortraitPicker
          icon="sliders-horizontal"
          label="Thinking"
          currentValue={currentThinking}
          items={thinkingItems}
          disabled={disabled}
          onSelect={(value) => {
            if (session.thinkingLevels.length > 0) void session.setThinkingLevel(value)
            else void session.configure('thinking_level', value)
          }}
        />
      )}
      {extraConfigs.map((option: ResearchConfig) => (
        <PortraitPicker
          key={option.id}
          icon="sliders-horizontal"
          label={option.name}
          currentValue={option.currentValue}
          items={option.options.map((v) => ({ value: v.value, label: v.name, detail: v.description }))}
          disabled={disabled}
          onSelect={(value) => void session.configure(option.id, value)}
        />
      ))}
      {(totalTokens !== undefined || cost !== undefined || maxTokens) && (
        <div
          className="fy-stats meta"
          data-testid="fy-session-stats"
          title={
            session.stats
              ? `Input ${numberFmt.format(session.stats.tokens.input)} · Output ${numberFmt.format(session.stats.tokens.output)} · Cache ${numberFmt.format(session.stats.tokens.cacheRead)}`
              : maxTokens
                ? `${numberFmt.format(usedTokens ?? 0)} / ${numberFmt.format(maxTokens)} tokens`
                : undefined
          }
        >
          {maxTokens ? (
            <div
              className="fy-meter"
              role="progressbar"
              aria-label="Context used"
              aria-valuemin={0}
              aria-valuemax={maxTokens}
              aria-valuenow={usedTokens ?? 0}
            >
              <span className="fy-meter-fill" style={{ width: `${pct}%` }} />
            </div>
          ) : null}
          {totalTokens !== undefined && (
            <span>
              {numberFmt.format(totalTokens)}
              {maxTokens ? ` / ${numberFmt.format(maxTokens)}` : ' tok'}
            </span>
          )}
          {cost !== undefined && <span>{formatCost(cost)}</span>}
        </div>
      )}
    </div>
  )
}

export function Composer({
  session,
  doc,
  documentID,
  input,
  choose,
  edit,
  setEdit,
  onOpenAlpha,
}: {
  session: ResearchSession
  doc?: DocumentSnapshot
  documentID: string
  input: RefObject<HTMLTextAreaElement | null>
  choose: (name: string) => void
  edit: boolean
  setEdit: (value: boolean) => void
  onOpenAlpha?: () => void
}) {
  const [browse, setBrowse] = useState(false)
  const [filter, setFilter] = useState('')
  const [dismissed, setDismissed] = useState<string>()
  const [index, setIndex] = useState(0)
  const results = useRef<HTMLDivElement>(null)
  const listId = useId()
  const slash = /^\/([^\s]*)$/.exec(session.draft)
  const action = slash ? undefined : activeAction(session.draft, session.commands)
  const argument = action?.argument ?? session.draft

  function setArgument(text: string) {
    session.draft = action ? '/' + action.command.name + ' ' + text : text
    setIndex(0)
    setDismissed(undefined)
    session.changed()
  }

  const visible = session.commands.length > 0 && (browse || (slash !== null && dismissed !== session.draft))
  const query = (browse ? filter : slash?.[1] ?? '').toLowerCase()
  const rank = (name: string) =>
    name === query ? 0 : name.startsWith(query) ? 1 : name.includes(query) ? 2 : 3
  const groupPriority = (group: string) => {
    const order = ['Workflows', 'Skills', 'Extensions']
    const idx = order.indexOf(group)
    return idx >= 0 ? idx : order.length
  }
  const commands = session.commands
    .filter((c) =>
      [c.name, actionLabel(c.name), c.description, actionGroup(c), actionMeta(c)]
        .join(' ')
        .toLowerCase()
        .includes(query),
    )
    .sort(
      (a, b) =>
        (query
          ? rank(a.name.toLowerCase()) - rank(b.name.toLowerCase())
          : groupPriority(actionGroup(a)) - groupPriority(actionGroup(b)) ||
            actionGroup(a).localeCompare(actionGroup(b))) ||
        actionLabel(a.name).localeCompare(actionLabel(b.name)),
    )
  const selected = Math.min(index, commands.length - 1)
  const busy = session.status === 'working'
  const offline = session.status === 'offline' || session.status === 'connecting'
  const locked = session.status !== 'ready' || session.configuring
  const hasDraft = Boolean(session.draft.trim())
  const canSend = !locked && hasDraft
  const totalQueued = session.queue.steering.length + session.queue.followUp.length

  function pick(name: string) {
    setBrowse(false)
    setFilter('')
    setIndex(0)
    choose(name)
  }

  function closeList(restoreFocus = true) {
    setBrowse(false)
    setDismissed(session.draft)
    if (restoreFocus) input.current?.focus()
  }

  function keys(e: React.KeyboardEvent) {
    if (e.nativeEvent.isComposing || e.keyCode === 229 || e.metaKey || e.ctrlKey || !visible) return false
    if (e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      closeList()
      return true
    }
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      setIndex(Math.max(0, Math.min(commands.length - 1, selected + (e.key === 'ArrowDown' ? 1 : -1))))
      return true
    }
    if ((e.key === 'Enter' || e.key === 'Tab') && !e.shiftKey && selected >= 0) {
      e.preventDefault()
      pick(commands[selected].name)
      return true
    }
    return false
  }

  useEffect(() => {
    results.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' })
  }, [selected, query])

  useEffect(() => {
    const el = input.current
    if (el) {
      el.style.height = 'auto'
      el.style.height = Math.min(124, el.scrollHeight) + 'px'
    }
  }, [session.draft, input])

  return (
    <div className="fy-compose">
      {totalQueued > 0 && (
        <div className="fy-queue-bar" data-testid="fy-queue-bar" aria-label="Queued messages">
          <span className="fy-queue-summary meta">
            {session.queue.steering.length > 0 && `${session.queue.steering.length} steering`}
            {session.queue.steering.length > 0 && session.queue.followUp.length > 0 && ' · '}
            {session.queue.followUp.length > 0 && `${session.queue.followUp.length} follow-up`}
          </span>
          <button
            type="button"
            className="pill quiet"
            data-testid="fy-clear-queue"
            onClick={() => void session.clearQueue()}
          >
            Clear queue
          </button>
        </div>
      )}
      {session.suggestedDraft && (
        <div className="fy-suggestion">
          <span>{session.suggestedDraft}</span>
          <button
            className="pill"
            type="button"
            onClick={() => {
              const text = session.suggestedDraft!
              session.suggestedDraft = undefined
              const suggestedAction = activeAction(text, session.commands)
              if (suggestedAction && !suggestedAction.argument.trim()) choose(suggestedAction.command.name)
              else {
                session.draft = [session.draft, text].filter(Boolean).join('\n')
                session.changed()
                input.current?.focus()
              }
            }}
          >
            Insert suggestion
          </button>
          <button
            className="fy-icon"
            type="button"
            aria-label="Dismiss suggestion"
            onClick={() => {
              session.suggestedDraft = undefined
              session.changed()
            }}
          >
            <ResearchIcon name="x" />
          </button>
        </div>
      )}
      {visible && (
        <section
          className="fy-command-palette"
          data-browsing={browse || undefined}
          aria-label="Choose a research action"
          onBlur={(e) => {
            if (e.relatedTarget && !e.currentTarget.contains(e.relatedTarget) && e.relatedTarget !== input.current) {
              closeList(false)
            }
          }}
        >
          {browse && (
            <div className="fy-filter">
              <ResearchIcon name="magnifying-glass" />
              <input
                autoFocus
                aria-label="Find a research action"
                placeholder="Find a workflow, skill or extension…"
                value={filter}
                onKeyDown={keys}
                onChange={(e) => {
                  setFilter(e.target.value)
                  setIndex(0)
                }}
              />
              <button type="button" className="fy-icon" aria-label="Close actions" onClick={() => closeList()}>
                <ResearchIcon name="x" />
              </button>
            </div>
          )}
          <div id={listId} role="listbox" aria-label="Commands and skills" className="fy-command-list" ref={results}>
            {commands.map((c, i) => {
              const group = actionGroup(c)
              const meta = actionMeta(c)
              return (
                <button
                  key={c.name}
                  id={listId + '-' + i}
                  role="option"
                  type="button"
                  aria-selected={i === selected}
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => pick(c.name)}
                >
                  {browse && !query && (i === 0 || group !== actionGroup(commands[i - 1])) && (
                    <small className="fy-group-name">{group}</small>
                  )}
                  <span className="fy-action-name">
                    <span>{actionLabel(c.name)}</span>
                    <code>/{c.name}</code>
                    {meta && <small className="fy-action-meta">{meta}</small>}
                  </span>
                  {c.description && <small>{c.description}</small>}
                </button>
              )
            })}
            {!commands.length && <p>No matching actions.</p>}
          </div>
        </section>
      )}
      <form
        className="fy-input-card"
        onSubmit={(e) => {
          e.preventDefault()
          if (busy && hasDraft) void session.followUp()
          else if (canSend) void session.send(documentID, edit ? doc : undefined)
        }}
      >
        {(session.sources.length > 0 || edit || action) && (
          <div className="fy-attachments">
            {action && (
              <button
                type="button"
                aria-label={'Remove ' + actionLabel(action.command.name)}
                title={action.command.description}
                onClick={() => {
                  session.draft = argument
                  session.changed()
                  input.current?.focus()
                }}
              >
                <ResearchIcon name="sparkle" />
                {actionLabel(action.command.name)} ×
              </button>
            )}
            {edit && (
              <button type="button" onClick={() => setEdit(false)} title="Remove edit target">
                <ResearchIcon name="pencil-simple" />
                {doc?.path.split('/').at(-1)} ×
              </button>
            )}
            {session.sources.map((path) => (
              <button
                type="button"
                key={path}
                title={'Remove ' + path}
                onClick={() => {
                  session.sources = session.sources.filter((s) => s !== path)
                  session.changed()
                }}
              >
                <ResearchIcon name="paperclip" />
                {path.split('/').at(-1)} ×
              </button>
            ))}
          </div>
        )}
        <textarea
          ref={input}
          aria-label="Research prompt"
          rows={2}
          placeholder={
            busy
              ? 'Steer or queue a follow-up…'
              : action
                ? action.command.input?.hint || 'Add a topic or instructions…'
                : 'Ask Feynman…'
          }
          value={argument}
          aria-autocomplete="list"
          aria-controls={visible ? listId : undefined}
          aria-activedescendant={visible && selected >= 0 ? listId + '-' + selected : undefined}
          onChange={(e) => setArgument(e.target.value)}
          onKeyDown={(e) => {
            if (e.nativeEvent.isComposing || e.keyCode === 229 || keys(e)) return
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault()
              e.currentTarget.form?.requestSubmit()
            }
          }}
        />
        <div className="fy-compose-actions">
          <button
            className="fy-icon"
            type="button"
            aria-label="Attach source files"
            title="Attach source files"
            disabled={offline || session.configuring}
            onClick={() => void session.attach()}
          >
            <ResearchIcon name="paperclip" />
          </button>
          <button
            className="fy-actions-trigger"
            type="button"
            aria-label="Research actions"
            title="Workflows, skills and extensions"
            aria-expanded={visible}
            disabled={!session.commands.length}
            onClick={() => {
              if (visible) closeList()
              else {
                setBrowse(true)
                setFilter('')
                setIndex(0)
              }
            }}
          >
            <ResearchIcon name="squares-four" />
            Actions
          </button>
          {onOpenAlpha && (
            <button
              className="fy-actions-trigger"
              type="button"
              data-testid="fy-alphaxiv-trigger"
              aria-label="alphaXiv research"
              title="Search and inspect alphaXiv papers"
              onClick={onOpenAlpha}
            >
              <ResearchIcon name="books" />
              alphaXiv
            </button>
          )}
          {doc?.path && (
            <label className={'fy-edit-toggle' + (edit ? ' selected' : '')} title={'Edit ' + doc.path.split('/').at(-1)}>
              <input
                type="checkbox"
                aria-label="Edit this document"
                checked={edit}
                disabled={busy}
                onChange={(e) => setEdit(e.target.checked)}
              />
              <ResearchIcon name="note-pencil" />
              <span>Edit document</span>
            </label>
          )}
          <div className="fy-action-spacer" />
          {busy ? (
            <div className="fy-midturn-actions">
              <button
                type="button"
                className="pill quiet"
                data-testid="fy-steer"
                disabled={!hasDraft}
                title="Steer current turn"
                onClick={() => void session.steer()}
              >
                Steer
              </button>
              <button
                type="button"
                className="pill quiet"
                data-testid="fy-follow-up"
                disabled={!hasDraft}
                title="Queue follow-up message"
                onClick={() => void session.followUp()}
              >
                Follow up
              </button>
              <button
                className="fy-send"
                type="button"
                aria-label="Abort research"
                title="Abort research"
                onClick={() => void session.abort()}
              >
                <ResearchIcon name="stop" />
              </button>
            </div>
          ) : (
            <button className="fy-send" disabled={!canSend} aria-label="Send" title="Send · Enter">
              <ResearchIcon name="arrow-up" />
              <span className="fy-sr">Send</span>
            </button>
          )}
        </div>
      </form>
      <ConfigControls session={session} />
    </div>
  )
}
