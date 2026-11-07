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
import './crepe-theme.css'
import {
  commandsCtx,
  editorViewCtx,
  parserCtx,
  schemaCtx,
  serializerCtx,
} from '@milkdown/kit/core'
import { clearTextInCurrentBlockCommand } from '@milkdown/kit/preset/commonmark'
import type { Node as PMNode } from '@milkdown/kit/prose/model'
import {
  EditorState,
  NodeSelection,
  Plugin,
  Selection,
  TextSelection,
} from '@milkdown/kit/prose/state'
import { Decoration, DecorationSet, type EditorView } from '@milkdown/kit/prose/view'
import { listenerCtx } from '@milkdown/kit/plugin/listener'
import type { Disposable } from '../host/types'
import { buildDocument, type BuiltDocument } from './blocks/build'
import {
  blockAtOffset,
  composeWithRanges,
  type ComposedBlock,
  type ComposedDocument,
  type ComposeContext,
} from './blocks/compose'
import { LOCKED_NODE, lockedBlockSchema } from './blocks/locked'
import { footnoteReferenceView, lockedBlockView } from './blocks/lockedView'
import { rebuildBlocks } from './blocks/rebuild'
import type { LockKind } from './blocks/segment'
import { noteBlock, noteInsertionOffset, parseNote, withNoteText } from '../markdown/notes'
import { languages } from './languages'
import { askForMarkdown, askForNote, cancelNoteComposer } from './noteDialog'
import { createSourceEditor, type SourceEditor } from './source/sourceEditor'

