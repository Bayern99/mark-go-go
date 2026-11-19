import { useEffect, useId, useRef, useState, type RefObject } from 'react'
import { Popover } from 'radix-ui'
import type { DocumentSnapshot } from '../../core'
import { ResearchHost, type ResearchConfig } from '../../host/generated/mygo'
import { ResearchIcon } from './Conversation'
import type { ResearchSession } from './session'
import { actionGroup, actionLabel, activeAction } from './actions'

const deliveryChoices = ['all', 'one-at-a-time']
const choicesByCommand = new Map([['autocompact', ['on', 'off']], ['steering', deliveryChoices], ['follow-up', deliveryChoices]])

export function useCommand(session: ResearchSession, input: RefObject<HTMLTextAreaElement | null>) {
  return (name: string) => {
    const args = activeAction(session.draft, session.commands)?.argument ??
      (/^\/[\w:-]*$/.test(session.draft) ? '' : session.draft)
    session.draft = '/' + name.replace(/^\//, '') + ' ' + args
    session.changed()
    input.current?.focus()
  }
}

function ConfigPicker({ option, session }: { option: ResearchConfig; session: ResearchSession }) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const current = option.options.find(v => v.value === option.currentValue)
  const matches = option.options.filter(v => (v.name + ' ' + v.value).toLowerCase().includes(query.toLowerCase()))
  return <Popover.Root open={open} onOpenChange={value => { setOpen(value); setQuery('') }}>
    <Popover.Trigger asChild><button type="button" className="fy-config" aria-label={option.name}
      title={option.name + ': ' + (current?.name || option.currentValue)} disabled={session.configuring || session.status !== 'ready'}>
      <ResearchIcon name={option.category === 'model' || option.id === 'model' ? 'cube' : 'sliders-horizontal'} />
      <span>{current?.name || option.currentValue || option.name}</span><ResearchIcon name="caret-down" />
    </button></Popover.Trigger>
    <Popover.Portal><Popover.Content className="fy-popover" side="top" align="start" sideOffset={7} collisionPadding={8} aria-label={option.name}>
      <label className="fy-filter"><ResearchIcon name="magnifying-glass" /><input autoFocus aria-label={'Find ' + option.name} placeholder={'Find ' + option.name.toLowerCase() + '…'}
        value={query} onChange={e => setQuery(e.target.value)} onKeyDown={e => {
          if (e.key === 'ArrowDown') {
            e.preventDefault()
            e.currentTarget.closest('.fy-popover')?.querySelector<HTMLButtonElement>('.fy-options button')?.focus()
          }
        }} /></label>
      <div className="fy-options" role="group" aria-label={option.name} onKeyDown={e => {
        if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return
        const choices = Array.from(e.currentTarget.querySelectorAll('button'))
        if (!choices.length) return
        e.preventDefault()
        const next = choices.indexOf(e.target as HTMLButtonElement) + (e.key === 'ArrowDown' ? 1 : -1)
        choices[(next + choices.length) % choices.length].focus()
      }}>{matches.map(value =>
        <button key={value.value} type="button" aria-pressed={value.value === option.currentValue} onClick={() => {
          setOpen(false); void session.configure(option.id, value.value)
        }}><span>{value.name}</span>{value.value === option.currentValue && <ResearchIcon name="check" />}</button>)}
        {!matches.length && <p>No matching options.</p>}</div>
    </Popover.Content></Popover.Portal>
  </Popover.Root>
}

