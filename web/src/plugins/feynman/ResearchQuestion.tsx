import { useState } from 'react'
import { ResearchHost, type ResearchUpdate } from '../../host/generated/mygo'
import { ResearchIcon } from './Conversation'
import type { ResearchSession } from './session'

export function ResearchQuestion({ session, question, onError }: {
  session: ResearchSession
  question: ResearchUpdate
  onError: (error: unknown) => void
}) {
  const [pending, setPending] = useState(false)
  const { kind, requestId, options = [] } = question
  const title = question.text || 'Choose how Feynman should proceed.'
  const textEntry = kind === 'input' || kind === 'editor'
  const toolList = title === 'Tools' && options.length > 0 && options.every(option => /^[\w.:-]+\s+—\s/.test(option.name))
  async function answer(value: string, cancel = false, useTool = false) {
    if (pending || !requestId) return
    setPending(true)
    try {
      if (textEntry) await ResearchHost.answerText(session.root, requestId, value, cancel)
      else await ResearchHost.answer(session.root, requestId, cancel ? '' : value)
      if (useTool) {
        const name = options.find(option => option.id === value)?.name.split(' — ')[0]
        if (name) {
          session.draft = session.draft.trim() ? session.draft.trimEnd() + ' Use the ' + name + ' tool.' : 'Use the ' + name + ' tool to '
          session.changed()
        }
      }
    } catch (error) {
      onError(error)
      setPending(false)
    }
  }
  return <section className="fy-permission" data-text-entry={textEntry || undefined} role="group" aria-label="Feynman confirmation">
    <strong><ResearchIcon name={textEntry ? 'pencil-simple' : 'hand-palm'} />{title}</strong>
    {textEntry ? <form onSubmit={event => {
      event.preventDefault()
      void answer(String(new FormData(event.currentTarget).get('reply') ?? ''))
    }}>
      {kind === 'editor' ? <textarea autoFocus name="reply" aria-label={title} rows={3}
        defaultValue={question.prefill} placeholder={question.placeholder} /> : <input autoFocus name="reply" aria-label={title}
        autoComplete="off" defaultValue={question.prefill} placeholder={question.placeholder} />}
      <div className="plugin-actions">
        <button className="pill" disabled={pending}>Submit</button>
        <button className="pill quiet" type="button" disabled={pending} onClick={() => void answer('', true)}>Cancel</button>
      </div>
    </form> : toolList || options.length > 4 ? <form className="fy-choice-form" onSubmit={event => {
      event.preventDefault()
      const button = (event.nativeEvent as SubmitEvent).submitter as HTMLButtonElement | null
      void answer(String(new FormData(event.currentTarget).get('choice')), false, button?.value === 'use')
    }}>
      <select name="choice" required defaultValue="" aria-label="Feynman choices"><option value="" disabled>Choose an option…</option>
        {options.map(option => <option key={option.id} value={option.id}>{option.name}</option>)}</select>
      <button className="pill" disabled={pending}>{toolList ? 'Show details' : 'Confirm'}</button>
      {toolList && <button className="pill" value="use" disabled={pending}>Use in question</button>}
      <button className="pill quiet" type="button" disabled={pending} onClick={() => void answer('', true)}>Cancel</button>
    </form> : <div className="plugin-actions">{options.map(option =>
      <button type="button" key={option.id} className="pill" disabled={pending} onClick={() => void answer(option.id)}>{option.name}</button>)}
      {options.every(option => option.kind === 'allow_once') && <button className="pill quiet" type="button"
        disabled={pending} onClick={() => void answer('', true)}>Cancel</button>}</div>}
  </section>
}
