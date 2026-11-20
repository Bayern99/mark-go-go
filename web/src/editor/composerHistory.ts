import { history, isolateHistory, redo, redoDepth, undo, undoDepth } from '@codemirror/commands'
import { EditorState, Transaction } from '@codemirror/state'
import type { Disposable } from '../host/types'
import { registerNativeField, type NativeEditCommand } from './nativeEdit'

function selection(input: HTMLTextAreaElement) {
  const { selectionStart: start, selectionEnd: end, selectionDirection: direction } = input
  return direction === 'backward' ? { anchor: end, head: start } : { anchor: start, head: end }
}

function changedRange(before: string, after: string) {
  let from = 0
  while (from < before.length && from < after.length && before[from] === after[from]) from += 1
  let to = before.length
  let end = after.length
  while (to > from && end > from && before[to - 1] === after[end - 1]) {
    to -= 1
    end -= 1
  }
  return { from, to, insert: after.slice(from, end) }
}

export function trackComposerHistory(input: HTMLTextAreaElement): Disposable {
  let state = EditorState.create({ doc: input.value, extensions: [history()] })
  let continuingComposition = false
  const events = new AbortController()
  const options = { signal: events.signal }
  const perform = (command: NativeEditCommand) => {
    if (input.disabled || input.readOnly) return false
    return (command === 'undo' ? undo : redo)({
      state,
      dispatch(transaction) {
        state = transaction.state
        input.value = state.doc.toString()
        const { from, to, anchor, head } = state.selection.main
        input.setSelectionRange(from, to, anchor > head ? 'backward' : 'forward')
        input.dispatchEvent(new Event('input', { bubbles: true }))
      },
    })
  }
  const registration = registerNativeField(input, {
    capabilities: () => ({ canUndo: undoDepth(state) > 0, canRedo: redoDepth(state) > 0 }),
    perform,
  })

  input.addEventListener('beforeinput', (event) => {
    if (event.inputType === 'historyUndo' || event.inputType === 'historyRedo') {
      if (event.cancelable) {
        event.preventDefault()
        perform(event.inputType === 'historyUndo' ? 'undo' : 'redo')
      }
      return
    }
    state = state.update({ selection: selection(input), annotations: Transaction.addToHistory.of(false) }).state
  }, options)

  input.addEventListener('input', (event) => {
    const before = state.doc.toString()
    if (before === input.value) return
    const inputType = event instanceof InputEvent ? event.inputType : ''
    let userEvent = 'input'
    let boundary: 'before' | 'full' | undefined = 'full'
    if (event instanceof InputEvent && (event.isComposing || inputType === 'insertFromComposition')) {
      userEvent = continuingComposition ? 'input.type.compose' : 'input.type.compose.start'
      boundary = continuingComposition ? undefined : 'before'
      continuingComposition = true
    } else {
      continuingComposition = false
      if (inputType === 'insertText' || inputType.startsWith('deleteContent')) {
        userEvent = inputType === 'insertText' ? 'input.type' : 'delete'
        boundary = undefined
      }
    }
    state = state.update({
      changes: changedRange(before, input.value),
      selection: selection(input),
      userEvent,
      annotations: boundary ? isolateHistory.of(boundary) : undefined,
    }).state
  }, options)

  input.addEventListener('compositionstart', () => { continuingComposition = false }, options)
  input.addEventListener('keydown', (event) => {
    if (event.altKey || event.isComposing || (!event.metaKey && !event.ctrlKey)) return
    const key = event.key.toLowerCase()
    if (key !== 'z' && !(key === 'y' && event.ctrlKey && !event.shiftKey)) return
    event.preventDefault()
    event.stopPropagation()
    perform(key === 'y' || event.shiftKey ? 'redo' : 'undo')
  }, options)

  return {
    dispose() {
      events.abort()
      registration.dispose()
    },
  }
}