export function Composer({ session, doc, documentID, input, choose, edit, setEdit }: {
  session: ResearchSession; doc?: DocumentSnapshot; documentID: string; input: RefObject<HTMLTextAreaElement | null>;
  choose: (name: string) => void; edit: boolean; setEdit: (value: boolean) => void
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
  const choices = action && choicesByCommand.get(action.command.name)
  function setArgument(text: string) {
    session.draft = action ? '/' + action.command.name + ' ' + text : text
    setIndex(0); setDismissed(undefined); session.changed()
  }
  const visible = session.commands.length > 0 && (browse || (slash !== null && dismissed !== session.draft))
  const query = (browse ? filter : slash?.[1] ?? '').toLowerCase()
  const rank = (name: string) => name === query ? 0 : name.startsWith(query) ? 1 : name.includes(query) ? 2 : 3
  const groups = ['Workflows', 'Skills', 'Extensions', 'Session']
  const commands = session.commands.filter(c => [c.name, actionLabel(c.name), c.description].join(' ').toLowerCase().includes(query))
    .sort((a, b) => (query ? rank(a.name.toLowerCase()) - rank(b.name.toLowerCase()) :
      groups.indexOf(actionGroup(a)) - groups.indexOf(actionGroup(b))) || actionLabel(a.name).localeCompare(actionLabel(b.name)))
  const selected = Math.min(index, commands.length - 1)
  const busy = session.status === 'working'
  const locked = session.status !== 'ready' || session.configuring
  const canSend = !locked && !!session.draft.trim() && !(edit && !!session.proposal)
  function pick(name: string) { setBrowse(false); setFilter(''); setIndex(0); choose(name) }
  function closeList(restoreFocus = true) {
    setBrowse(false); setDismissed(session.draft)
    if (restoreFocus) input.current?.focus()
  }
  function keys(e: React.KeyboardEvent) {
    if (e.nativeEvent.isComposing || e.keyCode === 229 || e.metaKey || e.ctrlKey || !visible) return false
    if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); closeList(); return true }
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      setIndex(Math.max(0, Math.min(commands.length - 1, selected + (e.key === 'ArrowDown' ? 1 : -1))))
      return true
    }
    if ((e.key === 'Enter' || e.key === 'Tab') && !e.shiftKey && selected >= 0) {
      e.preventDefault(); pick(commands[selected].name); return true
    }
    return false
  }
  useEffect(() => {
    results.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({block: 'nearest'})
  }, [selected, query])
  useEffect(() => {
    const el = input.current
    if (el) { el.style.height = 'auto'; el.style.height = Math.min(124, el.scrollHeight) + 'px' }
  }, [session.draft, input])
  const report = (err: unknown) => { session.error = String(err); session.changed() }
  return <div className="fy-compose">
    {session.suggestedDraft && <div className="fy-suggestion">
      <span>{session.suggestedDraft}</span>
      <button className="pill" type="button" onClick={() => {
        const text = session.suggestedDraft!
        session.suggestedDraft = undefined
        const suggestedAction = activeAction(text, session.commands)
        if (suggestedAction && !suggestedAction.argument.trim()) choose(suggestedAction.command.name)
        else { session.draft = [session.draft, text].filter(Boolean).join('\n'); session.changed(); input.current?.focus() }
      }}>Insert suggestion</button>
      <button className="fy-icon" type="button" aria-label="Dismiss suggestion" onClick={() => {
        session.suggestedDraft = undefined; session.changed()
      }}><ResearchIcon name="x" /></button></div>}
    {visible && <section className="fy-command-palette" data-browsing={browse || undefined} aria-label="Choose a research action" onBlur={e => {
      if (e.relatedTarget && !e.currentTarget.contains(e.relatedTarget) && e.relatedTarget !== input.current) closeList(false)
    }}>
      {browse && <div className="fy-filter"><ResearchIcon name="magnifying-glass" /><input autoFocus aria-label="Find a research action"
        placeholder="Find a workflow, skill or command…" value={filter} onKeyDown={keys} onChange={e => { setFilter(e.target.value); setIndex(0) }} />
        <button type="button" className="fy-icon" aria-label="Close actions" onClick={() => closeList()}><ResearchIcon name="x" /></button></div>}
      <div id={listId} role="listbox" aria-label="Commands and skills" className="fy-command-list" ref={results}>
        {commands.map((c,i) => <button key={c.name} id={listId + '-' + i} role="option" type="button" aria-selected={i === selected}
          onMouseDown={e => e.preventDefault()} onClick={() => pick(c.name)}>
          {browse && !query && (i === 0 || actionGroup(c) !== actionGroup(commands[i - 1])) && <small className="fy-group-name">{actionGroup(c)}</small>}
          <span className="fy-action-name">{actionLabel(c.name)} <code>/{c.name}</code></span>
          {c.description && <small>{c.description}</small>}</button>)}
        {!commands.length && <p>No matching actions.</p>}
      </div>
    </section>}
    <form className="fy-input-card" onSubmit={e => { e.preventDefault(); if (canSend) void session.send(documentID, edit ? doc : undefined) }}>
      {(session.sources.length > 0 || edit || action) && <div className="fy-attachments">
        {action && <button type="button" aria-label={'Remove ' + actionLabel(action.command.name)} title={action.command.description}
          onClick={() => { session.draft = argument; session.changed(); input.current?.focus() }}><ResearchIcon name="sparkle" />{actionLabel(action.command.name)} ×</button>}
        {edit && <button type="button" onClick={() => setEdit(false)} title="Remove edit target"><ResearchIcon name="pencil-simple" />{doc?.path.split('/').at(-1)} ×</button>}
        {session.sources.map(path => <button type="button" key={path} title={'Remove ' + path}
          onClick={() => { session.sources = session.sources.filter(s => s !== path); session.changed() }}><ResearchIcon name="paperclip" />{path.split('/').at(-1)} ×</button>)}
      </div>}
      {action && choices && (!argument || choices.includes(argument)) ? <select autoFocus aria-label={actionLabel(action.command.name)} value={argument}
        onChange={event => setArgument(event.target.value)}>
        <option value="">{action.command.name === 'autocompact' ? 'Toggle current setting' : 'Show current setting'}</option>
        {choices.map(value => <option key={value} value={value}>{value === 'one-at-a-time' ? 'One at a time' : value.charAt(0).toUpperCase() + value.slice(1)}</option>)}
      </select> : <textarea ref={input} aria-label="Research prompt" rows={2}
        placeholder={action ? action.command.input?.hint || 'Add a topic or instructions…' : 'Ask Feynman…'} value={argument}
        aria-autocomplete="list" aria-controls={visible ? listId : undefined} aria-activedescendant={visible && selected >= 0 ? listId + '-' + selected : undefined}
        onChange={e => setArgument(e.target.value)}
        onKeyDown={e => {
          if (e.nativeEvent.isComposing || e.keyCode === 229 || keys(e)) return
          if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); e.currentTarget.form?.requestSubmit() }
        }} />}
      <div className="fy-compose-actions">
        <button className="fy-icon" type="button" aria-label="Attach source files" title="Attach source files" disabled={locked} onClick={() => void session.attach()}><ResearchIcon name="paperclip" /></button>
        <button className="fy-actions-trigger" type="button" aria-label="Research actions" title="Workflows, skills and commands" aria-expanded={visible}
          disabled={!session.commands.length} onClick={() => { if (visible) closeList(); else { setBrowse(true); setFilter(''); setIndex(0) } }}><ResearchIcon name="squares-four" />Actions</button>
        {doc?.path && <label className={'fy-edit-toggle' + (edit ? ' selected' : '')} title={'Edit ' + doc.path.split('/').at(-1)}>
          <input type="checkbox" aria-label="Edit this document" checked={edit} disabled={busy || !!session.proposal}
            onChange={e => setEdit(e.target.checked)} /><ResearchIcon name="note-pencil" /><span>Edit document</span></label>}
        <div className="fy-action-spacer" />
        {busy ? <button className="fy-send" type="button" aria-label="Cancel research" title="Cancel research"
          onClick={() => void ResearchHost.cancel(session.root).catch(report)}><ResearchIcon name="stop" /></button>
          : <button className="fy-send" disabled={!canSend} aria-label="Send" title="Send · Enter"><ResearchIcon name="arrow-up" /><span className="fy-sr">Send</span></button>}
      </div>
    </form>
    {session.config.length > 0 && <div className="fy-configs">{session.config.map(option =>
      <ConfigPicker key={option.id} option={option} session={session} />)}
      {session.usage && <meter min={0} max={session.usage.size} value={session.usage.used} title={session.usage.used + ' / ' + session.usage.size + ' tokens'} aria-label="Context used" />}
    </div>}
  </div>
}
