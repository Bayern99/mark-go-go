import { describe, expect, it } from 'vitest'
import type { MarkdownEditor } from '../src/editor/createEditor'
import { segmentMarkdown } from '../src/editor/blocks/segment'
import { mountEditor } from './helpers/editor'
import editFixture from './fixtures/edit-blocks.md?raw'

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
  view.dispatch(view.state.tr.replaceWith(from, from + node.content.size, view.state.schema.text(newText)))
}

describe('block-preserving compose', () => {
  it('changes only the edited block', async () => {
    const mounted = await mountEditor(editFixture)
    try {
      replaceParagraph(mounted.editor, 'First paragraph.', 'First paragraph, edited.')
      const composed = mounted.editor.getMarkdown()

      expect(composed).toBe(
        '# Notes\n\nFirst paragraph, edited.\n\nSecond paragraph stays.\n\n- item one\n- item two\n',
      )

      const before = segmentMarkdown(editFixture).segments
      const after = segmentMarkdown(composed).segments
      expect(after.length).toBe(before.length)
      for (let i = 0; i < before.length; i += 1) {
        if (i === 1) {
          expect(after[i].source).not.toBe(before[i].source)
        } else {
          expect(after[i].source).toBe(before[i].source)
        }
      }
    } finally {
      await mounted.cleanup()
    }
  })

  it('is idempotent after an edit', async () => {
    const mounted = await mountEditor(editFixture)
    let composed = ''
    try {
      replaceParagraph(mounted.editor, 'Second paragraph stays.', 'Second paragraph, edited.')
      composed = mounted.editor.getMarkdown()
    } finally {
      await mounted.cleanup()
    }

    const reloaded = await mountEditor(composed)
    try {
      expect(reloaded.editor.getMarkdown()).toBe(composed)
    } finally {
      await reloaded.cleanup()
    }
  })

  it('keeps a locked alert block untouched', async () => {
    const source = 'Intro.\n\n> [!WARNING]\n> stay locked\n\nOutro.\n'
    const mounted = await mountEditor(source)
    try {
      const locked = mounted.root.querySelectorAll('[data-locked-block]')
      expect(locked.length).toBe(1)
      expect(locked[0].getAttribute('data-kind')).toBe('github-alert')
      expect(mounted.editor.getMarkdown()).toBe(source)
    } finally {
      await mounted.cleanup()
    }
  })
})

describe('empty documents', () => {
  it('loads and composes an empty document without throwing', async () => {
    const mounted = await mountEditor('')
    try {
      expect(mounted.editor.getMarkdown()).toBe('')
    } finally {
      await mounted.cleanup()
    }
  })

  it('preserves a whitespace-only document', async () => {
    const mounted = await mountEditor('\n\n')
    try {
      expect(mounted.editor.getMarkdown()).toBe('\n\n')
    } finally {
      await mounted.cleanup()
    }
  })
})
