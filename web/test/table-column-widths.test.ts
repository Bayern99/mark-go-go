import '../src/styles.css'
import { columnResizingPluginKey } from '@milkdown/kit/prose/tables'
import { cdp, page, userEvent } from 'vitest/browser'
import { describe, expect, it, vi } from 'vitest'
import { mountEditor, type MountedEditor } from './helpers/editor'
import { getSourceView, replaceSource } from './helpers/source'

const TABLE_DOC = `# 表格

正文保持舒适阅读宽度，表格则使用中央窗格的可用空间。

| 文件 | 说明 | 状态 |
| --- | --- | --- |
| \`src/editor/very-long-directory-name/table-column-width-controller.ts\` | 这是一段需要足够列宽才能顺畅阅读的中文说明，不能在每一个汉字后面都强制换行。 | 正常 |
| \`src/editor/second-file.ts\` | 第二行中文内容必须保留。 | 正常 |
| \`src/editor/third-file.ts\` | 第三行中文内容必须保留。 | 正常 |
| \`src/editor/fourth-file.ts\` | 第四行中文内容必须保留。 | 正常 |
`

async function mount(text: string, docKey?: string): Promise<MountedEditor> {
  const mounted = await mountEditor(text, docKey)
  mounted.root.classList.add('crepe-root')
  return mounted
}

function firstColumnWidth(editor: MountedEditor['editor'], tableIndex = 0): number | null {
  const widths: (number | null)[] = []
  editor.getView().state.doc.forEach((node) => {
    if (node.type.name !== 'table') return
    widths.push(node.firstChild?.firstChild?.attrs.colwidth?.[0] ?? null)
  })
  return widths[tableIndex] ?? null
}

async function previewFirstBoundary(
  mounted: MountedEditor,
  delta: number,
  tableIndex = 0,
  hover = true,
): Promise<() => Promise<void>> {
  await document.fonts.ready
  await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)))
  const table = mounted.root.querySelectorAll('.milkdown-table-block')[tableIndex]
  const cell = table?.querySelector('th')
  if (!cell) throw new Error('header cell missing')
  const rect = cell.getBoundingClientRect()
  const x = rect.right - 2
  const y = rect.bottom - 7
  const session = cdp()
  if (hover) {
    await session.send('Input.dispatchMouseEvent', { type: 'mouseMoved', x, y })
    await vi.waitFor(() => {
      expect(mounted.root.querySelector('.ProseMirror.resize-cursor')).not.toBeNull()
    })
  }
  await session.send('Input.dispatchMouseEvent', {
    type: 'mousePressed', x, y, button: 'left', clickCount: 1,
  })
  await session.send('Input.dispatchMouseEvent', {
    type: 'mouseMoved', x: x + delta, y, button: 'left', buttons: 1,
  })
  return async () => {
    await session.send('Input.dispatchMouseEvent', {
      type: 'mouseReleased', x: x + delta, y, button: 'left',
    })
  }
}

async function dragFirstBoundary(
  mounted: MountedEditor,
  delta: number,
  tableIndex = 0,
): Promise<void> {
  const release = await previewFirstBoundary(mounted, delta, tableIndex)
  await release()
}

