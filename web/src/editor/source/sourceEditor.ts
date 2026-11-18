import { defaultKeymap, history, historyKeymap, redo, redoDepth, undo, undoDepth } from '@codemirror/commands'
import { markdown } from '@codemirror/lang-markdown'
import { defaultHighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { Compartment, EditorState } from '@codemirror/state'
import { EditorView, keymap } from '@codemirror/view'

export interface SourceEditorOptions {
  parent: HTMLElement
  text: string
  readonly: boolean
  onChange(text: string): void
  onSelection(offset: number): void
}

export interface SourceEditor {
  getText(): string
  getSelection(): number
  setText(text: string): void
  setReadonly(value: boolean): void
  reveal(from: number, to?: number): boolean
  insert(at: number, text: string): void
  canUndo(): boolean
  canRedo(): boolean
  undo(): boolean
  redo(): boolean
  destroy(): void
}

const sourceTheme = EditorView.theme({
  '&': {
    minHeight: '100%',
    backgroundColor: 'transparent',
    fontSize: '13px',
    color: 'var(--ink)',
  },
  '.cm-scroller': {
    fontFamily: 'var(--font-mono)',
    lineHeight: '22px',
  },
  '.cm-content': {
    padding: '44px 40px 120px',
    maxWidth: '720px',
    margin: '0 auto',
    caretColor: 'var(--ink)',
  },
  '&.cm-focused': {
    outline: 'none',
  },
})

const readonlyExtensions = (value: boolean) => [
  EditorState.readOnly.of(value),
  EditorView.editable.of(!value),
]

const REVEAL_MARGIN_PX = 16

export function createSourceEditor(options: SourceEditorOptions): SourceEditor {
  const readonlyCompartment = new Compartment()
  let readonly = options.readonly

  const createState = (text: string): EditorState =>
    EditorState.create({
      doc: text,
      extensions: [
        EditorState.lineSeparator.of('\n'),
        markdown(),
        syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
        history(),
        keymap.of([...defaultKeymap, ...historyKeymap]),
        EditorView.lineWrapping,
        readonlyCompartment.of(readonlyExtensions(readonly)),
        sourceTheme,
        EditorView.updateListener.of((update) => {
          if (update.docChanged) options.onChange(update.state.doc.toString())
          if (update.docChanged || update.selectionSet) {
            options.onSelection(update.state.selection.main.from)
          }
        }),
      ],
    })

  const view = new EditorView({ state: createState(options.text), parent: options.parent })

  return {
    getText: () => view.state.doc.toString(),
    getSelection: () => view.state.selection.main.head,
    setText(text) {
      view.setState(createState(text))
    },
    setReadonly(value) {
      if (readonly === value) return
      readonly = value
      view.dispatch({
        effects: readonlyCompartment.reconfigure(readonlyExtensions(value)),
      })
    },
    reveal(from, to = from) {
      if (!Number.isFinite(from) || !Number.isFinite(to)) return false
      const length = view.state.doc.length
      if (from < 0 || to < from || to > length) return false
      view.dispatch({
        selection: { anchor: from, head: to },
        effects: EditorView.scrollIntoView(from, { y: 'start', yMargin: REVEAL_MARGIN_PX }),
      })
      view.focus()
      return true
    },
    insert(at, text) {
      view.dispatch({ changes: { from: at, insert: text } })
    },
    canUndo() {
      return undoDepth(view.state) > 0
    },
    canRedo() {
      return redoDepth(view.state) > 0
    },
    undo() {
      return undo({ state: view.state, dispatch: view.dispatch })
    },
    redo() {
      return redo({ state: view.state, dispatch: view.dispatch })
    },
    destroy() {
      view.destroy()
    },
  }
}
