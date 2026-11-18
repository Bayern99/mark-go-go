import '../src/styles.css'
import type { Node as PMNode } from '@milkdown/kit/prose/model'
import type { MarkdownEditor } from '../src/editor/createEditor'
import { page, userEvent } from 'vitest/browser'
import { describe, expect, it, vi } from 'vitest'
import { mountEditor, replaceParagraph } from './helpers/editor'
import { mountApp } from './helpers/app'
import { MOD } from './helpers/platform'
import techReport from '../../docs/editor-typeset/fixtures/tech-report-tables.md?raw'

const RAGGED_DOC = `# Ragged

A paragraph with _emphasis_ and *stars*.

* bullet one
* bullet two

| a   | b      |
|  ---|     ---|
|  x  |   y    |
`

const TWIN_TABLE_DOC = `# Twins

| a   | b      |
| --- | ------ |
| 1   | 2      |

|a|b|
|---|---|
|1|2|
`

const TWIN_A_SOURCE = '| a   | b      |\n| --- | ------ |\n| 1   | 2      |'
const TWIN_B_SOURCE = '|a|b|\n|---|---|\n|1|2|'

const occurrences = (text: string, needle: string) => text.split(needle).length - 1

function editFirstCell(editor: MarkdownEditor): void {
  const view = editor.getView()
  let position = -1
  view.state.doc.descendants((node: PMNode, pos: number) => {
    if (position !== -1) return false
    if (node.type.name === 'table_cell' || node.type.name === 'table_header') {
      position = pos + 2
      return false
    }
    return true
  })
  if (position === -1) throw new Error('no table cell found')
  view.dispatch(view.state.tr.insertText('Z', position))
}

describe('undo restores original bytes', () => {
  it('keeps the tech-report fixture byte-identical through edit/undo/redo/undo', async () => {
    const mounted = await mountEditor(techReport)
    try {
      editFirstCell(mounted.editor)
      const edited = mounted.editor.getMarkdown()
      expect(edited).not.toBe(techReport)

      expect(mounted.editor.undo()).toBe(true)
      expect(mounted.editor.getMarkdown()).toBe(techReport)

      expect(mounted.editor.redo()).toBe(true)
      expect(mounted.editor.getMarkdown()).toBe(edited)

      expect(mounted.editor.undo()).toBe(true)
      expect(mounted.editor.getMarkdown()).toBe(techReport)
    } finally {
      await mounted.cleanup()
    }
  })

  it('keeps ragged table padding and emphasis markers through undo', async () => {
    const mounted = await mountEditor(RAGGED_DOC)
    try {
      editFirstCell(mounted.editor)
      expect(mounted.editor.getMarkdown()).not.toBe(RAGGED_DOC)
      mounted.editor.undo()
      expect(mounted.editor.getMarkdown()).toBe(RAGGED_DOC)

      replaceParagraph(mounted.editor, 'A paragraph with', 'A paragraph with edits')
      expect(mounted.editor.getMarkdown()).toContain('edits')
      mounted.editor.undo()
      expect(mounted.editor.getMarkdown()).toBe(RAGGED_DOC)
    } finally {
      await mounted.cleanup()
    }
  })

  it('keeps each twin table on its own bytes when an unbound copy lands on the other twin', async () => {
    const mounted = await mountEditor(TWIN_TABLE_DOC)
    try {
      const view = mounted.editor.getView()
      const doc = view.state.doc
      const tableA = doc.child(1)
      const aPos = doc.child(0).nodeSize
      const clone = tableA.type.create(tableA.attrs, tableA.content, tableA.marks)
      const tr = view.state.tr.delete(aPos, aPos + tableA.nodeSize)
      tr.insert(tr.doc.content.size, clone)
      view.dispatch(tr)

      const md = mounted.editor.getMarkdown()
      expect(occurrences(md, TWIN_A_SOURCE)).toBe(1)
      expect(occurrences(md, TWIN_B_SOURCE)).toBe(1)
      expect(md.indexOf(TWIN_B_SOURCE)).toBeLessThan(md.indexOf(TWIN_A_SOURCE))
    } finally {
      await mounted.cleanup()
    }
  })

  it('keeps the surviving twin on its own bytes after deleting the other', async () => {
    const mounted = await mountEditor(TWIN_TABLE_DOC)
    try {
      const view = mounted.editor.getView()
      const doc = view.state.doc
      const t2 = doc.child(2)
      const t2pos = doc.child(0).nodeSize + doc.child(1).nodeSize
      view.dispatch(view.state.tr.delete(t2pos, t2pos + t2.nodeSize))
      const md = mounted.editor.getMarkdown()
      expect(occurrences(md, TWIN_A_SOURCE)).toBe(1)
      expect(md).not.toContain(TWIN_B_SOURCE)

      mounted.editor.undo()
      expect(mounted.editor.getMarkdown()).toBe(TWIN_TABLE_DOC)
    } finally {
      await mounted.cleanup()
    }
  })

  it('serializes a duplicated copy instead of re-emitting a twin', async () => {
    const mounted = await mountEditor(TWIN_TABLE_DOC)
    try {
      const view = mounted.editor.getView()
      const tableA = view.state.doc.child(1)
      const clone = tableA.type.create(tableA.attrs, tableA.content, tableA.marks)
      view.dispatch(view.state.tr.insert(view.state.doc.content.size, clone))
      const md = mounted.editor.getMarkdown()
      expect(occurrences(md, TWIN_A_SOURCE)).toBe(1)
      expect(occurrences(md, TWIN_B_SOURCE)).toBe(1)
    } finally {
      await mounted.cleanup()
    }
  })

  it('restores both twins byte-identically after a whole-document undo', async () => {
    const mounted = await mountEditor(TWIN_TABLE_DOC)
    try {
      const view = mounted.editor.getView()
      view.dispatch(view.state.tr.delete(0, view.state.doc.content.size))
      mounted.editor.undo()
      expect(mounted.editor.getMarkdown()).toBe(TWIN_TABLE_DOC)
    } finally {
      await mounted.cleanup()
    }
  })

  it('clears the edited flag in a full App flow once undo restores the bytes', async () => {
    const app = await mountApp(RAGGED_DOC)
    try {
      await userEvent.click(page.getByRole('cell').first())
      await userEvent.keyboard('Z')
      await vi.waitFor(
        async () => {
          const snap = await app.host.load!()
          expect(snap.text).not.toBe(RAGGED_DOC)
        },
        { timeout: 3000 },
      )
      let snap = await app.host.load!()
      expect(snap.state.edited).toBe(true)

      await userEvent.keyboard(`{${MOD}>}z{/${MOD}}`)
      await vi.waitFor(
        async () => {
          snap = await app.host.load!()
          expect(snap.text).toBe(RAGGED_DOC)
        },
        { timeout: 3000 },
      )
      expect(snap.state.edited).toBe(false)
    } finally {
      await app.unmount()
    }
  })
})
