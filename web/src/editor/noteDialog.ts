/**
 * Asks the reader for a note's text in a modal dialog. Resolves to the text,
 * trimmed, or to null when they cancel. It lives outside the editor's DOM, so
 * it works the same from rich text and from a view that hides the editor.
 */
export function askForNote(initial = ''): Promise<string | null> {
  return new Promise((resolve) => {
    const dialog = document.createElement('dialog')
    dialog.className = 'mgo-note-dialog'
    dialog.setAttribute('aria-label', initial === '' ? 'Add note' : 'Edit note')

    const form = document.createElement('form')
    form.method = 'dialog'

    const input = document.createElement('textarea')
    input.className = 'mgo-note-input'
    input.rows = 4
    input.value = initial
    input.placeholder = 'Write a note. It is saved in the document, out of sight in other Markdown viewers.'
    input.setAttribute('aria-label', 'Note')

    const actions = document.createElement('div')
    actions.className = 'mgo-note-dialog-actions'
    const cancel = document.createElement('button')
    cancel.type = 'button'
    cancel.textContent = 'Cancel'
    const save = document.createElement('button')
    save.type = 'submit'
    save.textContent = 'Save note'
    actions.append(cancel, save)
    form.append(input, actions)
    dialog.append(form)

    let result: string | null = null
    const syncSave = () => {
      save.disabled = input.value.trim() === ''
    }
    const submit = () => {
      if (input.value.trim() === '') return
      result = input.value.trim()
      dialog.close()
    }
    input.addEventListener('input', syncSave)
    input.addEventListener('keydown', (event) => {
      if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
        event.preventDefault()
        submit()
      }
    })
    form.addEventListener('submit', (event) => {
      event.preventDefault()
      submit()
    })
    cancel.addEventListener('click', () => dialog.close())
    dialog.addEventListener('close', () => {
      dialog.remove()
      resolve(result)
    })

    syncSave()
    document.body.append(dialog)
    dialog.showModal()
    input.focus()
    input.setSelectionRange(input.value.length, input.value.length)
  })
}
