import { describe, expect, it } from 'vitest'
import { mountEditor } from './helpers/editor'

const DOC = `# Alpha

Para one.

## Alpha.Sub

Inner para.

### Alpha.Sub.Deep

Deep para.

## Beta

Beta para.

# Gamma

Gamma para.
`

const codeDoc = '# A\n\n```\nline1\nline2\nline3\n```\n\nAfter.\n'

function headingPositions(editor: { getView(): any }): number[] {
  const out: number[] = []
  editor.getView().state.doc.forEach((node: any, pos: number) => {
    if (node.type.name === 'heading') out.push(pos)
  })
  return out
}

function hiddenCount(editor: { getView(): any }): number {
  return editor.getView().dom.querySelectorAll('.mgo-fold-hidden').length
}

describe('section folding', () => {
  it('keeps an untouched section folded between two applied edits', async () => {
    const original = '# Intro\n\nOld introduction.\n\n# Inner\n\nFolded body.\n\n# End\n\nOld conclusion.\n'
    const proposed = original.replace('Old introduction.', 'New introduction.').replace('Old conclusion.', 'New conclusion.')
    const { editor, cleanup } = await mountEditor(original)
    try {
      editor.toggleFold(headingPositions(editor)[1])
      expect(hiddenCount(editor)).toBe(1)
      expect(editor.applyMarkdown(original, proposed)).toBe(true)
      expect(editor.getMarkdown()).toBe(proposed)
      expect(hiddenCount(editor)).toBe(1)
      expect(editor.undo()).toBe(true)
      expect(editor.getMarkdown()).toBe(original)
      expect(hiddenCount(editor)).toBe(1)
    } finally {
      await cleanup()
    }
  })

  it('folds a section up to the next same-or-higher heading', async () => {
    const { editor, cleanup } = await mountEditor(DOC)
    const [alpha] = headingPositions(editor)

    editor.toggleFold(alpha)
    expect(hiddenCount(editor)).toBe(7)

    await cleanup()

    const t2 = await mountEditor(DOC)
    const heads = headingPositions(t2.editor)
    t2.editor.toggleFold(heads[1])
    expect(hiddenCount(t2.editor)).toBe(3)
    await t2.cleanup()
  })

  it('leaves child fold state predictable when expanding the parent', async () => {
    const { editor, cleanup } = await mountEditor(DOC)
    const [alpha, sub] = headingPositions(editor)

    editor.toggleFold(sub)
    editor.toggleFold(alpha)
    editor.toggleFold(alpha)
    expect(editor.foldAnchors()).toEqual([sub])
    expect(hiddenCount(editor)).toBe(3)
    await cleanup()
  })

  it('does not change markdown, dirty state, or undo history', async () => {
    const { editor, cleanup } = await mountEditor(DOC)
    const before = editor.getMarkdown()
    const undoBefore = editor.canUndo()

    const [alpha] = headingPositions(editor)
    editor.toggleFold(alpha)
    editor.toggleFold(alpha)

    expect(editor.getMarkdown()).toBe(before)
    expect(editor.canUndo()).toBe(undoBefore)
    await cleanup()
  })

  it('moves the caret out of a section being folded', async () => {
    const { editor, cleanup } = await mountEditor(DOC)
    const view = editor.getView()
    const [, sub] = headingPositions(editor)
    const subNode = view.state.doc.nodeAt(sub)!
    const inside = sub + subNode.nodeSize + 2
    const { TextSelection } = await import('@milkdown/kit/prose/state')
    view.dispatch(view.state.tr.setSelection(TextSelection.create(view.state.doc, inside)))

    editor.toggleFold(sub)
    const sel = view.state.selection
    expect(sel.from).toBeLessThanOrEqual(sub + subNode.nodeSize)
    await cleanup()
  })

  it('reveal expands the fold covering the target', async () => {
    const { editor, cleanup } = await mountEditor(DOC)
    const [alpha] = headingPositions(editor)
    editor.toggleFold(alpha)
    expect(hiddenCount(editor)).toBeGreaterThan(0)

    const offset = editor.getMarkdown().indexOf('Alpha.Sub.Deep')
    editor.reveal(offset)
    expect(editor.foldAnchors()).not.toContain(alpha)
    await cleanup()
  })

  it('keeps anchors aligned through heading edits and drops deleted headings', async () => {
    const { editor, cleanup } = await mountEditor(DOC)
    const view = editor.getView()
    const [, , , beta] = headingPositions(editor)
    editor.toggleFold(beta)

    view.dispatch(view.state.tr.insertText('X', beta + 1))
    const anchors = editor.foldAnchors()
    expect(anchors.length).toBe(1)
    const node = view.state.doc.nodeAt(anchors[0])!
    expect(node.type.name).toBe('heading')
    expect(node.textContent).toContain('XBeta')

    const hb = anchors[0]
    const hn = view.state.doc.nodeAt(hb)!
    view.dispatch(view.state.tr.delete(hb, hb + hn.nodeSize))
    expect(editor.foldAnchors().length).toBe(0)
    await cleanup()
  })

  it('persists fold keys per doc and restores on remount', async () => {
    const { createMarkdownEditor } = await import('../src/editor/createEditor')
    const mk = () => {
      const root = document.createElement('div')
      document.body.append(root)
      return root
    }
    const r1 = mk()
    const e1 = await createMarkdownEditor(r1, { initialText: DOC, docKey: 'test-doc' })
    const [, , , beta] = headingPositions(e1)
    e1.toggleFold(beta)
    const stored = JSON.parse(localStorage.getItem('mgo.folds:test-doc')!)
    expect(stored.entries.length).toBe(1)
    expect(stored.entries[0].base).toBe('h2:beta')
    await e1.destroy()
    r1.remove()

    const r2 = mk()
    const e2 = await createMarkdownEditor(r2, { initialText: DOC, docKey: 'test-doc' })
    expect(e2.foldAnchors().length).toBe(1)
    expect(hiddenCount(e2)).toBe(1)
    await e2.destroy()
    r2.remove()
    localStorage.removeItem('mgo.folds:test-doc')
  })
})

