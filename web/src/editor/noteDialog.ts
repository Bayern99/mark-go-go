export interface NoteComposerOptions {
  /** Block element after which the inline composer is placed when no custom mount is given. */
  anchor?: Element | null
  /** Fallback container when no anchor element is given. */
  container?: Element | null
  /** Custom mount hook (e.g. ProseMirror widget decoration) returning an optional cleanup. */
  mount?: (composer: HTMLElement) => (() => void) | void
}

let activeCancel: (() => void) | null = null

export function cancelNoteComposer(): void {
  activeCancel?.()
}

export function autoGrowTextarea(input: HTMLTextAreaElement): void {
  input.style.height = 'auto'
  input.style.height = `${Math.min(160, Math.max(40, input.scrollHeight))}px`
}

export const autoGrowNoteTextarea = autoGrowTextarea

/**
 * Opens an in-place paper composer (`<div class="composer mgo-note-composer">`,
 * never a `<dialog>`) anchored right after the target block in the editor.
 * Resolves to the trimmed note text on save (`⌘↩` / `Ctrl+↩` / `Save`), or
 * `null` when cancelled (`Escape` / `Cancel`).
 */
export function askForNote(
  initial = '',
  options?: Element | NoteComposerOptions | null,
): Promise<string | null> {
  cancelNoteComposer()

  const opts: NoteComposerOptions =
    options instanceof Element ? { anchor: options } : (options ?? {})

  return new Promise((resolve) => {
    const composer = document.createElement('div')
    composer.className = 'composer mgo-note-composer'
    composer.setAttribute('role', 'region')
    composer.setAttribute('aria-label', 'Note composer')
    composer.contentEditable = 'false'

    const input = document.createElement('textarea')
    input.className = 'mgo-note-input'
    input.rows = 2
    input.value = initial
    input.placeholder =
      'Write a note. It is saved in the document, out of sight in other Markdown viewers.'
    input.setAttribute('aria-label', 'Note')

    const foot = document.createElement('div')
    foot.className = 'composer-foot composer-bar'

    const hint = document.createElement('span')
    hint.className = 'composer-hint hint meta'
    hint.textContent = '⌘↩ to save · Esc to cancel'

    const cancel = document.createElement('button')
    cancel.type = 'button'
    cancel.className = 'pill xs quiet'
    cancel.textContent = 'Cancel'

    const save = document.createElement('button')
    save.type = 'submit'
    save.className = 'pill xs primary'
    save.textContent = 'Save'

    foot.append(hint, cancel, save)
    composer.append(input, foot)

    let closed = false
    let unmount: (() => void) | void

    const finish = (result: string | null) => {
      if (closed) return
      closed = true
      if (activeCancel === cancelCurrent) activeCancel = null
      if (unmount) unmount()
      composer.remove()
      resolve(result)
    }

    const cancelCurrent = () => finish(null)
    activeCancel = cancelCurrent

    const syncSave = () => {
      save.disabled = input.value.trim() === ''
      autoGrowTextarea(input)
    }

    const submit = () => {
      const trimmed = input.value.trim()
      if (trimmed === '') return
      finish(trimmed)
    }

    input.addEventListener('input', syncSave)
    input.addEventListener('keydown', (event) => {
      if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
        event.preventDefault()
        event.stopPropagation()
        submit()
      } else if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        finish(null)
      }
    })
    save.addEventListener('click', (event) => {
      event.preventDefault()
      event.stopPropagation()
      submit()
    })
    cancel.addEventListener('click', (event) => {
      event.preventDefault()
      event.stopPropagation()
      finish(null)
    })

    if (opts.mount) {
      unmount = opts.mount(composer)
    } else if (opts.anchor?.parentElement) {
      opts.anchor.after(composer)
      unmount = () => composer.remove()
    } else {
      ;(opts.container ?? document.body).append(composer)
      unmount = () => composer.remove()
    }

    syncSave()
    input.focus()
    input.setSelectionRange(input.value.length, input.value.length)
  })
}