describe('table column widths', () => {
  it('keeps prose at 624px while allowing tables to use the wider pane', async () => {
    const mounted = await mount(TABLE_DOC)
    try {
      const paragraph = mounted.root.querySelector<HTMLElement>('.ProseMirror > p')
      const table = mounted.root.querySelector<HTMLElement>('.ProseMirror > .milkdown-table-block')
      if (!paragraph || !table) throw new Error('content blocks missing')
      expect(Math.round(paragraph.getBoundingClientRect().width)).toBe(624)
      expect(table.getBoundingClientRect().width).toBeGreaterThan(1000)
      expect(paragraph.getBoundingClientRect().left).toBeGreaterThan(table.getBoundingClientRect().left)
    } finally {
      await mounted.cleanup()
    }
  })

  it('uses the 4px native thumb recipe on the actual table scroll container', async () => {
    const mounted = await mount(TABLE_DOC)
    try {
      const wrapper = mounted.root.querySelector<HTMLElement>('.table-wrapper')
      if (!wrapper) throw new Error('table wrapper missing')
      const style = getComputedStyle(wrapper)
      expect(style.scrollbarWidth).toBe('auto')
      expect(style.scrollbarColor).toBe('auto')
    } finally {
      await mounted.cleanup()
    }
  })

  it('caps long file columns without squeezing Chinese prose', async () => {
    const mounted = await mount(TABLE_DOC)
    try {
      const cells = [...mounted.root.querySelectorAll<HTMLElement>('tbody td')]
      const file = cells[0]?.getBoundingClientRect().width ?? 0
      const prose = cells[1]?.getBoundingClientRect().width ?? 0
      expect(file).toBeLessThanOrEqual(400)
      expect(prose).toBeGreaterThanOrEqual(240)
      expect(prose).toBeGreaterThan(file)
    } finally {
      await mounted.cleanup()
    }
  })

  it('never drops rows when a quoted table boundary is pressed or dragged', async () => {
    const source = [
      '> | A | B |',
      '> | --- | --- |',
      '> | a1 | b1 |',
      '> | a2 | b2 |',
      '> | a3 | b3 |',
      '> | a4 | b4 |',
      '',
    ].join('\n')
    const mounted = await mount(source)
    try {
      const cell = mounted.root.querySelector('th')
      if (!cell) throw new Error('quoted header cell missing')
      const rect = cell.getBoundingClientRect()
      const x = rect.right - 2
      const y = rect.bottom - 7
      const session = cdp()
      await session.send('Input.dispatchMouseEvent', { type: 'mouseMoved', x, y })
      await session.send('Input.dispatchMouseEvent', {
        type: 'mousePressed', x, y, button: 'left', clickCount: 1,
      })
      await session.send('Input.dispatchMouseEvent', {
        type: 'mouseMoved', x: x + 40, y, button: 'left', buttons: 1,
      })
      await session.send('Input.dispatchMouseEvent', {
        type: 'mouseReleased', x: x + 40, y, button: 'left',
      })
      await new Promise((resolve) => setTimeout(resolve, 100))
      expect(mounted.root.querySelectorAll('tbody tr')).toHaveLength(5)
      expect(mounted.editor.getMarkdown()).toBe(source)
    } finally {
      await mounted.cleanup()
    }
  })

  it('clears a previous table handle before pressing a quoted table', async () => {
    const rows = [
      '| A | B |',
      '| --- | --- |',
      '| a1 | b1 |',
      '| a2 | b2 |',
      '| a3 | b3 |',
      '| a4 | b4 |',
    ]
    const source = [...rows, '', ...rows.map((row) => `> ${row}`), ''].join('\n')
    const mounted = await mount(source)
    try {
      const tables = [...mounted.root.querySelectorAll<HTMLElement>('.milkdown-table-block')]
      if (tables.length !== 2) throw new Error('expected plain and quoted tables')
      const quotedHeader = tables[1].querySelector('th')
      const plainHeader = tables[0].querySelector('th')
      if (!quotedHeader || !plainHeader) throw new Error('headers missing')
      const quotedRect = quotedHeader.getBoundingClientRect()
      const plainRect = plainHeader.getBoundingClientRect()
      const session = cdp()
      const quotedCenter = { x: (quotedRect.left + quotedRect.right) / 2, y: quotedRect.bottom - 7 }
      await session.send('Input.dispatchMouseEvent', {
        type: 'mousePressed', ...quotedCenter, button: 'left', clickCount: 1,
      })
      await session.send('Input.dispatchMouseEvent', {
        type: 'mouseReleased', ...quotedCenter, button: 'left',
      })
      const plainBoundary = { x: plainRect.right - 2, y: plainRect.bottom - 7 }
      await session.send('Input.dispatchMouseEvent', { type: 'mouseMoved', ...plainBoundary })
      await vi.waitFor(() => {
        expect(mounted.root.querySelector('.ProseMirror.resize-cursor')).not.toBeNull()
      })
      const quotedBoundary = { x: quotedRect.right - 2, y: quotedRect.bottom - 7 }
      await session.send('Input.dispatchMouseEvent', { type: 'mouseMoved', ...quotedBoundary })
      await session.send('Input.dispatchMouseEvent', {
        type: 'mousePressed', ...quotedBoundary, button: 'left', clickCount: 1,
      })
      await session.send('Input.dispatchMouseEvent', {
        type: 'mouseReleased', ...quotedBoundary, button: 'left',
      })
      await new Promise((resolve) => setTimeout(resolve, 100))
      const currentTables = [...mounted.root.querySelectorAll('.milkdown-table-block')]
      expect(currentTables[0]?.querySelectorAll('tbody tr')).toHaveLength(5)
      expect(currentTables[1]?.querySelectorAll('tbody tr')).toHaveLength(5)
      expect(mounted.editor.getMarkdown()).toBe(source)
    } finally {
      await mounted.cleanup()
    }
  })

  it('cancels a resize when the document changes and allows the next drag', async () => {
    const mounted = await mount(TABLE_DOC)
    try {
      const view = mounted.editor.getView()
      await userEvent.click(page.getByText('正文保持舒适阅读宽度', { exact: false }))
      const cell = mounted.root.querySelector('th')
      if (!cell) throw new Error('header cell missing')
      const rect = cell.getBoundingClientRect()
      const x = rect.right - 2
      const y = rect.bottom - 7
      const session = cdp()
      await session.send('Input.dispatchMouseEvent', { type: 'mouseMoved', x, y })
      await vi.waitFor(() => {
        expect(mounted.root.querySelector('.ProseMirror.resize-cursor')).not.toBeNull()
      })
      await session.send('Input.dispatchMouseEvent', {
        type: 'mousePressed', x, y, button: 'left', clickCount: 1,
      })
      expect(columnResizingPluginKey.getState(view.state)?.dragging).toBeTruthy()

      await userEvent.keyboard('Z')
      const edited = mounted.editor.getMarkdown()
      expect(edited).toContain('Z')
      await session.send('Input.dispatchMouseEvent', {
        type: 'mouseMoved', x: x + 40, y, button: 'left', buttons: 1,
      })
      await session.send('Input.dispatchMouseEvent', {
        type: 'mouseReleased', x: x + 40, y, button: 'left',
      })
      await new Promise((resolve) => setTimeout(resolve, 100))

      expect(mounted.root.querySelectorAll('tbody tr')).toHaveLength(5)
      expect(mounted.editor.getMarkdown()).toContain('Z')
      expect(columnResizingPluginKey.getState(view.state)?.dragging).toBeFalsy()
      expect(firstColumnWidth(mounted.editor)).toBeNull()

      await dragFirstBoundary(mounted, 64)
      await vi.waitFor(() => {
        expect(firstColumnWidth(mounted.editor)).not.toBeNull()
      })
      expect(mounted.editor.getMarkdown()).toBe(edited)
    } finally {
      await mounted.cleanup()
    }
  })

  it('destroys the editor during a column resize preview', async () => {
    const mounted = await mount(TABLE_DOC)
    const view = mounted.editor.getView()
    let release: (() => Promise<void>) | undefined
    try {
      release = await previewFirstBoundary(mounted, 64)
      expect(columnResizingPluginKey.getState(view.state)?.dragging).toBeTruthy()
      expect(firstColumnWidth(mounted.editor)).toBeNull()
    } finally {
      try {
        await mounted.cleanup()
      } finally {
        await release?.()
        mounted.root.remove()
      }
    }
    expect(view.isDestroyed).toBe(true)
  })

  it('drags a boundary without a prior hover, with undo and reopen persistence', async () => {
    let mounted = await mount(TABLE_DOC, '/demo/columns.md')
    try {
      const release = await previewFirstBoundary(mounted, 64, 0, false)
      await release()
      const view = mounted.editor.getView()
      expect(view.state.selection.constructor.name).not.toBe('CellSelection')
      expect(window.getSelection()?.toString()).toBe('')
      await vi.waitFor(() => {
        expect(firstColumnWidth(mounted.editor)).not.toBeNull()
      })
      const resized = firstColumnWidth(mounted.editor)
      expect(mounted.editor.getMarkdown()).toBe(TABLE_DOC)

      expect(mounted.editor.undo()).toBe(true)
      expect(firstColumnWidth(mounted.editor)).toBeNull()
      expect(mounted.editor.getMarkdown()).toBe(TABLE_DOC)
      expect(mounted.editor.redo()).toBe(true)
      expect(firstColumnWidth(mounted.editor)).toBe(resized)

      mounted.editor.setMode('source')
      expect(mounted.editor.getMarkdown()).toBe(TABLE_DOC)
      mounted.editor.setMode('wysiwyg')
      expect(firstColumnWidth(mounted.editor)).toBe(resized)

      mounted.editor.setMode('source')
      const introduction = '表格前新增的段落。\n\n'
      replaceSource(getSourceView(mounted.root), 0, 0, introduction)
      mounted.editor.setMode('wysiwyg')
      expect(mounted.editor.getMarkdown()).toBe(introduction + TABLE_DOC)
      expect(firstColumnWidth(mounted.editor)).toBe(resized)

      await mounted.cleanup()
      mounted = await mount(TABLE_DOC, '/demo/columns.md')
      expect(firstColumnWidth(mounted.editor)).toBe(resized)
      expect(mounted.editor.getMarkdown()).toBe(TABLE_DOC)
    } finally {
      await mounted.cleanup()
    }
  })

  it('keeps widths distinct when concatenated cell text would collide', async () => {
    const source = [
      '| H | H |',
      '| --- | --- |',
      '| ab | c |',
      '',
      '| H | H |',
      '| --- | --- |',
      '| a | bc |',
      '',
    ].join('\n')
    let mounted = await mount(source, '/demo/signature.md')
    try {
      await dragFirstBoundary(mounted, 64, 0)
      const first = firstColumnWidth(mounted.editor, 0)
      await dragFirstBoundary(mounted, 128, 1)
      const second = firstColumnWidth(mounted.editor, 1)
      expect(first).not.toBeNull()
      expect(second).not.toBeNull()
      expect(second).not.toBe(first)

      await mounted.cleanup()
      mounted = await mount(source, '/demo/signature.md')
      expect(firstColumnWidth(mounted.editor, 0)).toBe(first)
      expect(firstColumnWidth(mounted.editor, 1)).toBe(second)
      expect(mounted.editor.getMarkdown()).toBe(source)
    } finally {
      await mounted.cleanup()
    }
  })
})
