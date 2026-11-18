import { EditorView } from '@codemirror/view'

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

export function visibleSource(container: HTMLElement): string {
  return sourceLines(container)
    .map((line) => line.textContent ?? '')
    .join('\n')
}

export function sourceLines(container: HTMLElement): HTMLElement[] {
  const content = container.querySelector<HTMLElement>('.mgo-source-root .cm-content')
  if (!content) throw new Error('source editor is not mounted')
  return Array.from(content.querySelectorAll<HTMLElement>('.cm-line'))
}

export function caretLineInSource(container: HTMLElement): HTMLElement | undefined {
  const anchor = window.getSelection()?.anchorNode
  const element = anchor instanceof Element ? anchor : anchor?.parentElement
  const line = element?.closest<HTMLElement>('.cm-line')
  return line && container.querySelector('.mgo-source-root')?.contains(line) ? line : undefined
}

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

export function replaceSourceText(view: EditorView, search: string, insert: string): string {
  const text = view.state.doc.toString()
  const from = text.indexOf(search)
  if (from === -1) throw new Error(`source text not found: ${search}`)
  replaceSource(view, from, from + search.length, insert)
  return text.slice(0, from) + insert + text.slice(from + search.length)
}
