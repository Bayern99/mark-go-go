export interface ComposerMount {
  /** Block element after which the inline composer is placed when no custom mount is given. */
  anchor?: Element | null
  /** Fallback container when no custom mount is given. */
  container?: Element | null
  /** Custom mount hook (e.g. ProseMirror widget) returning an optional cleanup. */
  mount?: (composer: HTMLElement) => (() => void) | void
}

export type NoteComposerOptions = ComposerMount

export interface MarkdownComposerOptions extends ComposerMount {
  /** Accessible name of the region. */
  label?: string
}

let activeCancel: (() => void) | null = null

export function cancelNoteComposer(): void {
  activeCancel?.()
}

export const cancelComposer = cancelNoteComposer

export function autoGrowTextarea(input: HTMLTextAreaElement): void {
  input.style.height = 'auto'
  input.style.height = `${Math.max(38, input.scrollHeight)}px`
}

export const autoGrowNoteTextarea = autoGrowTextarea

interface OpenComposerConfig extends ComposerMount {
  initial: string
  className: string
  ariaLabel: string
  inputLabel: string
  placeholder: string
  allowEmpty: boolean
  trim: boolean
}

function openComposer(config: OpenComposerConfig): Promise<string | null> {
  cancelNoteComposer()

  return new Promise((resolve) => {
    const composer = document.createElement('div')
    composer.className = config.className
    composer.setAttribute('role', 'region')
    composer.setAttribute('aria-label', config.ariaLabel)
    composer.contentEditable = 'false'

    const input = document.createElement('textarea')
    input.className = 'mgo-note-input'
    input.rows = 2
    input.value = config.initial
    input.placeholder = config.placeholder
    input.setAttribute('aria-label', config.inputLabel)
    input.spellcheck = false

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

    const valueOf = () => (config.trim ? input.value.trim() : input.value)

    const syncSave = () => {
      save.disabled = config.allowEmpty ? false : valueOf() === ''
      autoGrowTextarea(input)
    }

    const submit = () => {
      const next = valueOf()
      if (!config.allowEmpty && next === '') return
      finish(next)
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

    if (config.mount) {
      unmount = config.mount(composer)
    } else if (config.anchor?.parentElement) {
      config.anchor.after(composer)
      unmount = () => composer.remove()
    } else {
      ;(config.container ?? document.body).append(composer)
      unmount = () => composer.remove()
    }

    syncSave()
    input.focus()
    input.setSelectionRange(input.value.length, input.value.length)
  })
}

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
  const opts: NoteComposerOptions =
    options instanceof Element ? { anchor: options } : (options ?? {})
  return openComposer({
    ...opts,
    initial,
    className: 'composer mgo-note-composer',
    ariaLabel: 'Note composer',
    inputLabel: 'Note',
    placeholder:
      'Write a note. It is saved in the document, out of sight in other Markdown viewers.',
    allowEmpty: false,
    trim: true,
  })
}

/**
 * Opens the same paper composer for a special block's raw Markdown. The saved
 * string is the textarea contents as typed, including whitespace; cancel returns
 * null and writes nothing.
 */
export function askForMarkdown(
  initial: string,
  options: MarkdownComposerOptions = {},
): Promise<string | null> {
  return openComposer({
    ...options,
    initial,
    className: 'composer mgo-block-composer',
    ariaLabel: options.label ?? 'Markdown composer',
    inputLabel: 'Markdown',
    placeholder: 'Edit this block as Markdown.',
    allowEmpty: true,
    trim: false,
  })
}