describe('code block folding', () => {
  it('collapses a code block without touching markdown', async () => {
    const { editor, cleanup } = await mountEditor(codeDoc)
    const before = editor.getMarkdown()
    const view = editor.getView()
    let codePos = -1
    view.state.doc.forEach((node: any, pos: number) => {
      if (node.type.name === 'code_block') codePos = pos
    })
    expect(codePos).toBeGreaterThan(-1)

    editor.toggleFold(codePos)
    expect(editor.getMarkdown()).toBe(before)
    expect(view.dom.querySelector('.mgo-code-folded')).toBeTruthy()

    editor.toggleFold(codePos)
    expect(view.dom.querySelector('.mgo-code-folded')).toBeFalsy()
    await cleanup()
  })
})

describe('fold boundary cases', () => {
  it('restores reordered duplicate sections by content signature', async () => {
    const { createMarkdownEditor } = await import('../src/editor/createEditor')
    const mk = () => {
      const root = document.createElement('div')
      document.body.append(root)
      return root
    }
    const doc = '# A\n\nx\n\n## Same\n\nfirst body\n\n## Same\n\nsecond body\n\n## Other\n\no\n'
    const r1 = mk()
    const e1 = await createMarkdownEditor(r1, { initialText: doc, docKey: 'sig-doc' })
    let heads: number[] = []
    e1.getView().state.doc.forEach((n: any, pos: number) => {
      if (n.type.name === 'heading' && n.textContent === 'Same') heads.push(pos)
    })
    e1.toggleFold(heads[1])
    await e1.destroy()
    r1.remove()

    const reordered = '# A\n\nx\n\n## Same\n\nsecond body\n\n## Same\n\nfirst body\n\n## Other\n\no\n'
    const r2 = mk()
    const e2 = await createMarkdownEditor(r2, { initialText: reordered, docKey: 'sig-doc' })
    const anchors = e2.foldAnchors()
    expect(anchors.length).toBe(1)
    const node = e2.getView().state.doc.nodeAt(anchors[0])!
    const after = e2.getView().state.doc.textBetween(
      anchors[0] + node.nodeSize,
      anchors[0] + node.nodeSize + 20,
    )
    expect(after).toContain('second body')
    await e2.destroy()
    r2.remove()

    const replaced = '# A\n\nx\n\n## Same\n\nsecond body\n\n## Same\n\nCHANGED\n\n## Other\n\no\n'
    localStorage.removeItem('mgo.folds:sig-doc')
    const r3 = mk()
    const e3 = await createMarkdownEditor(r3, { initialText: doc, docKey: 'sig-doc' })
    e3.toggleFold(heads[0])
    await e3.destroy()
    r3.remove()
    const r4 = mk()
    const e4 = await createMarkdownEditor(r4, { initialText: replaced, docKey: 'sig-doc' })
    expect(e4.foldAnchors().length).toBe(0)
    await e4.destroy()
    r4.remove()
    localStorage.removeItem('mgo.folds:sig-doc')
  })

  it('expands a fold when the caret is placed inside its hidden range', async () => {
    const { editor, cleanup } = await mountEditor(DOC)
    const view = editor.getView()
    const [alpha] = headingPositions(editor)
    editor.toggleFold(alpha)
    expect(hiddenCount(editor)).toBeGreaterThan(0)

    const hiddenPos = alpha + view.state.doc.nodeAt(alpha)!.nodeSize + 2
    const { TextSelection } = await import('@milkdown/kit/prose/state')
    view.dispatch(view.state.tr.setSelection(TextSelection.create(view.state.doc, hiddenPos)))
    await new Promise((r) => setTimeout(r, 0))
    expect(editor.foldAnchors()).not.toContain(alpha)
    await cleanup()
  })

  it('expands instead of silently backspacing into a folded region', async () => {
    const { editor, cleanup } = await mountEditor(DOC)
    const view = editor.getView()
    const [, sub] = headingPositions(editor)
    editor.toggleFold(sub)
    const betaPos = (() => {
      let p = -1
      view.state.doc.forEach((n: any, pos: number) => {
        if (n.type.name === 'heading' && n.textContent === 'Beta') p = pos
      })
      return p
    })()
    const { TextSelection } = await import('@milkdown/kit/prose/state')
    view.dispatch(view.state.tr.setSelection(TextSelection.create(view.state.doc, betaPos + 1)))
    view.dom.dispatchEvent(new KeyboardEvent('keydown', { key: 'Backspace', bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    expect(editor.foldAnchors()).not.toContain(sub)
    expect(editor.getMarkdown()).toBe(DOC)
    await cleanup()
  })
})
