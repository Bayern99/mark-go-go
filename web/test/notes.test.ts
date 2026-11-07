import { undo as undoSource } from '@codemirror/commands'
import { undo as undoRich } from '@milkdown/kit/prose/history'
import { userEvent } from 'vitest/browser'
import { describe, expect, it, vi } from 'vitest'
import { noteBlock, parseNote, withNoteText } from '../src/markdown/notes'
import { mountEditor, type MountedEditor } from './helpers/editor'
import { getSourceView, visibleSource } from './helpers/source'

function undo(mounted: MountedEditor): void {
  const view = mounted.editor.getView()
  undoRich(view.state, view.dispatch)
}

function noteElements(mounted: MountedEditor): HTMLElement[] {
  return Array.from(mounted.root.querySelectorAll<HTMLElement>('[data-note]'))
}

function noteText(element: HTMLElement): string {
  return element.querySelector('.mgo-note-text')?.textContent ?? ''
}

async function openDialog(): Promise<HTMLDialogElement> {
  return vi.waitFor(() => {
    const dialog = document.querySelector<HTMLDialogElement>('dialog.mgo-note-dialog')
    if (!dialog) throw new Error('no note dialog')
    return dialog
  })
}

async function answerDialog(text: string): Promise<void> {
  const dialog = await openDialog()
  const input = dialog.querySelector('textarea')
  const save = dialog.querySelector<HTMLButtonElement>('button[type="submit"]')
  if (!input || !save) throw new Error('incomplete note dialog')
  await userEvent.fill(input, text)
  await userEvent.click(save)
  await vi.waitFor(() => expect(document.querySelector('dialog.mgo-note-dialog')).toBeNull())
}

async function clickNoteAction(mounted: MountedEditor, index: number, action: 'edit' | 'delete'): Promise<void> {
  const button = noteElements(mounted)[index]?.querySelector<HTMLButtonElement>(`[data-note-action="${action}"]`)
  if (!button) throw new Error(`no ${action} button on note ${index}`)
  await userEvent.click(button)
}

describe('note format', () => {
  it('reads and writes the text, attributes kept', () => {
    expect(parseNote('<!-- note: Hello -->')?.text).toBe('Hello')
    expect(noteBlock('Hello')).toBe('<!-- note: Hello -->')
    expect(withNoteText('<!-- note author="jerry" id="7": old -->', 'new')).toBe(
      '<!-- note author="jerry" id="7": new -->',
    )
  })

  it('escapes what would end the comment, and reads it back unchanged', () => {
    const cases: Array<[text: string, written: string]> = [
      ['a --> b', '<!-- note: a --\\> b -->'],
      ['a --!> b', '<!-- note: a --\\!> b -->'],
      ['literal --\\> stays', '<!-- note: literal --\\\\> stays -->'],
      ['---> x -->', '<!-- note: ---\\> x --\\> -->'],
      ['ends with --', '<!-- note: ends with -- -->'],
      ['two\n\nlines', '<!-- note: two\n\nlines -->'],
    ]
    for (const [text, written] of cases) {
      expect(noteBlock(text)).toBe(written)
      expect(parseNote(written)?.text).toBe(text)
      // The only comment end in it is its own.
      expect(written.slice(0, -3)).not.toMatch(/--!?>/)
    }
  })

  it('takes only a whole note block for a note', () => {
    for (const source of [
      '<!-- a comment -->',
      '<!-- note that this one is prose: really -->',
      '<!-- notes: nope -->',
      '<!-- note: a --> trailing',
      '<!-- note: a --> <!-- note: b -->',
      '<div><!-- note: a --></div>',
    ]) {
      expect(parseNote(source)).toBeNull()
    }
  })
})

