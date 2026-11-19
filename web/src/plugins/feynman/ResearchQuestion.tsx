import { useState } from 'react'
import { ResearchHost, type ResearchUpdate } from '../../host/generated/mygo'
import { ResearchIcon } from './Conversation'
import type { ResearchSession } from './session'

export function ResearchQuestion({
  session,
  question,
  onError,
}: {
  session: ResearchSession
  question: ResearchUpdate
  onError: (error: unknown) => void
}) {
  const [pending, setPending] = useState(false)
  const [selectedChoice, setSelectedChoice] = useState('')
  const [filter, setFilter] = useState('')
  const { kind, requestId, options = [] } = question
  const title = question.text || question.title || 'Choose how Feynman should proceed.'
  const textEntry = kind === 'input' || kind === 'editor'
  const isConfirm = kind === 'confirm'
  const toolList =
    title === 'Tools' &&
    options.length > 0 &&
    options.every((option) => /^[\w.:-]+\s+—\s/.test(option.name))

  async function answer(value: string, cancel = false, useTool = false) {
    if (pending || !requestId) return
    setPending(true)
    try {
      if (textEntry) {
        await ResearchHost.answerText(session.root, requestId, value, cancel)
      } else if (isConfirm && typeof ResearchHost.answerConfirm === 'function') {
        await ResearchHost.answerConfirm(session.root, requestId, value === 'yes', cancel)
      } else {
        await ResearchHost.answer(session.root, requestId, cancel ? '' : value)
      }
      if (useTool) {
        const name = options.find((option) => option.id === value)?.name.split(' — ')[0]
        if (name) {
          session.draft = session.draft.trim()
            ? session.draft.trimEnd() + ' Use the ' + name + ' tool.'
            : 'Use the ' + name + ' tool to '
          session.changed()
        }
      }
    } catch (error) {
      onError(error)
      setPending(false)
    }
  }

  const visibleOptions = options.filter((option) =>
    option.name.toLowerCase().includes(filter.trim().toLowerCase()),
  )
  const useMenuList = !isConfirm && (toolList || options.length > 4)

  return (
    <section
      className="fy-permission"
      data-text-entry={textEntry || undefined}
      role="group"
      aria-label="Feynman confirmation"
    >
      <strong>
        <ResearchIcon name={textEntry ? 'pencil-simple' : isConfirm ? 'check-circle' : 'hand-palm'} />
        {title}
      </strong>
      {question.message && question.message !== title && <p className="fy-permission-msg">{question.message}</p>}
      {textEntry ? (
        <form
          onSubmit={(event) => {
            event.preventDefault()
            void answer(String(new FormData(event.currentTarget).get('reply') ?? ''))
          }}
        >
          {kind === 'editor' ? (
            <textarea
              autoFocus
              name="reply"
              aria-label={title}
              rows={3}
              defaultValue={question.prefill}
              placeholder={question.placeholder}
            />
          ) : (
            <input
              autoFocus
              name="reply"
              aria-label={title}
              autoComplete="off"
              defaultValue={question.prefill}
              placeholder={question.placeholder}
            />
          )}
          <div className="plugin-actions">
            <button className="pill primary" disabled={pending}>
              Submit
            </button>
            <button
              className="pill quiet"
              type="button"
              disabled={pending}
              onClick={() => void answer('', true)}
            >
              Cancel
            </button>
          </div>
        </form>
      ) : useMenuList ? (
        <div className="fy-choice-menu">
          {options.length > 6 && (
            <label className="fy-filter">
              <ResearchIcon name="magnifying-glass" />
              <input
                type="search"
                aria-label="Filter choices"
                placeholder="Filter choices…"
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
              />
            </label>
          )}
          <div className="fy-choice-list" role="listbox" aria-label="Feynman choices">
            {visibleOptions.map((option) => {
              const active = selectedChoice === option.id
              return (
                <button
                  key={option.id}
                  type="button"
                  role="option"
                  aria-selected={active}
                  className={`fy-choice-option${active ? ' selected' : ''}`}
                  disabled={pending}
                  onClick={() => setSelectedChoice(option.id)}
                >
                  <span>{option.name}</span>
                  {active && <ResearchIcon name="check" />}
                </button>
              )
            })}
            {!visibleOptions.length && <p className="fy-options-empty">No matching choices.</p>}
          </div>
          <div className="plugin-actions">
            <button
              type="button"
              className="pill primary"
              disabled={pending || !selectedChoice}
              onClick={() => void answer(selectedChoice, false, false)}
            >
              {toolList ? 'Show details' : 'Confirm'}
            </button>
            {toolList && (
              <button
                type="button"
                className="pill"
                disabled={pending || !selectedChoice}
                onClick={() => void answer(selectedChoice, false, true)}
              >
                Use in question
              </button>
            )}
            <button
              className="pill quiet"
              type="button"
              disabled={pending}
              onClick={() => void answer('', true)}
            >
              Cancel
            </button>
          </div>
        </div>
      ) : (
        <div className="plugin-actions">
          {options.map((option) => (
            <button
              type="button"
              key={option.id}
              className={`pill${option.id === 'yes' || option.kind === 'confirm' ? ' primary' : ''}`}
              disabled={pending}
              onClick={() => void answer(option.id)}
            >
              {option.name}
            </button>
          ))}
          {options.every((option) => option.kind === 'allow_once') && (
            <button
              className="pill quiet"
              type="button"
              disabled={pending}
              onClick={() => void answer('', true)}
            >
              Cancel
            </button>
          )}
        </div>
      )}
    </section>
  )
}
