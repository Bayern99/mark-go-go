import { describe, expect, it } from 'vitest'
import { mountEditor } from './helpers/editor'

function endOfBlock(doc: { forEach(fn: (node: { textContent: string; nodeSize: number }, offset: number) => void): void }, text: string): number {
  let end = -1
  doc.forEach((node, offset) => {
    if (end === -1 && node.textContent.includes(text)) end = offset + node.nodeSize
  })
  if (end === -1) throw new Error(`no block holds ${text}`)
  return end
}

describe('blocks inserted or removed between others', () => {
  it('keeps the bytes between the blocks after an inserted one', async () => {
    const source = 'One.\r\n\r\nTwo.\n\n\n\nThree.\r\n\r\nFour.'
    const mounted = await mountEditor(source)
    try {
      const view = mounted.editor.getView()
      const paragraph = view.state.schema.nodes.paragraph.create(null, view.state.schema.text('New.'))
      view.dispatch(view.state.tr.insert(endOfBlock(view.state.doc, 'One.'), paragraph))

      expect(mounted.editor.getMarkdown()).toBe('One.\n\nNew.\r\n\r\nTwo.\n\n\n\nThree.\r\n\r\nFour.')
    } finally {
      await mounted.cleanup()
    }
  })

  it('keeps the bytes between the blocks after a removed one', async () => {
    const source = 'One.\n\nTwo.\r\n\r\nThree.\n\n\n\nFour.\n'
    const mounted = await mountEditor(source)
    try {
      const view = mounted.editor.getView()
      const end = endOfBlock(view.state.doc, 'Two.')
      view.dispatch(view.state.tr.delete(end - 6, end))

      expect(mounted.editor.getMarkdown()).toBe('One.\r\n\r\nThree.\n\n\n\nFour.\n')
    } finally {
      await mounted.cleanup()
    }
  })

  it('does not merge a block into a new one when its own gap has no blank line', async () => {
    const source = '# Title\n2. second\n'
    const mounted = await mountEditor(source)
    try {
      const view = mounted.editor.getView()
      const paragraph = view.state.schema.nodes.paragraph.create(null, view.state.schema.text('New.'))
      view.dispatch(view.state.tr.insert(endOfBlock(view.state.doc, 'Title'), paragraph))

      expect(mounted.editor.getMarkdown()).toBe('# Title\n\nNew.\n\n2. second\n')
    } finally {
      await mounted.cleanup()
    }
  })
})
