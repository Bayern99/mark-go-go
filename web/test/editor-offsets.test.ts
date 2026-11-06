import { describe, expect, it } from 'vitest'
import type { MarkdownEditor } from '../src/editor/createEditor'
import { mountEditor } from './helpers/editor'

function reportedOffsets(editor: MarkdownEditor): number[] {
  const offsets: number[] = []
  editor.onSelectionChange((offset) => offsets.push(offset))
  return offsets
}

function replaceParagraph(editor: MarkdownEditor, oldText: string, newText: string): void {
  const view = editor.getView()
  let position = -1
  view.state.doc.descendants((node, pos) => {
    if (position !== -1) return false
    if (node.type.name !== 'paragraph') return true
    if (!node.textContent.includes(oldText)) return true
    position = pos
    return false
  })
  if (position === -1) throw new Error(`paragraph not found: ${oldText}`)
  const node = view.state.doc.nodeAt(position)
  if (!node) throw new Error('paragraph node missing')
  const from = position + 1
  view.dispatch(
    view.state.tr.replaceWith(from, from + node.content.size, view.state.schema.text(newText)),
  )
}

describe('Markdown source offsets', () => {
  it('reveals the start of the top-level block containing an offset', async () => {
    const source = '# Heading\n\nFirst paragraph.\n\nSecond paragraph.\n'
    const mounted = await mountEditor(source)
    try {
      const offsets = reportedOffsets(mounted.editor)
      const secondStart = source.indexOf('Second paragraph.')

      expect(mounted.editor.reveal(secondStart + 5)).toBe(true)
      expect(offsets.at(-1)).toBe(secondStart)
    } finally {
      await mounted.cleanup()
    }
  })

  it('maps offsets inside a locked block to that block', async () => {
    const source = 'Intro.\n\n> [!WARNING]\n> stay locked\n\nOutro.\n'
    const mounted = await mountEditor(source)
    try {
      const offsets = reportedOffsets(mounted.editor)
      const lockedStart = source.indexOf('> [!WARNING]')

      expect(mounted.editor.reveal(lockedStart + 8)).toBe(true)
      expect(offsets.at(-1)).toBe(lockedStart)
    } finally {
      await mounted.cleanup()
    }
  })

  it('recomputes block ranges after an edit before the cursor', async () => {
    const source = 'First paragraph.\n\nSecond paragraph.\n'
    const mounted = await mountEditor(source)
    try {
      replaceParagraph(mounted.editor, 'First paragraph.', 'First paragraph, now much longer.')

      const composed = mounted.editor.getMarkdown()
      const secondStart = composed.indexOf('Second paragraph.')
      expect(secondStart).toBeGreaterThan(source.indexOf('Second paragraph.'))

      const offsets = reportedOffsets(mounted.editor)
      expect(mounted.editor.reveal(secondStart + 4)).toBe(true)
      expect(offsets.at(-1)).toBe(secondStart)
    } finally {
      await mounted.cleanup()
    }
  })

  it('reports the moved offset when an edit before the cursor shifts the block', async () => {
    const source = 'First paragraph.\n\nSecond paragraph.\n'
    const mounted = await mountEditor(source)
    try {
      const offsets = reportedOffsets(mounted.editor)
      const originalSecond = source.indexOf('Second paragraph.')
      expect(mounted.editor.reveal(originalSecond)).toBe(true)
      expect(offsets.at(-1)).toBe(originalSecond)

      replaceParagraph(mounted.editor, 'First paragraph.', 'First paragraph, now much longer.')

      const composed = mounted.editor.getMarkdown()
      const movedSecond = composed.indexOf('Second paragraph.')
      expect(movedSecond).toBeGreaterThan(originalSecond)
      expect(offsets.at(-1)).toBe(movedSecond)
    } finally {
      await mounted.cleanup()
    }
  })

  it('reveals inside a block that was edited after load', async () => {
    const source = 'First paragraph.\n\nSecond paragraph.\n'
    const mounted = await mountEditor(source)
    try {
      replaceParagraph(mounted.editor, 'First paragraph.', 'First paragraph, now much longer.')
      const offsets = reportedOffsets(mounted.editor)

      const composed = mounted.editor.getMarkdown()
      expect(mounted.editor.reveal(composed.indexOf('much longer') + 2)).toBe(true)
      expect(offsets.at(-1)).toBe(0)
    } finally {
      await mounted.cleanup()
    }
  })

  it('returns false for offsets outside the composed text', async () => {
    const mounted = await mountEditor('Only paragraph.\n')
    try {
      expect(mounted.editor.reveal(-1)).toBe(false)
      expect(mounted.editor.reveal(10_000)).toBe(false)
    } finally {
      await mounted.cleanup()
    }
  })
})