describe('notes in rich text', () => {
  it('shows a note as a note, and a plain comment or a fence as what they are', async () => {
    const source = 'One.\n\n<!-- note: Mine -->\n\n<!-- plain -->\n\n```md\n<!-- note: fenced -->\n```\n'
    const mounted = await mountEditor(source)
    try {
      expect(noteElements(mounted).map(noteText)).toEqual(['Mine'])
      expect(mounted.root.querySelector('[data-kind="raw-html"]')?.textContent).toBe('<!-- plain -->')
      expect(mounted.editor.getMarkdown()).toBe(source)
    } finally {
      await mounted.cleanup()
    }
  })

  it('edits one note: only its text changes, and undo restores every byte', async () => {
    const source =
      '# T\r\n\r\nOne.\n\n\n<!-- note author="jerry" id="7":   old   -->\n\n\n\nTwo.\r\n\r\n<!-- note: keep  -->\n'
    const mounted = await mountEditor(source)
    try {
      await clickNoteAction(mounted, 0, 'edit')
      const dialog = await openDialog()
      expect(dialog.querySelector('textarea')?.value).toBe('old')
      await answerDialog('new --> text')

      const edited =
        '# T\r\n\r\nOne.\n\n\n<!-- note author="jerry" id="7": new --\\> text -->\n\n\n\nTwo.\r\n\r\n<!-- note: keep  -->\n'
      expect(mounted.editor.getMarkdown()).toBe(edited)
      expect(noteElements(mounted).map(noteText)).toEqual(['new --> text', 'keep'])

      undo(mounted)
      expect(mounted.editor.getMarkdown()).toBe(source)
    } finally {
      await mounted.cleanup()
    }
  })

  it('deletes a note with its gap, and undo restores every byte', async () => {
    const source = 'One.\n\n\n<!-- note: gone -->\r\n\r\nTwo.\n\n<!-- note: last -->'
    const mounted = await mountEditor(source)
    try {
      await clickNoteAction(mounted, 0, 'delete')
      expect(mounted.editor.getMarkdown()).toBe('One.\r\n\r\nTwo.\n\n<!-- note: last -->')

      await clickNoteAction(mounted, 0, 'delete')
      expect(mounted.editor.getMarkdown()).toBe('One.\r\n\r\nTwo.')

      undo(mounted)
      undo(mounted)
      expect(mounted.editor.getMarkdown()).toBe(source)
    } finally {
      await mounted.cleanup()
    }
  })

  it('adds a note from the block menu, right after the block it was opened for', async () => {
    const mounted = await mountEditor('One.\n\nTwo.\n')
    try {
      const first = mounted.root.querySelector('.ProseMirror p')
      if (!first) throw new Error('no paragraph')
      await userEvent.click(first)
      await userEvent.keyboard('{End}{Enter}/')
      const item = await vi.waitFor(() => {
        const found = Array.from(document.querySelectorAll<HTMLElement>('.milkdown-slash-menu li')).find(
          (candidate) => candidate.textContent?.includes('Add note'),
        )
        if (!found) throw new Error('no Add note item')
        return found
      })
      await userEvent.click(item)
      await answerDialog('From the menu')

      expect(mounted.editor.getMarkdown()).toBe('One.\n\n<!-- note: From the menu -->\n\nTwo.\n')
      expect(noteElements(mounted).map(noteText)).toEqual(['From the menu'])
    } finally {
      await mounted.cleanup()
    }
  })

  it('leaves nothing behind when adding from the block menu is cancelled', async () => {
    const mounted = await mountEditor('One.\n\nTwo.\n')
    try {
      const first = mounted.root.querySelector('.ProseMirror p')
      if (!first) throw new Error('no paragraph')
      await userEvent.click(first)
      await userEvent.keyboard('{End}{Enter}/')
      const item = await vi.waitFor(() => {
        const found = Array.from(document.querySelectorAll<HTMLElement>('.milkdown-slash-menu li')).find(
          (candidate) => candidate.textContent?.includes('Add note'),
        )
        if (!found) throw new Error('no Add note item')
        return found
      })
      await userEvent.click(item)
      const dialog = await openDialog()
      const cancel = Array.from(dialog.querySelectorAll('button')).find((b) => b.textContent === 'Cancel')
      if (!cancel) throw new Error('no Cancel')
      await userEvent.click(cancel)
      await vi.waitFor(() => expect(document.querySelector('dialog.mgo-note-dialog')).toBeNull())

      await vi.waitFor(() => expect(mounted.editor.getMarkdown()).toBe('One.\n\nTwo.\n'))
    } finally {
      await mounted.cleanup()
    }
  })
})

describe('adding a note at a source offset', () => {
  const SOURCE = '# A\n\nBody.\n\n\n# B\r\n\r\nOther.'

  it('rich text: ends the text before the offset, as an undoable edit, every other byte kept', async () => {
    const mounted = await mountEditor(SOURCE)
    try {
      expect(mounted.editor.addNote(SOURCE.indexOf('# B'), 'First')).toBe(true)
      expect(mounted.editor.getMarkdown()).toBe(
        '# A\n\nBody.\n\n<!-- note: First -->\n\n\n# B\r\n\r\nOther.',
      )

      const end = mounted.editor.getMarkdown().length
      expect(mounted.editor.addNote(end, 'At the very end')).toBe(true)
      expect(mounted.editor.getMarkdown()).toBe(
        '# A\n\nBody.\n\n<!-- note: First -->\n\n\n# B\r\n\r\nOther.\n\n<!-- note: At the very end -->',
      )

      undo(mounted)
      undo(mounted)
      expect(mounted.editor.getMarkdown()).toBe(SOURCE)
    } finally {
      await mounted.cleanup()
    }
  })

  it('source: writes the same bytes as rich text, shows them raw, and undoes', async () => {
    const mounted = await mountEditor(SOURCE)
    try {
      mounted.editor.setMode('source')
      expect(mounted.editor.addNote(SOURCE.indexOf('# B'), 'First')).toBe(true)
      expect(mounted.editor.getMarkdown()).toBe(
        '# A\n\nBody.\n\n<!-- note: First -->\n\n\n# B\r\n\r\nOther.',
      )
      expect(visibleSource(mounted.root)).toContain('<!-- note: First -->')

      const view = getSourceView(mounted.root)
      undoSource(view)
      expect(mounted.editor.getMarkdown()).toBe(SOURCE)
    } finally {
      await mounted.cleanup()
    }
  })

  it('does not add to a read-only document', async () => {
    const mounted = await mountEditor(SOURCE)
    try {
      mounted.editor.setReadonly(true)
      expect(mounted.editor.addNote(SOURCE.length, 'Nope')).toBe(false)
      expect(mounted.editor.getMarkdown()).toBe(SOURCE)
    } finally {
      await mounted.cleanup()
    }
  })
})
