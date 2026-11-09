import '../src/styles.css'
import { describe, expect, it, vi } from 'vitest'
import { mountEditor } from './helpers/editor'

const DOCUMENT = `# Heading

Body.

<!-- note: 判断原文已改时不算批注 -->
`

describe('Portrait notes', () => {
  it('shows note secondary actions on keyboard focus, not only hover', async () => {
    const editor = await mountEditor(DOCUMENT)
    try {
      const note = editor.root.querySelector<HTMLElement>('.mgo-note')
      const actions = note?.querySelector<HTMLElement>('.mgo-note-actions')
      const edit = note?.querySelector<HTMLButtonElement>('[data-note-action="edit"]')
      if (!note || !actions || !edit) throw new Error('note actions missing')
      expect(getComputedStyle(actions).opacity).toBe('0')
      edit.focus()
      await vi.waitFor(() => expect(getComputedStyle(actions).opacity).toBe('1'))
    } finally {
      await editor.cleanup()
    }
  })
})
