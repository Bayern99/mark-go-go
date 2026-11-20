import { defaultKeymap, history, historyKeymap, isolateHistory, redo, redoDepth, undo, undoDepth } from '@codemirror/commands'
import { markdown } from '@codemirror/lang-markdown'
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { Compartment, EditorState } from '@codemirror/state'
import { EditorView, keymap } from '@codemirror/view'
import { tags } from '@lezer/highlight'

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
  replace(text: string): void
  insert(at: number, text: string): void
  canUndo(): boolean
  canRedo(): boolean
  undo(): boolean
  redo(): boolean
  destroy(): void
}

const portraitHighlightStyle = HighlightStyle.define([
  { tag: [tags.heading, tags.heading1, tags.heading2, tags.heading3, tags.heading4, tags.heading5, tags.heading6], color: 'var(--ink)', fontWeight: '600', textDecoration: 'none' },
  { tag: [tags.processingInstruction, tags.punctuation, tags.meta, tags.contentSeparator], color: 'var(--ink-4)' },
  { tag: tags.quote, color: 'var(--ink-2)' },
  { tag: tags.strong, color: 'var(--ink)', fontWeight: '600' },
  { tag: tags.emphasis, color: 'var(--ink-2)', fontStyle: 'italic' },
  { tag: tags.strikethrough, color: 'var(--ink-3)', textDecoration: 'line-through' },
  { tag: [tags.link, tags.url], color: 'var(--accent)', textDecoration: 'none' },
  { tag: [tags.monospace, tags.labelName, tags.keyword], color: 'var(--accent)' },
  { tag: tags.string, color: 'var(--content-note)' },
  { tag: [tags.number, tags.bool, tags.atom], color: 'var(--content-warning)' },
  { tag: tags.comment, color: 'var(--ink-3)', fontStyle: 'italic' },
  { tag: tags.invalid, color: 'var(--danger)' },
])

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
    caretColor: 'var(--accent)',
  },
  '.cm-cursor, .cm-dropCursor': {
    borderLeftColor: 'var(--accent)',
    borderLeftWidth: '1.5px',
  },
  '&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection': {
    backgroundColor: 'var(--accent-soft)',
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
        syntaxHighlighting(portraitHighlightStyle, { fallback: true }),
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
    replace(text) {
      view.dispatch({
        changes: { from: 0, to: view.state.doc.length, insert: text },
        annotations: isolateHistory.of('full'),
      })
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
