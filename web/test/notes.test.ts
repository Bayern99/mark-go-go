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

async function openComposer(mounted: MountedEditor): Promise<HTMLElement> {
  return vi.waitFor(() => {
    expect(document.querySelector('dialog')).toBeNull()
    const composer = mounted.root.querySelector<HTMLElement>(
      '.ProseMirror .composer.mgo-note-composer[role="region"][aria-label="Note composer"]',
    )
    if (!composer) throw new Error('no inline note composer')
    return composer
  })
}

async function answerComposer(
  mounted: MountedEditor,
  text: string,
  commit: 'click' | 'shortcut' = 'click',
): Promise<void> {
  const composer = await openComposer(mounted)
  const input = composer.querySelector<HTMLTextAreaElement>('textarea[aria-label="Note"]')
  const save = Array.from(composer.querySelectorAll<HTMLButtonElement>('.composer-foot button')).find(
    (b) => b.textContent === 'Save',
  )
  const hint = composer.querySelector('.composer-foot .composer-hint')
  if (!input || !save || !hint) throw new Error('incomplete inline note composer')
  expect(hint.textContent).toContain('⌘↩ to save · Esc to cancel')
  await userEvent.fill(input, text)
  if (commit === 'shortcut') {
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', metaKey: true, bubbles: true, cancelable: true }))
  } else {
    await userEvent.click(save)
  }
  await vi.waitFor(() => expect(mounted.root.querySelector('.mgo-note-composer')).toBeNull())
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

  it('edits one note in-place: only its text changes, and undo restores every byte', async () => {
    const source =
      '# T\r\n\r\nOne.\n\n\n<!-- note author="jerry" id="7":   old   -->\n\n\n\nTwo.\r\n\r\n<!-- note: keep  -->\n'
    const mounted = await mountEditor(source)
    try {
      await clickNoteAction(mounted, 0, 'edit')
      const composer = await openComposer(mounted)
      // Anchored directly after the first note element inside the editor DOM.
      expect(noteElements(mounted)[0]?.nextElementSibling).toBe(composer)
      expect(composer.querySelector<HTMLTextAreaElement>('textarea[aria-label="Note"]')?.value).toBe('old')
      await answerComposer(mounted, 'new --> text', 'shortcut')

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

  it('adds a note from the block menu via in-place composer, right after the block it was opened for', async () => {
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
      await answerComposer(mounted, 'From the menu', 'click')

      expect(mounted.editor.getMarkdown()).toBe('One.\n\n<!-- note: From the menu -->\n\nTwo.\n')
      expect(noteElements(mounted).map(noteText)).toEqual(['From the menu'])
    } finally {
      await mounted.cleanup()
    }
  })

  it('leaves nothing behind when adding from the block menu is cancelled by Escape or Cancel button', async () => {
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
      const composer = await openComposer(mounted)
      const input = composer.querySelector<HTMLTextAreaElement>('textarea[aria-label="Note"]')
      if (!input) throw new Error('no Note textarea')
      await userEvent.fill(input, 'Line 1\nLine 2\nLine 3\nLine 4')
      expect(Number.parseFloat(input.style.height)).toBeGreaterThanOrEqual(40)
      input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }))
      await vi.waitFor(() => expect(mounted.root.querySelector('.mgo-note-composer')).toBeNull())
      await vi.waitFor(() => expect(mounted.editor.getMarkdown()).toBe('One.\n\nTwo.\n'))

      // Also verify cancelling via the Cancel button on an existing note edit
      mounted.editor.setMarkdown('One.\n\n<!-- note: keep -->\n')
      await clickNoteAction(mounted, 0, 'edit')
      const editComposer = await openComposer(mounted)
      const cancelBtn = Array.from(editComposer.querySelectorAll('button')).find((b) => b.textContent === 'Cancel')
      if (!cancelBtn) throw new Error('no Cancel button')
      await userEvent.click(cancelBtn)
      await vi.waitFor(() => expect(mounted.root.querySelector('.mgo-note-composer')).toBeNull())
      expect(mounted.editor.getMarkdown()).toBe('One.\n\n<!-- note: keep -->\n')
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
