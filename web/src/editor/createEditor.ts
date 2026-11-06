import { CrepeBuilder } from '@milkdown/crepe/builder'
import { blockEdit } from '@milkdown/crepe/feature/block-edit'
import { codeMirror } from '@milkdown/crepe/feature/code-mirror'
import { cursor } from '@milkdown/crepe/feature/cursor'
import { imageBlock } from '@milkdown/crepe/feature/image-block'
import { linkTooltip } from '@milkdown/crepe/feature/link-tooltip'
import { listItem } from '@milkdown/crepe/feature/list-item'
import { placeholder } from '@milkdown/crepe/feature/placeholder'
import { table } from '@milkdown/crepe/feature/table'
import { toolbar } from '@milkdown/crepe/feature/toolbar'
import '@milkdown/crepe/theme/common/style.css'
import '@milkdown/crepe/theme/classic.css'
import {
  editorViewCtx,
  parserCtx,
  schemaCtx,
  serializerCtx,
} from '@milkdown/kit/core'
import type { Node as PMNode } from '@milkdown/kit/prose/model'
import {
  EditorState,
  NodeSelection,
  Plugin,
  Selection,
} from '@milkdown/kit/prose/state'
import type { EditorView } from '@milkdown/kit/prose/view'
import { listenerCtx } from '@milkdown/kit/plugin/listener'
import type { Disposable } from '../host/types'
import { buildDocument, type BuiltDocument } from './blocks/build'
import {
  blockAtOffset,
  composeWithRanges,
  type ComposedDocument,
  type ComposeContext,
} from './blocks/compose'
import { LOCKED_NODE, lockedBlockSchema } from './blocks/locked'
import { languages } from './languages'
import { createSourceEditor, type SourceEditor } from './source/sourceEditor'

/**
 * Plugins that rewrite the document on load or after transactions. They break
 * node identity, which is what lets compose() emit untouched blocks byte for
 * byte (heading ids, renumbered lists, an auto-appended trailing paragraph).
 */
const IDENTITY_BREAKING_PLUGIN_KEYS: Record<string, true> = {
  MILKDOWN_HEADING_ID: true,
  MILKDOWN_KEEP_LIST_ORDER: true,
  MILKDOWN_TRAILING: true,
}

export type EditorMode = 'wysiwyg' | 'source'

export interface MarkdownEditor {
  /** Current editor surface: rich text or raw Markdown source. */
  getMode(): EditorMode
  /** Switch mode; a no-op when the mode is already active. */
  setMode(mode: EditorMode): void
  /** Switch between rich text and source. */
  toggleMode(): void
  /** Fires after the active mode changes. */
  onModeChange(fn: (mode: EditorMode) => void): Disposable
  /** Composed Markdown in rich text mode, the exact CodeMirror text in source. */
  getMarkdown(): string
  /** Replace the editor content (external reload). Undo history is reset. */
  setMarkdown(text: string): void
  /** Fires after every user edit with freshly composed Markdown. */
  onChange(fn: (markdown: string) => void): Disposable
  /**
   * Place the cursor at the start of the top-level block containing a Markdown
   * source offset and scroll that block to the top of the editor's scroll area.
   * Returns false when the offset falls outside the current composed text.
   */
  reveal(offset: number): boolean
  /**
   * Fires with the source start offset of the cursor's top-level block whenever
   * the selection changes block or an edit moves that offset. Also reports the
   * current offset synchronously when the listener is registered, so views can
   * highlight the current section immediately.
   */
  onSelectionChange(fn: (offset: number) => void): Disposable
  setReadonly(readonly: boolean): void
  /** Low-level access for tests and host integrations. */
  getView(): EditorView
  destroy(): Promise<void>
}

export interface CreateEditorOptions {
  initialText?: string
  onChange?: (markdown: string) => void
}

