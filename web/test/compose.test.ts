import { describe, expect, it } from 'vitest'
import { segmentMarkdown } from '../src/editor/blocks/segment'
import { mountEditor, replaceParagraph } from './helpers/editor'
import editFixture from './fixtures/edit-blocks.md?raw'


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
      const locked = mounted.root.querySelectorAll('[data-locked-block][data-kind="github-alert"]')
      expect(locked.length).toBe(1)
      expect(locked[0].classList.contains('mgo-alert')).toBe(true)
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

  it('keeps leading whitespace when the first block is typed in', async () => {
    const source = '\n\n  \n'
    const mounted = await mountEditor(source)
    try {
      expect(mounted.editor.getMarkdown()).toBe(source)

      const view = mounted.editor.getView()
      view.dispatch(view.state.tr.insertText('Hello', 1))

      expect(mounted.editor.getMarkdown()).toBe(`${source}Hello\n`)

      const offsets: number[] = []
      mounted.editor.onSelectionChange((offset) => offsets.push(offset))
      expect(offsets[0]).toBe(source.length)

      expect(mounted.editor.reveal(source.length + 2)).toBe(true)
      expect(offsets.at(-1)).toBe(source.length)
    } finally {
      await mounted.cleanup()
    }
  })
})
