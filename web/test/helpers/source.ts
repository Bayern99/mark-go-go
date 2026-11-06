import { EditorView } from '@codemirror/view'

/** The CodeMirror view behind the source-mode editor, resolved via public API. */
export function getSourceView(container: HTMLElement): EditorView {
  const element = container.querySelector<HTMLElement>('.mgo-source-root .cm-editor')
  if (!element) throw new Error('source editor is not mounted')
  const view = EditorView.findFromDOM(element)
  if (!view) throw new Error('source editor view not found')
  return view
}

export function sourceText(container: HTMLElement): string {
  return getSourceView(container).state.doc.toString()
}

/**
 * Replace a range the way a user edit does, so updates carry an input event
 * instead of looking like a programmatic document replacement.
 */
export function replaceSource(
  view: EditorView,
  from: number,
  to: number,
  insert: string,
): void {
  view.dispatch({
    changes: { from, to, insert },
    selection: { anchor: from + insert.length },
    userEvent: 'input.type',
    scrollIntoView: true,
  })
}

/** Replace the first occurrence of `search` in the source text. */
export function replaceSourceText(view: EditorView, search: string, insert: string): string {
  const text = view.state.doc.toString()
  const from = text.indexOf(search)
  if (from === -1) throw new Error(`source text not found: ${search}`)
  replaceSource(view, from, from + search.length, insert)
  return text.slice(0, from) + insert + text.slice(from + search.length)
}