export async function createMarkdownEditor(
  root: HTMLElement,
  options: CreateEditorOptions = {},
): Promise<MarkdownEditor> {
  const wysiwygRoot = document.createElement('div')
  wysiwygRoot.className = 'mgo-wysiwyg-root'
  const sourceRoot = document.createElement('div')
  sourceRoot.className = 'mgo-source-root'
  sourceRoot.hidden = true
  root.append(wysiwygRoot, sourceRoot)

  const builder = new CrepeBuilder({ root: wysiwygRoot, defaultValue: '' })
    .addFeature(cursor)
    .addFeature(listItem)
    .addFeature(placeholder, { text: 'Start writing…', mode: 'doc' })
    .addFeature(linkTooltip)
    .addFeature(blockEdit)
    .addFeature(table)
    .addFeature(imageBlock)
    .addFeature(toolbar)
    .addFeature(codeMirror, { languages })

  builder.editor.use(lockedBlockSchema)

  const editor = await builder.create()
  const view = editor.action((ctx) => ctx.get(editorViewCtx))
  const schema = editor.action((ctx) => ctx.get(schemaCtx))
  const parser = editor.action((ctx) => ctx.get(parserCtx))
  const serializer = editor.action((ctx) => ctx.get(serializerCtx))

  let built = buildDocument(options.initialText ?? '', schema, parser)
  let suppressChange = false
  let suppressSelectionReporting = false
  let readonly = false
  const listeners = new Set<(markdown: string) => void>()
  const selectionListeners = new Set<(offset: number) => void>()
  const modeListeners = new Set<(mode: EditorMode) => void>()
  let lastReportedOffset: number | undefined
  let mode: EditorMode = 'wysiwyg'
  let sourceEditor: SourceEditor | undefined
  /** Text the rich-text editor represented when source mode was entered. */
  let sourceBaseText: string | undefined

  const composeContext = (current: typeof built): ComposeContext => ({
    schema,
    serializer,
    origins: current.origins,
    segments: current.segments,
    prefix: current.prefix,
    trailing: current.trailing,
  })

  /**
   * Composing re-serializes every edited block, so keep the last result while
   * the ProseMirror document (and load baseline) is unchanged. Selection-only
   * transactions reuse it, which keeps cursor movement O(1).
   */
  let composedCache:
    | { built: BuiltDocument; doc: PMNode; composed: ComposedDocument }
    | undefined

  const composed = (doc: PMNode): ComposedDocument => {
    if (composedCache && composedCache.built === built && composedCache.doc === doc) {
      return composedCache.composed
    }
    const result = composeWithRanges(doc, composeContext(built))
    composedCache = { built, doc, composed: result }
    return result
  }

  const compose = (doc: PMNode): string => composed(doc).text

  /** Source start offset of the top-level block holding the current selection. */
  const selectionOffset = (state: EditorState): number | undefined => {
    const { blocks } = composed(state.doc)
    if (blocks.length === 0 || state.doc.childCount === 0) return undefined

    const { $from } = state.selection
    // Text selections live inside a top-level block; document-level selections
    // (node selection, gap cursor, select-all) index the doc itself. Never
    // compare a ProseMirror position with a Markdown offset.
    const top =
      $from.depth >= 1
        ? $from.node(1)
        : state.doc.child(Math.min($from.index(0), state.doc.childCount - 1))
    return blocks.find((block) => block.node === top)?.start
  }

  const emitSelection = (offset: number) => {
    lastReportedOffset = offset
    for (const fn of selectionListeners) fn(offset)
  }

  const reportSelection = (state: EditorState) => {
    if (suppressSelectionReporting || selectionListeners.size === 0) return
    const offset = selectionOffset(state)
    if (offset === undefined || offset === lastReportedOffset) return
    emitSelection(offset)
  }

  const ensureSourceEditor = (text: string): SourceEditor => {
    if (sourceEditor) {
      sourceEditor.setText(text)
      return sourceEditor
    }
    sourceEditor = createSourceEditor({
      parent: sourceRoot,
      text,
      readonly,
      onChange(markdown) {
        for (const fn of listeners) fn(markdown)
      },
      onSelection(offset) {
        if (selectionListeners.size === 0 || offset === lastReportedOffset) return
        emitSelection(offset)
      },
    })
    return sourceEditor
  }

  const revealSource = (offset: number, alwaysReport: boolean): boolean => {
    if (!sourceEditor) return false
    lastReportedOffset = undefined
    if (!sourceEditor.reveal(offset)) return false
    // The dispatch reports through the update listener unless the selection
    // was already exactly there; a switch must still emit the current offset.
    if (alwaysReport && lastReportedOffset !== offset) emitSelection(offset)
    return true
  }

  const selectionPlugin = new Plugin({
    view: () => ({
      update(currentView, prevState) {
        const state = currentView.state
        if (state.doc === prevState.doc && state.selection.eq(prevState.selection)) return
        reportSelection(state)
      },
    }),
  })

  const plugins = view.state.plugins.filter((plugin) => !isIdentityBreaking(plugin))
  plugins.push(selectionPlugin)

  const load = (text: string) => {
    suppressChange = true
    try {
      built = buildDocument(text, schema, parser)
      composedCache = undefined
      const state = EditorState.create({
        schema,
        doc: built.doc,
        plugins,
        selection: Selection.near(built.doc.resolve(1), 1),
      })
      view.updateState(state)
    } finally {
      suppressChange = false
    }
  }

  const notify = (markdown: string) => {
    if (suppressChange) return
    for (const fn of listeners) fn(markdown)
  }

  editor.action((ctx) => {
    ctx.get(listenerCtx).markdownUpdated(() => {
      notify(compose(view.state.doc))
    })
  })

  load(options.initialText ?? '')
  if (options.onChange) listeners.add(options.onChange)

  /**
   * Resolve an offset against the *current* composed text, then select the
   * start of the corresponding top-level block. Atom blocks (frontmatter,
   * alerts, ...) get a node selection; everything else a text cursor.
   */
  const reveal = (offset: number): boolean => {
    if (mode === 'source') return revealSource(offset, true)
    if (!Number.isFinite(offset)) return false
    const { text, blocks } = composed(view.state.doc)
    if (offset < 0 || offset > text.length) return false

    const block = blockAtOffset(blocks, offset)
    // Empty or whitespace-only documents compose to no blocks; the invisible
    // empty paragraph is still a valid place for the cursor.
    const pos = block ? topLevelPosition(view.state.doc, block.node) : 1
    if (pos === undefined) return false

    const selection =
      block && block.node.type.name === LOCKED_NODE
        ? NodeSelection.create(view.state.doc, pos)
        : Selection.near(
            view.state.doc.resolve(Math.min(pos + 1, view.state.doc.content.size)),
            1,
          )

    // Revealing is an explicit navigation request: report the block even when
    // the cursor was already inside it. Clearing the dedupe first lets the
    // selection plugin do the reporting when the selection actually moves.
    lastReportedOffset = undefined
    // A caller may still be focused in source mode or on a toolbar button;
    // focus first so the selection lands in the ProseMirror DOM.
    view.focus()
    view.dispatch(view.state.tr.setSelection(selection))
    // Same top alignment as the source view; scroll-margin-top in styles.css
    // keeps the gap above the block. Blocks without their own DOM node fall
    // back to ProseMirror's nearest-edge scroll.
    const target = view.nodeDOM(pos)
    if (target instanceof HTMLElement) target.scrollIntoView({ block: 'start' })
    else view.dispatch(view.state.tr.scrollIntoView())
    if (block && lastReportedOffset !== block.start) emitSelection(block.start)
    return true
  }

  const setMode = (next: EditorMode) => {
    if (next === mode) return

    if (next === 'source') {
      const text = compose(view.state.doc)
      const offset = selectionOffset(view.state) ?? 0
      mode = 'source'
      sourceBaseText = text
      wysiwygRoot.hidden = true
      sourceRoot.hidden = false
      ensureSourceEditor(text)
      revealSource(offset, true)
    } else {
      const text = sourceEditor?.getText() ?? ''
      const offset = sourceEditor?.getSelection() ?? 0
      mode = 'wysiwyg'
      sourceRoot.hidden = true
      wysiwygRoot.hidden = false
      // Untouched source keeps the existing ProseMirror state (and its undo
      // history); edits rebuild through the normal load pipeline.
      if (sourceBaseText === undefined || text !== sourceBaseText) {
        // load() briefly parks the selection at the document start; only the
        // reveal below is a real cursor move for listeners.
        suppressSelectionReporting = true
        try {
          load(text)
        } finally {
          suppressSelectionReporting = false
        }
      }
      sourceBaseText = undefined
      if (!reveal(offset)) reveal(0)
    }

    for (const fn of modeListeners) fn(mode)
  }

  const setReadonly = (value: boolean) => {
    if (readonly === value) return
    readonly = value
    builder.setReadonly(value)
    sourceEditor?.setReadonly(value)
  }

  return {
    getMode: () => mode,
    setMode,
    toggleMode: () => setMode(mode === 'wysiwyg' ? 'source' : 'wysiwyg'),
    onModeChange(fn) {
      modeListeners.add(fn)
      return {
        dispose() {
          modeListeners.delete(fn)
        },
      }
    },
    getMarkdown: () =>
      mode === 'source' ? (sourceEditor?.getText() ?? '') : compose(view.state.doc),
    setMarkdown(text) {
      if (mode === 'source') {
        // Any reload invalidates the hidden rich-text copy and its undo
        // history; leaving source must rebuild from this exact text.
        sourceBaseText = undefined
        const source = ensureSourceEditor(text)
        // setState resets the CodeMirror caret without an update event, so
        // listeners would otherwise keep a stale offset and drop the next move
        // back to it.
        lastReportedOffset = undefined
        emitSelection(source.getSelection())
        return
      }
      load(text)
    },
    onChange(fn) {
      listeners.add(fn)
      return {
        dispose() {
          listeners.delete(fn)
        },
      }
    },
    reveal,
    onSelectionChange(fn) {
      selectionListeners.add(fn)
      // Emit the current offset to the new listener only. Mutating the shared
      // dedupe would make listeners registered at different moments suppress
      // each other's next real selection event.
      const offset =
        mode === 'source'
          ? sourceEditor?.getSelection()
          : selectionOffset(view.state)
      if (offset !== undefined) fn(offset)
      return {
        dispose() {
          selectionListeners.delete(fn)
        },
      }
    },
    setReadonly,
    getView: () => view,
    async destroy() {
      listeners.clear()
      selectionListeners.clear()
      modeListeners.clear()
      sourceEditor?.destroy()
      sourceEditor = undefined
      await builder.destroy()
    },
  }
}

function isIdentityBreaking(plugin: Plugin): boolean {
  const key = pluginKey(plugin)
  if (key === undefined) return false
  // ProseMirror appends a suffix when the same key name is registered twice.
  return IDENTITY_BREAKING_PLUGIN_KEYS[key.replace(/\$\d*$/, '')] === true
}

function pluginKey(plugin: Plugin): string | undefined {
  // ProseMirror stores the plugin key name as a plain string on `Plugin.key`,
  // but its typings do not declare that field.
  if (!('key' in plugin)) return undefined
  const key: unknown = plugin.key
  return typeof key === 'string' ? key : undefined
}

function topLevelPosition(doc: PMNode, target: PMNode): number | undefined {
  let found: number | undefined
  doc.forEach((node, offset) => {
    if (found === undefined && node === target) found = offset
  })
  return found
}