const NOTE_ICON =
  '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 5h14v10H10l-5 4z"/></svg>'

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
   * Select a Markdown source range and scroll its start to the top of the
   * editor's scroll area. Rich text selects whole top-level blocks: from the
   * block holding `from` through the last block starting before `to`; source
   * selects the exact offsets. Returns false when the range falls outside the
   * current composed text.
   */
  select(from: number, to: number): boolean
  /**
   * Add a note as a normal, undoable edit, in rich text or source: right after
   * the last top-level block that starts before the source offset. Returns
   * false when the document is read only or no block starts before the offset.
   */
  addNote(offset: number, text: string): boolean
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

  /** Set once the view exists; the block menu can only open after that. */
  let addNoteFromMenu: () => Promise<void> = () => Promise.resolve()

  const builder = new CrepeBuilder({ root: wysiwygRoot, defaultValue: '' })
    .addFeature(cursor)
    .addFeature(listItem)
    .addFeature(placeholder, { text: 'Start writing…', mode: 'doc' })
    .addFeature(linkTooltip)
    .addFeature(blockEdit, {
      buildMenu: (menu) => {
        menu.addGroup('review', 'Review').addItem('note', {
          label: 'Add note',
          icon: NOTE_ICON,
          onRun: (ctx) => {
            ctx.get(commandsCtx).call(clearTextInCurrentBlockCommand.key)
            void addNoteFromMenu()
          },
        })
      },
    })
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

  const revealSource = (from: number, to: number, alwaysReport: boolean): boolean => {
    if (!sourceEditor) return false
    lastReportedOffset = undefined
    if (!sourceEditor.reveal(from, to)) return false
    // The dispatch reports through the update listener unless the selection
    // was already exactly there; a switch must still emit the current offset.
    if (alwaysReport && lastReportedOffset !== from) emitSelection(from)
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

  const noteNode = (text: string): PMNode =>
    schema.nodes[LOCKED_NODE].create({ source: noteBlock(text), kind: 'note' })

  /**
   * A note in place of the empty paragraph the block menu was opened in, which
   * is right after the block it was opened for; else after the cursor's block.
   */
  const insertNoteAtCursor = (text: string) => {
    const { state } = view
    const { $from } = state.selection
    if ($from.depth < 1) return
    const top = $from.node(1)
    const pos = $from.before(1)
    view.dispatch(
      isEmptyParagraph(top)
        ? state.tr.replaceWith(pos, pos + top.nodeSize, noteNode(text))
        : state.tr.insert(pos + top.nodeSize, noteNode(text)),
    )
  }

  /** Leave no empty paragraph behind when adding a note from the block menu is cancelled. */
  const dropEmptyParagraphAtCursor = () => {
    const { state } = view
    const { $from } = state.selection
    if ($from.depth < 1 || state.doc.childCount < 2 || !isEmptyParagraph($from.node(1))) return
    const pos = $from.before(1)
    view.dispatch(state.tr.delete(pos, pos + $from.node(1).nodeSize))
  }

  const mountComposer = (anchor: Element | null, composer: HTMLElement): (() => void) => {
    const observer = (view as { domObserver?: { stop(): void; start(): void } }).domObserver
    observer?.stop()
    if (anchor && anchor.parentElement === view.dom) {
      anchor.after(composer)
    } else {
      view.dom.append(composer)
    }
    return () => {
      composer.remove()
      observer?.start()
    }
  }

  addNoteFromMenu = async () => {
    if (readonly) return
    const { $from } = view.state.selection
    const targetNode = $from.depth >= 1 ? $from.node(1) : undefined
    const targetPos = $from.depth >= 1 ? $from.before(1) : undefined
    const anchor = targetPos !== undefined ? (view.nodeDOM(targetPos) as Element | null) : null
    const text = await askForNote('', {
      mount: (composer) => mountComposer(anchor, composer),
    })
    if (readonly) return
    if (text === null) {
      if (
        targetNode &&
        targetPos !== undefined &&
        view.state.doc.nodeAt(targetPos) === targetNode &&
        view.state.doc.childCount >= 2 &&
        isEmptyParagraph(targetNode)
      ) {
        view.dispatch(view.state.tr.delete(targetPos, targetPos + targetNode.nodeSize))
      } else {
        dropEmptyParagraphAtCursor()
      }
    } else if (
      targetNode &&
      targetPos !== undefined &&
      view.state.doc.nodeAt(targetPos) === targetNode
    ) {
      view.dispatch(
        isEmptyParagraph(targetNode)
          ? view.state.tr.replaceWith(targetPos, targetPos + targetNode.nodeSize, noteNode(text))
          : view.state.tr.insert(targetPos + targetNode.nodeSize, noteNode(text)),
      )
    } else {
      insertNoteAtCursor(text)
    }
    view.focus()
  }

  /**
   * A changed note keeps its place in the file: it stays the block loaded
   * there, so only its text changes and the bytes around it stay.
   */
  const setNoteText = (pos: number, text: string) => {
    const { state } = view
    const node = state.doc.nodeAt(pos)
    if (!node) return
    const source = withNoteText(node.attrs.source as string, text)
    const tr = state.tr.setNodeMarkup(pos, undefined, { ...node.attrs, source })
    const next = tr.doc.nodeAt(pos)
    const origin = built.origins.get(node)
    if (origin && next) built.origins.set(next, { ...origin, source, node: next })
    view.dispatch(tr)
  }

  const writeLockedSource = (pos: number, previous: PMNode, markdown: string) => {
    if (markdown === (previous.attrs.source as string)) return
    const rebuilt = rebuildBlocks(markdown, schema, parser)
    const nodes = rebuilt.map((block) => block.node)
    const tr = view.state.tr.replaceWith(pos, pos + previous.nodeSize, nodes)
    const origin = built.origins.get(previous)
    if (origin && rebuilt.length === 1) {
      const placed = tr.doc.nodeAt(pos)
      if (placed) built.origins.set(placed, { ...origin, source: rebuilt[0].source, node: placed })
    }
    view.dispatch(tr)
  }

  const handleNoteAction = (action: string, element: Element) => {
    if (readonly) return
    const at = positionOfDom(element)
    if (at === undefined) return
    const node = view.state.doc.nodeAt(at)
    if (!node) return
    if (action === 'delete') {
      cancelNoteComposer()
      view.dispatch(view.state.tr.delete(at, at + node.nodeSize))
      return
    }
    void askForNote(parseNote(node.attrs.source as string)?.text ?? '', {
      mount: (composer) => mountComposer(element, composer),
    }).then((text) => {
      if (text !== null && view.state.doc.nodeAt(at) === node) setNoteText(at, text)
    })
  }

  const handleBlockEdit = (element: Element) => {
    if (readonly) return
    const at = positionOfDom(element)
    if (at === undefined) return
    const node = view.state.doc.nodeAt(at)
    if (!node || node.type.name !== LOCKED_NODE) return
    const kind = node.attrs.kind as LockKind
    void askForMarkdown(node.attrs.source as string, {
      label: `Edit ${kind} as Markdown`,
      mount: (composer) => mountComposer(element, composer),
    }).then((markdown) => {
      if (markdown === null) return
      if (view.state.doc.nodeAt(at) !== node) return
      writeLockedSource(at, node, markdown)
    })
  }

  const positionOfDom = (element: Element): number | undefined => {
    let pos: number | undefined
    view.state.doc.forEach((_node, offset) => {
      if (view.nodeDOM(offset) === element) pos = offset
    })
    return pos
  }

  const hopFootnote = (target: Element): boolean => {
    const back = target.closest<HTMLElement>('[data-footnote-back]')
    if (back) {
      const label = back.dataset.footnoteBack ?? ''
      const dest = view.dom.querySelector<HTMLElement>(`[data-footnote-ref="${cssAttr(label)}"]`)
      dest?.scrollIntoView({ block: 'center' })
      dest?.focus()
      return true
    }
    const ref = target.closest<HTMLElement>('[data-footnote-ref]')
    if (ref) {
      const label = ref.dataset.footnoteRef ?? ''
      const dest = view.dom.querySelector<HTMLElement>(`[data-footnote-label="${cssAttr(label)}"]`)
      dest?.scrollIntoView({ block: 'start' })
      dest?.focus()
      return true
    }
    return false
  }

  const isInsideComposer = (event: Event): boolean =>
    event.target instanceof Element && event.target.closest('.composer') !== null

  const lockedViewPlugin = new Plugin({
    props: {
      decorations(state) {
        const widgets: Decoration[] = []
        const mark = /\[\^([^\]]+)\]/g
        state.doc.descendants((node, pos) => {
          if (node.type.name === LOCKED_NODE) return false
          if (!node.isTextblock) return true
          const text = node.textContent
          mark.lastIndex = 0
          let match: RegExpExecArray | null
          while ((match = mark.exec(text))) {
            const from = pos + 1 + match.index
            const to = from + match[0].length
            const label = match[1]
            widgets.push(
              Decoration.inline(from, to, {
                class: 'mgo-fnref',
                style: 'display:none',
              }),
            )
            widgets.push(
              Decoration.widget(from, () => {
                const link = document.createElement('a')
                link.className = 'mgo-fnref'
                link.id = `fnref-${cssSafe(label)}`
                link.href = `#fn-${cssSafe(label)}`
                link.dataset.footnoteRef = label
                link.textContent = label
                link.setAttribute('aria-label', `Footnote ${label}`)
                link.tabIndex = 0
                link.contentEditable = 'false'
                return link
              }, { side: -1, ignoreSelection: true, key: `fnref-${from}-${label}` }),
            )
          }
          return true
        })
        return DecorationSet.create(state.doc, widgets)
      },
      nodeViews: {
        [LOCKED_NODE]: (node, editorView) => lockedBlockView(node, editorView),
        footnote_reference: (node) => footnoteReferenceView(node),
      },
      handleDOMEvents: {
        keydown(_view, event) {
          return isInsideComposer(event)
        },
        keypress(_view, event) {
          return isInsideComposer(event)
        },
        keyup(_view, event) {
          return isInsideComposer(event)
        },
        beforeinput(_view, event) {
          return isInsideComposer(event)
        },
        input(_view, event) {
          return isInsideComposer(event)
        },
        mousedown(_view, event) {
          return isInsideComposer(event)
        },
        mouseup(_view, event) {
          return isInsideComposer(event)
        },
        click(_view, event) {
          if (isInsideComposer(event)) return true
          const target = event.target instanceof Element ? event.target : null
          if (!target) return false
          if (hopFootnote(target)) {
            event.preventDefault()
            return true
          }
          const blockButton = target.closest<HTMLElement>('[data-block-action]')
          const locked = blockButton?.closest('[data-locked-block]')
          if (blockButton && locked && blockButton.dataset.blockAction === 'edit') {
            event.preventDefault()
            handleBlockEdit(locked)
            return true
          }
          const button = target.closest<HTMLElement>('[data-note-action]')
          const note = button?.closest('[data-note]')
          if (!button || !note) return false
          event.preventDefault()
          handleNoteAction(button.dataset.noteAction ?? '', note)
          return true
        },
      },
    },
  })

  const addNote = (offset: number, text: string): boolean => {
    if (readonly || !Number.isFinite(offset) || text.trim() === '') return false
    if (mode === 'source') {
      if (!sourceEditor) return false
      const at = noteInsertionOffset(sourceEditor.getText(), offset)
      if (at === undefined) return false
      sourceEditor.insert(at, `\n\n${noteBlock(text)}`)
      return true
    }
    const { doc } = view.state
    const block = composed(doc).blocks.findLast((candidate) => candidate.start < offset)
    const pos = block && topLevelPosition(doc, block.node)
    if (!block || pos === undefined) return false
    view.dispatch(view.state.tr.insert(pos + block.node.nodeSize, noteNode(text)))
    return true
  }

  const plugins = view.state.plugins.filter((plugin) => !isIdentityBreaking(plugin))
  plugins.push(selectionPlugin, lockedViewPlugin)

  const load = (text: string) => {
    cancelNoteComposer()
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
   * Select `selection`, which starts at the top-level `block` at `pos`, and
   * scroll that block to the top. Navigating is an explicit request: the
   * block is reported even when the selection was already inside it.
   */
  const land = (selection: Selection, block: ComposedBlock | undefined, pos: number) => {
    // Clearing the dedupe first lets the selection plugin do the reporting
    // when the selection actually moves.
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
  }

  /**
   * Resolve an offset against the *current* composed text, then select the
   * start of the corresponding top-level block. Atom blocks (frontmatter,
   * alerts, ...) get a node selection; everything else a text cursor.
   */
  const reveal = (offset: number): boolean => {
    if (mode === 'source') return revealSource(offset, offset, true)
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
    land(selection, block, pos)
    return true
  }

  const select = (from: number, to: number): boolean => {
    if (!Number.isFinite(from) || !Number.isFinite(to) || to < from) return false
    if (mode === 'source') return revealSource(from, to, true)
    const { text, blocks } = composed(view.state.doc)
    if (from < 0 || to > text.length) return false

    const first = blockAtOffset(blocks, from)
    if (!first) return reveal(from)
    let last = first
    for (const block of blocks) {
      if (block.start >= to) break
      if (block.start > first.start) last = block
    }
    const doc = view.state.doc
    const start = topLevelPosition(doc, first.node)
    const lastStart = topLevelPosition(doc, last.node)
    if (start === undefined || lastStart === undefined) return false

    const selection =
      first === last && first.node.type.name === LOCKED_NODE
        ? NodeSelection.create(doc, start)
        : TextSelection.between(doc.resolve(start), doc.resolve(lastStart + last.node.nodeSize))
    land(selection, first, start)
    return true
  }

  const setMode = (next: EditorMode) => {
    if (next === mode) return
    cancelNoteComposer()

    if (next === 'source') {
      const text = compose(view.state.doc)
      const offset = selectionOffset(view.state) ?? 0
      mode = 'source'
      sourceBaseText = text
      wysiwygRoot.hidden = true
      sourceRoot.hidden = false
      ensureSourceEditor(text)
      revealSource(offset, offset, true)
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
    if (value) cancelNoteComposer()
    builder.setReadonly(value)
    sourceEditor?.setReadonly(value)
    view.dispatch(view.state.tr.setMeta('mgo-readonly', value))
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
    select,
    addNote,
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
      cancelNoteComposer()
      listeners.clear()
      selectionListeners.clear()
      modeListeners.clear()
      sourceEditor?.destroy()
      sourceEditor = undefined
      await builder.destroy()
    },
  }
}

function isEmptyParagraph(node: PMNode): boolean {
  return node.type.name === 'paragraph' && node.content.size === 0
}

function cssAttr(value: string): string {
  return value.replace(/\\/g, '\\\\').replace(/"/g, '\\"')
}

function cssSafe(label: string): string {
  return encodeURIComponent(label).replace(/%/g, '_')
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
