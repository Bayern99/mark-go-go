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
  it('folds a section up to the next same-or-higher heading', async () => {
    const { editor, cleanup } = await mountEditor(DOC)
    const [alpha] = headingPositions(editor)

    editor.toggleFold(alpha) // h1 hides everything after it
    expect(hiddenCount(editor)).toBe(7) // 4 headings + 4 paras

    await cleanup()

    const t2 = await mountEditor(DOC)
    const heads = headingPositions(t2.editor)
    t2.editor.toggleFold(heads[1]) // h2 hides inner para + h3 + deep para
    expect(hiddenCount(t2.editor)).toBe(3)
    await t2.cleanup()
  })

  it('leaves child fold state predictable when expanding the parent', async () => {
    const { editor, cleanup } = await mountEditor(DOC)
    const [alpha, sub] = headingPositions(editor)

    editor.toggleFold(sub) // fold child first
    editor.toggleFold(alpha) // fold parent
    editor.toggleFold(alpha) // expand parent
    // child stays folded
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
    const inside = sub + subNode.nodeSize + 2 // inner para text
    const { TextSelection } = await import('@milkdown/kit/prose/state')
    view.dispatch(view.state.tr.setSelection(TextSelection.create(view.state.doc, inside)))

    editor.toggleFold(sub)
    const sel = view.state.selection
    // caret now sits inside the heading (visible)
    expect(sel.from).toBeLessThanOrEqual(sub + subNode.nodeSize)
    await cleanup()
  })

  it('reveal expands the fold covering the target', async () => {
    const { editor, cleanup } = await mountEditor(DOC)
    const [alpha] = headingPositions(editor)
    editor.toggleFold(alpha)
    expect(hiddenCount(editor)).toBeGreaterThan(0)

    // offset of the deep heading's text in markdown source
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

    // Edit the heading text: fold stays on the (renamed) Beta heading.
    view.dispatch(view.state.tr.insertText('X', beta + 1))
    const anchors = editor.foldAnchors()
    expect(anchors.length).toBe(1)
    const node = view.state.doc.nodeAt(anchors[0])!
    expect(node.type.name).toBe('heading')
    expect(node.textContent).toContain('XBeta')

    // Delete that heading entirely: fold is dropped.
    const hb = anchors[0]
    const hn = view.state.doc.nodeAt(hb)!
    view.dispatch(view.state.tr.delete(hb, hb + hn.nodeSize))
    expect(editor.foldAnchors().length).toBe(0)
    await cleanup()
  })

  it('persists fold keys per doc and restores on remount', async () => {
    localStorage.setItem(
      'mgo.folds:test-doc',
      JSON.stringify({ keys: ['h2:beta#0'], counts: { 'h2:beta': 1 } }),
    )
    const root = document.createElement('div')
    document.body.append(root)
    const { createMarkdownEditor } = await import('../src/editor/createEditor')
    const editor = await createMarkdownEditor(root, {
      initialText: DOC,
      docKey: 'test-doc',
    })
    expect(editor.foldAnchors().length).toBe(1)
    expect(hiddenCount(editor)).toBe(1) // Beta's section body is one para
    // collapse state written back
    const stored = JSON.parse(localStorage.getItem('mgo.folds:test-doc')!)
    expect(stored.keys).toEqual(['h2:beta#0'])
    expect(stored.counts['h2:beta']).toBe(1)
    await editor.destroy()
    root.remove()
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
  it('restores only unambiguous folds when same-name counts changed', async () => {
    // saved: second "Same" folded, "Other" folded; then external edit added
    // a third "Same" heading -> every 'h2:same' key is ambiguous -> dropped,
    // while 'h2:other' still matches.
    localStorage.setItem(
      'mgo.folds:dup-doc',
      JSON.stringify({ keys: ['h2:same#1', 'h2:other#0'], counts: { 'h2:same': 2, 'h2:other': 1 } }),
    )
    const edited = '# A\n\nx\n\n## Same\n\ns0\n\n## Same\n\ns1\n\n## Same\n\ns2\n\n## Other\n\no\n'
    const root = document.createElement('div')
    document.body.append(root)
    const { createMarkdownEditor } = await import('../src/editor/createEditor')
    const editor = await createMarkdownEditor(root, { initialText: edited, docKey: 'dup-doc' })
    // 'h2:same' dropped (count 2 -> 3), 'h2:other' survives
    expect(editor.foldAnchors().length).toBe(1)
    const pos = editor.foldAnchors()[0]
    expect(editor.getView().state.doc.nodeAt(pos)!.textContent).toBe('Other')
    await editor.destroy()
    root.remove()
    localStorage.removeItem('mgo.folds:dup-doc')
  })

  it('expands a fold when the caret is placed inside its hidden range', async () => {
    const { editor, cleanup } = await mountEditor(DOC)
    const view = editor.getView()
    const [alpha] = headingPositions(editor)
    editor.toggleFold(alpha)
    expect(hiddenCount(editor)).toBeGreaterThan(0)

    // Programmatic caret inside the hidden body — as an ArrowRight from the
    // heading end would land.
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
    editor.toggleFold(sub) // hide Alpha.Sub's body; 'Beta' is the next visible node
    const betaPos = (() => {
      let p = -1
      view.state.doc.forEach((n: any, pos: number) => {
        if (n.type.name === 'heading' && n.textContent === 'Beta') p = pos
      })
      return p
    })()
    // Caret at start of the first visible node after the fold (Beta's
    // heading text), Backspace would merge into hidden content.
    const { TextSelection } = await import('@milkdown/kit/prose/state')
    view.dispatch(view.state.tr.setSelection(TextSelection.create(view.state.doc, betaPos + 1)))
    view.dom.dispatchEvent(new KeyboardEvent('keydown', { key: 'Backspace', bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    expect(editor.foldAnchors()).not.toContain(sub)
    expect(editor.getMarkdown()).toBe(DOC) // nothing deleted
    await cleanup()
  })
})
