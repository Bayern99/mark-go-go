import { createMarkdownEditor, type MarkdownEditor } from '../../src/editor/createEditor'

export interface MountedEditor {
  editor: MarkdownEditor
  root: HTMLDivElement
  cleanup(): Promise<void>
}

export function replaceParagraph(
  editor: MarkdownEditor,
  oldText: string,
  newText: string,
): void {
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

export async function mountEditor(text: string): Promise<MountedEditor> {
  const root = document.createElement('div')
  document.body.append(root)
  const editor = await createMarkdownEditor(root, { initialText: text })
  return {
    editor,
    root,
    async cleanup() {
      await editor.destroy()
      root.remove()
    },
  }
}
