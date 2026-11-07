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
  /**
   * The caret or document moved; reports the exact CodeMirror offset where
   * the selection starts, as rich text reports the block of its `$from`.
   */
  onSelection(offset: number): void
}

export interface SourceEditor {
  getText(): string
  getSelection(): number
  /** Replace all text and reset the CodeMirror undo history. */
  setText(text: string): void
  setReadonly(value: boolean): void
  /**
   * Select from `from` to `to` (a caret when `to` is omitted), scroll the
   * line of `from` to the top of the view, and focus.
   */
  reveal(from: number, to?: number): boolean
  /** Insert text at `at` as an undoable edit. */
  insert(at: number, text: string): void
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

/** Space kept above a revealed line; matches scroll-margin-top for rich text in styles.css. */
const REVEAL_MARGIN_PX = 16

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
        // Navigation (outline, GenUI chips) should land the target at the top
        // like a table of contents, not on whichever edge is nearest.
        effects: EditorView.scrollIntoView(from, { y: 'start', yMargin: REVEAL_MARGIN_PX }),
      })
      view.focus()
      return true
    },
    insert(at, text) {
      view.dispatch({ changes: { from: at, insert: text } })
    },
    destroy() {
      view.destroy()
    },
  }
}
