import { trackComposerHistory } from './composerHistory'

export interface ComposerMount {
  anchor?: Element | null
  container?: Element | null
  mount?: (composer: HTMLElement) => (() => void) | void
}

export class ComposerScope {
  private cancelActive: (() => void) | null = null

  cancel(): void {
    this.cancelActive?.()
  }

  track(cancel: (() => void) | null): void {
    this.cancelActive = cancel
  }
}

export interface NoteComposerOptions extends ComposerMount {
  scope: ComposerScope
}

export interface MarkdownComposerOptions extends ComposerMount {
  label?: string
  scope: ComposerScope
}

export function autoGrowTextarea(input: HTMLTextAreaElement): void {
  input.style.height = 'auto'
  input.style.height = `${Math.max(38, input.scrollHeight)}px`
}

interface OpenComposerConfig extends ComposerMount {
  initial: string
  className: string
  ariaLabel: string
  inputLabel: string
  placeholder: string
  allowEmpty: boolean
  trim: boolean
  restoreNewlines?: boolean
  scope: ComposerScope
}

function openComposer(config: OpenComposerConfig): Promise<string | null> {
  config.scope.cancel()

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

    const history = trackComposerHistory(input)
    let closed = false
    let unmount: (() => void) | void

    const finish = (result: string | null) => {
      if (closed) return
      closed = true
      history.dispose()
      config.scope.track(null)
      if (unmount) unmount()
      composer.remove()
      resolve(result)
    }

    const cancelCurrent = () => finish(null)
    config.scope.track(cancelCurrent)

    const valueOf = () => {
      const raw = config.trim ? input.value.trim() : input.value
      return config.restoreNewlines ? restoreNewlines(config.initial, raw) : raw
    }

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

export function askForNote(initial: string, options: NoteComposerOptions): Promise<string | null> {
  return openComposer({
    ...options,
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

export function askForMarkdown(
  initial: string,
  options: MarkdownComposerOptions,
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
    restoreNewlines: true,
  })
}

function restoreNewlines(original: string, typed: string): string {
  const originalLines = original.split(/\r\n|\n|\r/)
  const typedLines = typed.split(/\n/)
  const endings: string[] = []
  const ending = /\r\n|\n|\r/g
  let match: RegExpExecArray | null
  while ((match = ending.exec(original))) endings.push(match[0])
  let out = ''
  const count = Math.max(originalLines.length, typedLines.length)
  for (let i = 0; i < count; i += 1) {
    if (i > 0) out += endings[i - 1] ?? (endings[endings.length - 1] ?? '\n')
    if (i < typedLines.length) out += typedLines[i]
  }
  return out
}
