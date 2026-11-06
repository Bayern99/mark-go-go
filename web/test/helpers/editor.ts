import { createMarkdownEditor, type MarkdownEditor } from '../../src/editor/createEditor'

export interface MountedEditor {
  editor: MarkdownEditor
  root: HTMLDivElement
  cleanup(): Promise<void>
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
