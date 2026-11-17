import '../src/styles.css'
import { TextSelection } from '@milkdown/kit/prose/state'
import { cdp, page, userEvent } from 'vitest/browser'
import { describe, expect, it, vi } from 'vitest'
import { mountEditor, type MountedEditor } from './helpers/editor'
import { MOD } from './helpers/platform'

const TABLE_DOC = `# T

| A | B |
|---|---|
| x1 | y1 |
| x2 | y2 |
`

const LIST_DOC = `# L

- first item
- second item
`

const QUOTE_DOC = `# Q

> first quoted paragraph
>
> second quoted paragraph
`

function filler(lines: number): string {
  return Array.from({ length: lines }, (_, i) => `filler paragraph ${i}`).join('\n\n')
}

const CODE_DOC = `# C

${filler(40)}

\`\`\`txt
one
two
three
\`\`\`
`

// App.tsx mounts the editor inside a `.crepe-root` div; the theme CSS is
// scoped to that class, so replicate it here or no theme rule applies.
async function mountInAppRoot(text: string): Promise<MountedEditor> {
  const mounted = await mountEditor(text)
  mounted.root.classList.add('crepe-root')
  return mounted
}

describe('editor typeset', () => {
  it('resets cell paragraphs to the table metrics', async () => {
    const mounted = await mountInAppRoot(TABLE_DOC)
    try {
      const tdP = mounted.root.querySelector('td > p')
      const thP = mounted.root.querySelector('th > p')
      if (!tdP || !thP) throw new Error('cell paragraphs missing')
      for (const p of [tdP, thP]) {
        const cs = getComputedStyle(p)
        expect(cs.marginBottom).toBe('0px')
        expect(cs.fontSize).toBe('14px')
      }
    } finally {
      await mounted.cleanup()
    }
  })

  it('removes the bottom margin inside list items', async () => {
    const mounted = await mountInAppRoot(LIST_DOC)
    try {
      const p = mounted.root.querySelector('.milkdown-list-item-block .children > .content-dom > p')
      if (!p) throw new Error('list paragraph missing')
      expect(getComputedStyle(p).marginBottom).toBe('0px')
    } finally {
      await mounted.cleanup()
    }
  })

  it('keeps the 16px flow between blockquote paragraphs', async () => {
    const mounted = await mountInAppRoot(QUOTE_DOC)
    try {
      const ps = [...mounted.root.querySelectorAll('blockquote > p')]
      if (ps.length < 2) throw new Error('blockquote paragraphs missing')
      const gap = ps[1].getBoundingClientRect().top - ps[0].getBoundingClientRect().bottom
      expect(Math.round(gap)).toBe(16)
    } finally {
      await mounted.cleanup()
    }
  })

  it('lands a collapsed caret on a single click into a table cell', async () => {
    const mounted = await mountInAppRoot(TABLE_DOC)
    try {
      const view = mounted.editor.getView()
      // locator, not a captured element: Crepe swaps the preview table in
      // asynchronously and can detach the node mid-click
      await userEvent.click(page.getByRole('cell').first())
      await new Promise((resolve) => setTimeout(resolve, 100))
      const selection = view.state.selection
      expect(selection.constructor.name).toBe('TextSelection')
      expect(selection.empty).toBe(true)
      await userEvent.keyboard('X')
      await vi.waitFor(() => {
        const md = mounted.editor.getMarkdown()
        expect(md).toContain('x1')
        expect(md).toContain('y1')
        expect(md).toMatch(/x?1?X?/)
        expect(md).toContain('X')
      })
    } finally {
      await mounted.cleanup()
    }
  })

  it('flags pointer drags on the wysiwyg root', async () => {
    const mounted = await mountInAppRoot(`# L\n\nplain paragraph\n`)
    try {
      const wysiwyg = mounted.root.querySelector('.mgo-wysiwyg-root')
      const p = mounted.root.querySelector('.ProseMirror > p')
      if (!wysiwyg || !p) throw new Error('editor root missing')
      expect(wysiwyg.classList.contains('mgo-pointer-selecting')).toBe(false)
      p.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, pointerId: 1, isPrimary: true }))
      expect(wysiwyg.classList.contains('mgo-pointer-selecting')).toBe(true)
      window.dispatchEvent(new PointerEvent('pointerup', { bubbles: true, pointerId: 1, isPrimary: true }))
      expect(wysiwyg.classList.contains('mgo-pointer-selecting')).toBe(false)
    } finally {
      await mounted.cleanup()
    }
  })

  it('renders compact one-line rows without cell borders', async () => {
    const mounted = await mountInAppRoot(TABLE_DOC)
    try {
      const rows = [...mounted.root.querySelectorAll('tbody tr')]
      const heights = rows.map((r) => r.getBoundingClientRect().height)
      for (const h of heights) expect(h).toBeLessThanOrEqual(35)
      const td = mounted.root.querySelector('td')
      if (!td) throw new Error('cell missing')
      for (const side of ['Top', 'Right', 'Bottom', 'Left'] as const) {
        expect(getComputedStyle(td)[`border${side}Width`]).toBe('0px')
      }
    } finally {
      await mounted.cleanup()
    }
  })

  it('scrolls a wide table horizontally instead of squeezing it', async () => {
    const cells = Array.from({ length: 9 }, (_, i) => `col${i}-` + 'abcdefghij'.repeat(3))
    const row = `| ${cells.join(' | ')} |`
    const doc = `# W\n\n${row}\n|${' --- |'.repeat(9)}\n${row}\n${row}\n`
    const mounted = await mountInAppRoot(doc)
    try {
      mounted.root.style.width = '720px'
      const wrapper = mounted.root.querySelector<HTMLElement>('.table-wrapper')
      if (!wrapper) throw new Error('table wrapper missing')
      expect(wrapper.scrollWidth).toBeGreaterThan(wrapper.clientWidth)
      wrapper.scrollLeft = 100
      expect(wrapper.scrollLeft).toBe(100)
    } finally {
      await mounted.cleanup()
    }
  })

  it('keeps a 4px gap between list items at every depth', async () => {
    const doc = `# N

- parent one
  - nested a
  - nested b
    - deep item
- parent two
- parent three
`
    const mounted = await mountInAppRoot(doc)
    try {
      const blocks = [...mounted.root.querySelectorAll('.milkdown-list-item-block')]
      const gaps: number[] = []
      for (const blk of blocks) {
        const next = blk.nextElementSibling
        if (!next || !next.classList.contains('milkdown-list-item-block')) continue
        const last = blk.querySelector(':scope > .list-item > .children > .content-dom')?.lastElementChild
        const first = next.querySelector(':scope > .list-item > .children > .content-dom')?.firstElementChild
        if (!last || !first) continue
        gaps.push(
          first.getBoundingClientRect().top - last.getBoundingClientRect().bottom,
        )
      }
      expect(gaps.length).toBeGreaterThanOrEqual(3)
      for (const g of gaps) expect(Math.abs(g - 4)).toBeLessThanOrEqual(0.5)
    } finally {
      await mounted.cleanup()
    }
  })

  it('keeps a newer selection when Crepe replays the stale cell selection (#2495)', async () => {
    // Crepe's table view dispatches the whole-cell NodeSelection from a rAF
    // (or a 20ms timeout) built on the click-time state. Hold rAF so a newer
    // selection can land first; replaying the frame must not clobber it.
    const frames: FrameRequestCallback[] = []
    const original = window.requestAnimationFrame
    window.requestAnimationFrame = ((cb: FrameRequestCallback) => {
      frames.push(cb)
      return frames.length
    }) as typeof window.requestAnimationFrame
    const mounted = await mountInAppRoot(TABLE_DOC)
    try {
      const view = mounted.editor.getView()
      await userEvent.click(page.getByRole('cell').first())
      let posB = -1
      let seen = 0
      view.state.doc.descendants((node, pos) => {
        if (posB !== -1) return false
        if (node.type.name === 'table_cell' || node.type.name === 'table_header') {
          seen += 1
          if (seen === 2) {
            posB = pos + 2
            return false
          }
        }
        return true
      })
      if (posB === -1) throw new Error('second cell missing')
      view.dispatch(view.state.tr.setSelection(TextSelection.create(view.state.doc, posB)))
      expect(view.state.selection.from).toBe(posB)

      for (const cb of frames.splice(0)) cb(performance.now())
      await new Promise((resolve) => setTimeout(resolve, 50))

      expect(view.state.selection.from).toBe(posB)
      expect(view.state.selection.empty).toBe(true)
    } finally {
      window.requestAnimationFrame = original
      await mounted.cleanup()
    }
  })

  it('inserts typed text at the click point without waiting', async () => {
    const mounted = await mountInAppRoot(TABLE_DOC)
    try {
      const cell = page.getByRole('cell').nth(1)
      await userEvent.click(cell)
      await userEvent.keyboard('X')
      await vi.waitFor(() => {
        const cells = [...mounted.root.querySelectorAll('tbody td')]
        const text = cells[1]?.textContent ?? ''
        expect(text).toHaveLength(3)
        expect(text).toContain('X')
        expect(text).toContain('y')
        expect(text).toContain('1')
      })
    } finally {
      await mounted.cleanup()
    }
  })

  it('selects a word inside a cell on double click', async () => {
    const mounted = await mountInAppRoot(TABLE_DOC)
    try {
      const view = mounted.editor.getView()
      // aim at the text itself: a cell-centre click lands on empty space
      // past the short text and the browser keeps a collapsed caret
      await page.getByRole('cell').first().dblClick({ position: { x: 6, y: 15 } })
      await vi.waitFor(() => {
        expect(view.state.selection.empty).toBe(false)
      })
      const selection = view.state.selection
      expect(view.state.doc.textBetween(selection.from, selection.to)).toBe('x1')
    } finally {
      await mounted.cleanup()
    }
  })

  it('drags to text in one cell and to a CellSelection across cells', async () => {
    const mounted = await mountInAppRoot(TABLE_DOC)
    try {
      const view = mounted.editor.getView()
      const session = cdp()
      const rects = [...mounted.root.querySelectorAll('tbody td')].map((td) =>
        td.getBoundingClientRect(),
      )
      const inside = rects[0]
      // drag within one cell -> plain text selection
      await session.send('Input.dispatchMouseEvent', { type: 'mousePressed', x: inside.x + 2, y: inside.y + 10, button: 'left', clickCount: 1 })
      await session.send('Input.dispatchMouseEvent', { type: 'mouseMoved', x: inside.x + inside.width - 6, y: inside.y + 12, button: 'left', buttons: 1 })
      await session.send('Input.dispatchMouseEvent', { type: 'mouseReleased', x: inside.x + inside.width - 6, y: inside.y + 12, button: 'left' })
      await vi.waitFor(() => {
        expect(view.state.selection.empty).toBe(false)
      })
      expect(view.state.selection.constructor.name).toBe('TextSelection')

      // drag across the cell boundary: Crepe's stopEvent swallows the
      // mousedown before prosemirror-tables can arm its drag watcher, so a
      // raw-listener bridge in tableCellDrag.ts dispatches the CellSelection
      // itself once the pointer crosses a cell boundary.
      await session.send('Input.dispatchMouseEvent', { type: 'mousePressed', x: inside.x + 2, y: inside.y + 10, button: 'left', clickCount: 1 })
      const next = rects[1]
      const steps = 6
      for (let i = 1; i <= steps; i += 1) {
        const x = inside.x + 2 + ((next.x + next.width - 4 - (inside.x + 2)) * i) / steps
        await session.send('Input.dispatchMouseEvent', { type: 'mouseMoved', x, y: inside.y + 12, button: 'left', buttons: 1 })
      }
      await session.send('Input.dispatchMouseEvent', { type: 'mouseReleased', x: next.x + next.width - 4, y: next.y + 12, button: 'left' })
      await vi.waitFor(() => {
        expect(view.state.selection.constructor.name).toBe('CellSelection')
      })
      // the drop indicator schedules onHide on a 30ms timer; let it fire while
      // the editor is still alive so teardown doesn't hit a disposed ctx
      await new Promise((resolve) => setTimeout(resolve, 80))
    } finally {
      await mounted.cleanup()
    }
  })

  it('moves the caret between cells with Tab and Shift-Tab', async () => {
    const mounted = await mountInAppRoot(TABLE_DOC)
    try {
      const view = mounted.editor.getView()
      await userEvent.click(page.getByRole('cell').first())
      const cellPos = () => {
        const { $from } = view.state.selection
        for (let d = $from.depth; d > 0; d -= 1) {
          const name = $from.node(d).type.name
          if (name === 'table_cell' || name === 'table_header') return $from.before(d)
        }
        return -1
      }
      const first = cellPos()
      expect(first).toBeGreaterThanOrEqual(0)
      await userEvent.keyboard('{Tab}')
      const second = cellPos()
      expect(second).toBeGreaterThan(first)
      await userEvent.keyboard('{Shift>}{Tab}{/Shift}')
      expect(cellPos()).toBe(first)
    } finally {
      await mounted.cleanup()
    }
  })

  it('inserts IME text at the caret after a single cell click', async () => {
    const mounted = await mountInAppRoot(TABLE_DOC)
    try {
      const session = cdp()
      await userEvent.click(page.getByRole('cell').first())
      await session.send('Input.imeSetComposition', { text: 'zhongwen', selectionStart: 8, selectionEnd: 8 })
      await session.send('Input.insertText', { text: '中文' })
      await vi.waitFor(() => {
        const cell = mounted.root.querySelector('tbody td')
        expect(cell?.textContent).toContain('中文')
        expect(cell?.textContent).toContain('1')
      })
    } finally {
      await mounted.cleanup()
    }
  })

  it('keeps select-all working inside a table cell', async () => {
    const mounted = await mountInAppRoot(TABLE_DOC)
    try {
      const view = mounted.editor.getView()
      await userEvent.click(page.getByRole('cell').first())
      await vi.waitFor(() => {
        expect(view.state.selection.empty).toBe(true)
      })
      await userEvent.keyboard(`{${MOD}>}a{/${MOD}}`)
      const selection = view.state.selection
      expect(selection.constructor.name).toBe('AllSelection')
      expect(selection.to).toBe(view.state.doc.content.size)
    } finally {
      await mounted.cleanup()
    }
  })

  it('keeps triple-click and Shift-selections inside a cell', async () => {
    const mounted = await mountInAppRoot(TABLE_DOC)
    try {
      const view = mounted.editor.getView()
      const cell = page.getByRole('cell').first()
      // aim at the text, like the double-click test: a centre click lands on
      // empty space and triple-click needs the text itself
      await cell.tripleClick({ position: { x: 6, y: 15 } })
      await vi.waitFor(() => {
        expect(view.state.selection.empty).toBe(false)
      })
      expect(view.state.selection.constructor.name).toBe('TextSelection')
      expect(view.state.doc.textBetween(view.state.selection.from, view.state.selection.to)).toBe('x1')

      // deterministic caret: first cell's text 'x1' spans [start, start+2]
      let start = -1
      view.state.doc.descendants((node, pos) => {
        if (start !== -1) return false
        if (node.type.name === 'table_cell') {
          start = pos + 2
          return false
        }
        return true
      })
      if (start === -1) throw new Error('cell missing')

      // Shift+Home from the end of the cell text selects to the line start;
      // the browser's line-box edge may sit one position before the
      // paragraph node, so assert the range covers the cell's text
      view.dispatch(view.state.tr.setSelection(TextSelection.create(view.state.doc, start + 2)))
      await userEvent.keyboard('{Shift>}{Home}{/Shift}')
      await vi.waitFor(() => {
        expect(view.state.doc.textBetween(view.state.selection.from, view.state.selection.to)).toContain('x1')
      })
      expect(view.state.selection.constructor.name).toBe('TextSelection')
      expect(view.state.selection.from).toBeLessThanOrEqual(start)
      expect(view.state.selection.to).toBe(start + 2)

      // Shift+ArrowRight from the start selects one character
      view.dispatch(view.state.tr.setSelection(TextSelection.create(view.state.doc, start)))
      await userEvent.keyboard('{Shift>}{ArrowRight}{/Shift}')
      await vi.waitFor(() => {
        expect(view.state.doc.textBetween(view.state.selection.from, view.state.selection.to)).toBe('x')
      })
      expect(view.state.selection.constructor.name).toBe('TextSelection')
    } finally {
      await mounted.cleanup()
    }
  })

  it('lets Escape reach other handlers and ignores it at a collapsed caret', async () => {
    const mounted = await mountInAppRoot(`# T\n\nselectable words here\n`)
    try {
      const view = mounted.editor.getView()
      const wysiwyg = mounted.root.querySelector('.mgo-wysiwyg-root')
      if (!wysiwyg) throw new Error('wysiwyg root missing')

      // collapsed caret: nothing to dismiss, the class must stay off
      await page.getByText('selectable words here').click({ position: { x: 60, y: 14 } })
      await vi.waitFor(() => {
        expect(view.state.selection.empty).toBe(true)
      })
      await userEvent.keyboard('{Escape}')
      expect(wysiwyg.classList.contains('mgo-toolbar-dismissed')).toBe(false)

      // open selection: Escape hides the toolbar but must not be consumed —
      // menus, composers and other keymaps still get their keydown
      await page.getByText('selectable words here').dblClick({ position: { x: 60, y: 14 } })
      await vi.waitFor(() => {
        expect(view.state.selection.empty).toBe(false)
      })
      let reached = false
      const onKey = (event: KeyboardEvent) => {
        if (event.key === 'Escape') reached = true
      }
      document.addEventListener('keydown', onKey)
      await userEvent.keyboard('{Escape}')
      document.removeEventListener('keydown', onKey)
      expect(reached).toBe(true)
      expect(wysiwyg.classList.contains('mgo-toolbar-dismissed')).toBe(true)
      expect(view.state.selection.empty).toBe(false)

      // and a plain pointerdown restores it, matching the class contract
      const p = mounted.root.querySelector('.ProseMirror > p')
      if (!p) throw new Error('paragraph missing')
      p.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, pointerId: 1, isPrimary: true }))
      expect(wysiwyg.classList.contains('mgo-toolbar-dismissed')).toBe(false)
    } finally {
      await mounted.cleanup()
    }
  })

  it('hides the toolbar on Escape until the selection changes', async () => {
    const mounted = await mountInAppRoot(`# T\n\nselectable words here\n`)
    try {
      const view = mounted.editor.getView()
      const wysiwyg = mounted.root.querySelector('.mgo-wysiwyg-root')
      if (!wysiwyg) throw new Error('wysiwyg root missing')
      // position lands on the text, not the empty rest of the full-width p
      await page.getByText('selectable words here').dblClick({ position: { x: 60, y: 14 } })
      await vi.waitFor(() => {
        expect(view.state.selection.empty).toBe(false)
      })

      await userEvent.keyboard('{Escape}')
      expect(wysiwyg.classList.contains('mgo-toolbar-dismissed')).toBe(true)
      expect(view.state.selection.empty).toBe(false)

      await userEvent.keyboard('{Shift>}{ArrowRight}{/Shift}')
      expect(wysiwyg.classList.contains('mgo-toolbar-dismissed')).toBe(false)
      expect(view.state.selection.empty).toBe(false)
    } finally {
      await mounted.cleanup()
    }
  })

  it('keeps code block height stable across the placeholder/mounted swap', async () => {
    const mounted = await mountInAppRoot(CODE_DOC)
    try {
      const block = mounted.root.querySelector<HTMLElement>('.milkdown-code-block')
      if (!block) throw new Error('code block missing')
      // The block starts far below the fold: still a placeholder.
      const placeholder = block.querySelector('.milkdown-code-block-placeholder')
      expect(placeholder).not.toBeNull()
      const placeholderH = block.getBoundingClientRect().height
      block.scrollIntoView({ block: 'center' })
      await vi.waitFor(
        () => {
          expect(block.querySelector('.cm-editor')).not.toBeNull()
        },
        { timeout: 4000 },
      )
      const mountedH = block.getBoundingClientRect().height
      expect(Math.abs(mountedH - placeholderH)).toBeLessThanOrEqual(0.5)
    } finally {
      await mounted.cleanup()
    }
  })
})
