import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { markdown } from '@codemirror/lang-markdown'
import { defaultHighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { Compartment, EditorState } from '@codemirror/state'
import { EditorView, keymap } from '@codemirror/view'

export interface SourceEditorOptions {
  parent: HTMLElement
  text: string
  readonly: boolean
  /** A user edit produced new document text. */
  onChange(text: string): void
  /** The caret or document moved; reports an exact CodeMirror offset. */
  onSelection(offset: number): void
}

export interface SourceEditor {
  getText(): string
  getSelection(): number
  /** Replace all text and reset the CodeMirror undo history. */
  setText(text: string): void
  setReadonly(value: boolean): void
  /** Select an exact offset, scroll it into view, and focus. */
  reveal(offset: number): boolean
  destroy(): void
}

const sourceTheme = EditorView.theme({
  '&': {
    minHeight: '100%',
    backgroundColor: 'transparent',
    fontSize: '14px',
  },
  '.cm-scroller': {
    fontFamily: "ui-monospace, 'SFMono-Regular', Menlo, monospace",
    lineHeight: '1.65',
  },
  '.cm-content': {
    padding: '24px 32px 64px',
  },
  '&.cm-focused': {
    outline: 'none',
  },
})

const readonlyExtensions = (value: boolean) => [
  EditorState.readOnly.of(value),
  EditorView.editable.of(!value),
]

export function createSourceEditor(options: SourceEditorOptions): SourceEditor {
  const readonlyCompartment = new Compartment()
  let readonly = options.readonly

  const createState = (text: string): EditorState =>
    EditorState.create({
      doc: text,
      extensions: [
        // Keep every line break exactly as it appears in the file: only LF
        // splits lines, so CR stays part of the preceding line's text.
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
            options.onSelection(update.state.selection.main.head)
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
    reveal(offset) {
      if (!Number.isFinite(offset)) return false
      const length = view.state.doc.length
      if (offset < 0 || offset > length) return false
      view.dispatch({
        selection: { anchor: offset },
        scrollIntoView: true,
      })
      view.focus()
      return true
    },
    destroy() {
      view.destroy()
    },
  }
}
