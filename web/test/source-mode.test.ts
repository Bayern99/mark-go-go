import { redo, undo } from '@codemirror/commands'
import { EditorState } from '@codemirror/state'
import { undo as undoPreview } from '@milkdown/kit/prose/history'
import { describe, expect, it } from 'vitest'
import { mountEditor, replaceParagraph } from './helpers/editor'
import { getSourceView, replaceSource, replaceSourceText } from './helpers/source'

const files = import.meta.glob('./fixtures/*.md', {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>


describe('source mode round trip', () => {
  const names = Object.keys(files).sort()

  for (const name of names) {
    it(name.split('/').pop() ?? name, async () => {
      const source = files[name]
      const mounted = await mountEditor(source)
      const changes: string[] = []
      mounted.editor.onChange((text) => changes.push(text))
      try {
        expect(mounted.editor.getMode()).toBe('preview')
        mounted.editor.setMode('source')
        expect(mounted.editor.getMode()).toBe('source')
        expect(getSourceView(mounted.root).state.doc.toString()).toBe(source)
        expect(mounted.editor.getMarkdown()).toBe(source)

        mounted.editor.setMode('preview')
        expect(mounted.editor.getMode()).toBe('preview')
        expect(mounted.editor.getMarkdown()).toBe(source)
        expect(changes).toEqual([])
      } finally {
        await mounted.cleanup()
      }
    })
  }
})

describe('source mode bytes', () => {
  const cases: Record<string, { source: string; search: string; replacement: string }> = {
    crlf: {
      source: '# CRLF title\r\n\r\nTarget line.\r\nSecond line.\r\n',
      search: 'Target line.',
      replacement: 'Target line, edited.',
    },
    mixed: {
      source: 'Alpha\r\nBeta\n\nGamma\r\nDelta\n',
      search: 'Gamma',
      replacement: 'Gamma, edited.',
    },
    'no final newline': {
      source: 'First paragraph.\n\nSecond paragraph.',
      search: 'First paragraph.',
      replacement: 'First paragraph, edited.',
    },
    'lone cr': {
      source: 'Alpha\rBeta\r\rGamma',
      search: 'Gamma',
      replacement: 'Gamma, edited.',
    },
  }

  for (const [label, { source, search, replacement }] of Object.entries(cases)) {
    it(`edits one line and keeps every other byte (${label})`, async () => {
      const mounted = await mountEditor(source)
      try {
        mounted.editor.setMode('source')
        const view = getSourceView(mounted.root)
        replaceSourceText(view, search, replacement)
        const expected = source.replace(search, replacement)
        expect(mounted.editor.getMarkdown()).toBe(expected)
        expect(view.state.doc.toString()).toBe(expected)
      } finally {
        await mounted.cleanup()
      }
    })
  }

  it('highlights Markdown tokens', async () => {
    const source = '# Heading\n\n**Bold** and [link](https://example.com).\n'
    const mounted = await mountEditor(source)
    try {
      mounted.editor.setMode('source')
      const view = getSourceView(mounted.root)
      const tokens = Array.from(view.contentDOM.querySelectorAll('.cm-line span'))
      expect(tokens.length).toBeGreaterThan(0)
      expect(tokens.some((token) => token.textContent?.trim() === 'Heading')).toBe(true)
      expect(tokens.some((token) => token.className.length > 0)).toBe(true)
    } finally {
      await mounted.cleanup()
    }
  })

  it('undoes and redoes an edit inside source mode', async () => {
    const source = 'First paragraph.\n\nSecond paragraph.\n'
    const mounted = await mountEditor(source)
    try {
      mounted.editor.setMode('source')
      const view = getSourceView(mounted.root)
      replaceSourceText(view, 'Second paragraph.', 'Second paragraph, edited.')
      expect(view.state.doc.toString()).not.toBe(source)

      undo(view)
      expect(view.state.doc.toString()).toBe(source)
      redo(view)
      expect(view.state.doc.toString()).toContain('Second paragraph, edited.')
    } finally {
      await mounted.cleanup()
    }
  })
})

describe('source mode cursor mapping', () => {
  const source = '# Heading\n\nFirst paragraph.\n\nSecond paragraph.\n'

  it('reveals and reports exact source offsets', async () => {
    const mounted = await mountEditor(source)
    try {
      mounted.editor.setMode('source')
      const view = getSourceView(mounted.root)
      const offsets: number[] = []
      mounted.editor.onSelectionChange((offset) => offsets.push(offset))

      const target = source.indexOf('Second paragraph.') + 3
      expect(mounted.editor.reveal(target)).toBe(true)
      expect(view.state.selection.main.head).toBe(target)
      expect(offsets.at(-1)).toBe(target)

      view.dispatch({ selection: { anchor: 2 } })
      expect(offsets.at(-1)).toBe(2)
    } finally {
      await mounted.cleanup()
    }
  })

  it('selects an exact source range and reports where it starts', async () => {
    const mounted = await mountEditor(source)
    try {
      mounted.editor.setMode('source')
      const view = getSourceView(mounted.root)
      const offsets: number[] = []
      mounted.editor.onSelectionChange((offset) => offsets.push(offset))

      const from = source.indexOf('First paragraph.')
      const to = from + 'First paragraph.'.length
      expect(mounted.editor.select(from, to)).toBe(true)
      expect(view.state.sliceDoc(view.state.selection.main.from, view.state.selection.main.to)).toBe(
        'First paragraph.',
      )
      expect(offsets.at(-1)).toBe(from)
      expect(mounted.editor.select(to, from)).toBe(false)
    } finally {
      await mounted.cleanup()
    }
  })

  it('notifies listeners registered at different times', async () => {
    const mounted = await mountEditor(source)
    try {
      mounted.editor.setMode('source')
      const view = getSourceView(mounted.root)
      view.dispatch({ selection: { anchor: 2 } })

      const first: number[] = []
      mounted.editor.onSelectionChange((offset) => first.push(offset))
      view.dispatch({ selection: { anchor: 5 } })

      const second: number[] = []
      mounted.editor.onSelectionChange((offset) => second.push(offset))
      view.dispatch({ selection: { anchor: 7 } })

      expect(first).toEqual([2, 5, 7])
      expect(second).toEqual([5, 7])

      view.dispatch({ selection: { anchor: 9 } })
      expect(first.at(-1)).toBe(9)
      expect(second.at(-1)).toBe(9)
    } finally {
      await mounted.cleanup()
    }
  })

  it('keeps the cursor block visible across both switches', async () => {
    const mounted = await mountEditor(source)
    try {
      const offsets: number[] = []
      mounted.editor.onSelectionChange((offset) => offsets.push(offset))
      const secondStart = source.indexOf('Second paragraph.')

      expect(mounted.editor.reveal(secondStart + 4)).toBe(true)
      expect(offsets.at(-1)).toBe(secondStart)

      mounted.editor.setMode('source')
      const view = getSourceView(mounted.root)
      expect(view.state.selection.main.head).toBe(secondStart)

      view.dispatch({ selection: { anchor: secondStart + 5 } })
      mounted.editor.setMode('preview')
      expect(offsets.at(-1)).toBe(secondStart)
    } finally {
      await mounted.cleanup()
    }
  })

  it('notifies mode listeners synchronously', async () => {
    const mounted = await mountEditor(source)
    try {
      const modes: string[] = []
      mounted.editor.onModeChange((mode) => modes.push(mode))

      mounted.editor.setMode('source')
      mounted.editor.setMode('source')
      mounted.editor.toggleMode()
      expect(modes).toEqual(['source', 'preview'])
    } finally {
      await mounted.cleanup()
    }
  })
})

describe('source mode switching boundaries', () => {
  const source = 'First paragraph.\n\nSecond paragraph.\n'

  it('keeps Preview state and undo history when leaving unchanged', async () => {
    const mounted = await mountEditor(source)
    try {
      replaceParagraph(mounted.editor, 'First paragraph.', 'First paragraph, edited.')
      const composed = mounted.editor.getMarkdown()
      const previewView = mounted.editor.getView()

      mounted.editor.setMode('source')
      mounted.editor.setMode('preview')
      expect(mounted.editor.getMarkdown()).toBe(composed)

      undoPreview(previewView.state, (transaction) => previewView.dispatch(transaction))
      expect(mounted.editor.getMarkdown()).toBe(source)
    } finally {
      await mounted.cleanup()
    }
  })

  it('rebuilds Preview from edited source and resets its undo history', async () => {
    const mounted = await mountEditor(source)
    try {
      mounted.editor.setMode('source')
      const view = getSourceView(mounted.root)
      replaceSourceText(view, 'Second paragraph.', 'Second paragraph, edited.')
      const edited = view.state.doc.toString()

      mounted.editor.setMode('preview')
      expect(mounted.editor.getMarkdown()).toBe(edited)

      const previewView = mounted.editor.getView()
      undoPreview(previewView.state, (transaction) => previewView.dispatch(transaction))
      expect(mounted.editor.getMarkdown()).toBe(edited)
    } finally {
      await mounted.cleanup()
    }
  })

  it('rebuilds Preview when a source reload matches the pre-source text', async () => {
    const mounted = await mountEditor(source)
    try {
      replaceParagraph(mounted.editor, 'First paragraph.', 'First paragraph, edited.')
      const composed = mounted.editor.getMarkdown()

      mounted.editor.setMode('source')
      mounted.editor.setMarkdown(composed)
      mounted.editor.setMode('preview')
      expect(mounted.editor.getMarkdown()).toBe(composed)

      const previewView = mounted.editor.getView()
      undoPreview(previewView.state, (transaction) => previewView.dispatch(transaction))
      expect(mounted.editor.getMarkdown()).toBe(composed)
    } finally {
      await mounted.cleanup()
    }
  })

  it('does not emit a transient leading offset when returning from edited source', async () => {
    const source = 'First paragraph.\n\nSecond paragraph.\n'
    const mounted = await mountEditor(source)
    try {
      mounted.editor.setMode('source')
      const view = getSourceView(mounted.root)
      const secondStart = source.indexOf('Second paragraph.')
      replaceSourceText(view, 'Second paragraph.', 'Second paragraph, edited.')

      const offsets: number[] = []
      mounted.editor.onSelectionChange((offset) => offsets.push(offset))
      offsets.length = 0

      mounted.editor.setMode('preview')
      expect(offsets).toEqual([secondStart])
    } finally {
      await mounted.cleanup()
    }
  })

  it('does not emit change events for switching or reloading', async () => {
    const mounted = await mountEditor(source)
    const changes: string[] = []
    mounted.editor.onChange((text) => changes.push(text))
    try {
      mounted.editor.setMode('source')
      mounted.editor.setMode('preview')
      mounted.editor.setMode('source')
      mounted.editor.setMarkdown('Replaced while in source.\n')
      mounted.editor.setMode('preview')
      expect(changes).toEqual([])
    } finally {
      await mounted.cleanup()
    }
  })
})

describe('source mode external state', () => {
  it('reports the caret reset when source is reloaded', async () => {
    const mounted = await mountEditor('First paragraph.\n\nSecond paragraph.\n')
    try {
      mounted.editor.setMode('source')
      const view = getSourceView(mounted.root)
      const target = view.state.doc.toString().indexOf('Second paragraph.')
      view.dispatch({ selection: { anchor: target } })

      const offsets: number[] = []
      mounted.editor.onSelectionChange((offset) => offsets.push(offset))
      expect(offsets).toEqual([target])

      mounted.editor.setMarkdown('First paragraph.\n\nSecond paragraph.\n')
      expect(offsets.at(-1)).toBe(0)

      view.dispatch({ selection: { anchor: target } })
      expect(offsets.at(-1)).toBe(target)
    } finally {
      await mounted.cleanup()
    }
  })

  it('replaces the visible source and resets its history without changing mode', async () => {
    const mounted = await mountEditor('Original.\n')
    try {
      mounted.editor.setMode('source')
      const view = getSourceView(mounted.root)
      replaceSource(view, 0, view.state.doc.length, 'Dirty.\n')
      expect(view.state.doc.toString()).toBe('Dirty.\n')

      mounted.editor.setMarkdown('Reloaded.\n')
      expect(mounted.editor.getMode()).toBe('source')
      expect(view.state.doc.toString()).toBe('Reloaded.\n')
      undo(view)
      expect(view.state.doc.toString()).toBe('Reloaded.\n')
    } finally {
      await mounted.cleanup()
    }
  })

  it('applies read-only to the source editor and lifts it again', async () => {
    const mounted = await mountEditor('First paragraph.\n')
    try {
      mounted.editor.setReadonly(true)
      mounted.editor.setMode('source')
      const view = getSourceView(mounted.root)
      expect(view.state.facet(EditorState.readOnly)).toBe(true)
      expect(view.contentDOM.getAttribute('contenteditable')).toBe('false')

      mounted.editor.setReadonly(false)
      expect(view.state.facet(EditorState.readOnly)).toBe(false)
      expect(view.contentDOM.getAttribute('contenteditable')).toBe('true')
    } finally {
      await mounted.cleanup()
    }
  })
})
