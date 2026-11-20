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
import {
  closeHistory,
  undo as undoRich,
  redo as redoRich,
  undoDepth,
  redoDepth,
} from '@milkdown/kit/prose/history'
import { listenerCtx } from '@milkdown/kit/plugin/listener'
import type { Disposable } from '../host/types'
import { notifyCopied } from '../components/feedback'
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
import { askForMarkdown, askForNote, ComposerScope } from './noteDialog'
import { createSourceEditor, type SourceEditor } from './source/sourceEditor'
import { createFoldFeature, foldPluginKey, type PersistedFolds } from './folding'
import { createCellDragPlugin } from './tableCellDrag'
import {
  createTableColumnWidthPlugins,
  readTableColumnWidths,
  restoreTableColumnWidths,
} from './tableColumnWidths'

const NOTE_ICON =
  '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 5h14v10H10l-5 4z"/></svg>'

const IDENTITY_BREAKING_PLUGIN_KEYS: Record<string, true> = {
  MILKDOWN_HEADING_ID: true,
  MILKDOWN_KEEP_LIST_ORDER: true,
  MILKDOWN_TRAILING: true,
}

export type EditorMode = 'preview' | 'source'

export interface MarkdownEditor {
  getMode(): EditorMode
  setMode(mode: EditorMode): void
  toggleMode(): void
  onModeChange(fn: (mode: EditorMode) => void): Disposable
  getMarkdown(): string
  applyMarkdown(expected: string, text: string): boolean
  setMarkdown(text: string): void
  onChange(fn: (markdown: string) => void): Disposable
  reveal(offset: number): boolean
  toggleFold(pos: number): void
  foldAnchors(): number[]
  select(from: number, to: number): boolean
  addNote(offset: number, text: string): boolean
  onSelectionChange(fn: (offset: number) => void): Disposable
  setReadonly(readonly: boolean): void
  canUndo(): boolean
  canRedo(): boolean
  undo(): boolean
  redo(): boolean
  onHistoryChange(fn: () => void): Disposable
  getView(): EditorView
  destroy(): Promise<void>
}

export interface CreateEditorOptions {
  initialText?: string
  onChange?: (markdown: string) => void
  docKey?: string
}

export async function createMarkdownEditor(
  root: HTMLElement,
  options: CreateEditorOptions = {},
): Promise<MarkdownEditor> {
  const previewRoot = document.createElement('div')
  previewRoot.className = 'mgo-preview-root'
  const sourceRoot = document.createElement('div')
  sourceRoot.className = 'mgo-source-root'
  sourceRoot.hidden = true
  root.append(previewRoot, sourceRoot)

  let addNoteFromMenu: () => Promise<void> = () => Promise.resolve()

  const builder = new CrepeBuilder({ root: previewRoot, defaultValue: '' })
    .addFeature(cursor)
    .addFeature(listItem)
    .addFeature(placeholder, { text: 'Start writing…', mode: 'doc' })
    .addFeature(linkTooltip, { onCopyLink: () => notifyCopied('Link') })
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
    .addFeature(codeMirror, { languages, onCopy: () => notifyCopied('Code') })

  builder.editor.use(lockedBlockSchema)

  const editor = await builder.create()
  const view = editor.action((ctx) => ctx.get(editorViewCtx))
  const schema = editor.action((ctx) => ctx.get(schemaCtx))
  const parser = editor.action((ctx) => ctx.get(parserCtx))
  const serializer = editor.action((ctx) => ctx.get(serializerCtx))

  let built = buildDocument(options.initialText ?? '', schema, parser)
  const builtSnapshots: Array<{ doc: PMNode; built: BuiltDocument }> = []
  let suppressChange = false
  let suppressSelectionReporting = false
  let readonly = false
  const listeners = new Set<(markdown: string) => void>()
  const selectionListeners = new Set<(offset: number) => void>()
  const modeListeners = new Set<(mode: EditorMode) => void>()
  const historyListeners = new Set<() => void>()
  let lastReportedOffset: number | undefined
  let mode: EditorMode = 'preview'
  let sourceEditor: SourceEditor | undefined
  let sourceBaseText: string | undefined
  const composerScope = new ComposerScope()

  const recordBuiltSnapshot = (doc: PMNode, targetBuilt: BuiltDocument) => {
    for (let i = builtSnapshots.length - 1; i >= 0; i -= 1) {
      if (sameTopLevelChildren(builtSnapshots[i].doc, doc)) {
        builtSnapshots.splice(i, 1)
      }
    }
    builtSnapshots.push({ doc, built: targetBuilt })
  }

  const resolveBuiltForDoc = (doc: PMNode): BuiltDocument => {
    for (let i = builtSnapshots.length - 1; i >= 0; i -= 1) {
      if (sameTopLevelChildren(builtSnapshots[i].doc, doc)) {
        built = builtSnapshots[i].built
        return built
      }
    }
    return built
  }

  const composeContext = (current: typeof built): ComposeContext => ({
    schema,
    serializer,
    origins: current.origins,
    loaded: current.loaded,
    segments: current.segments,
    prefix: current.prefix,
    trailing: current.trailing,
  })

  let composedCache:
    | { built: BuiltDocument; doc: PMNode; composed: ComposedDocument }
    | undefined

  const composed = (doc: PMNode): ComposedDocument => {
    const activeBuilt = resolveBuiltForDoc(doc)
    if (composedCache && composedCache.built === activeBuilt && composedCache.doc === doc) {
      return composedCache.composed
    }
    const result = composeWithRanges(doc, composeContext(activeBuilt))
    composedCache = { built: activeBuilt, doc, composed: result }
    return result
  }

  const compose = (doc: PMNode): string => composed(doc).text

  const selectionOffset = (state: EditorState): number | undefined => {
    const { blocks } = composed(state.doc)
    if (blocks.length === 0 || state.doc.childCount === 0) return undefined

    const { $from } = state.selection
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

  const emitHistory = () => {
    for (const fn of historyListeners) fn()
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
        emitHistory()
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
      scope: composerScope,
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
      composerScope.cancel()
      view.dispatch(view.state.tr.delete(at, at + node.nodeSize))
      return
    }
    void askForNote(parseNote(node.attrs.source as string)?.text ?? '', {
      mount: (composer) => mountComposer(element, composer),
      scope: composerScope,
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
      scope: composerScope,
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
                nodeName: 'span',
                class: 'mgo-fnref-src',
                style: 'font-size:0;width:0;height:0;overflow:hidden;white-space:nowrap',
              }),
            )
            widgets.push(
              Decoration.widget(
                from,
                () => {
                  const link = document.createElement('a')
                  link.className = 'mgo-fnref'
                  link.id = `fnref-${cssSafe(label)}`
                  link.href = `#fn-${cssSafe(label)}`
                  link.dataset.footnoteRef = label
                  link.textContent = label
                  link.setAttribute('aria-label', `Footnote ${label}`)
                  link.tabIndex = 0
                  link.contentEditable = 'false'
                  const wrap = document.createElement('sup')
                  wrap.className = 'mgo-fnref-wrap'
                  wrap.append(link)
                  return wrap
                },
                { side: -1, ignoreSelection: true, key: `fnref-${from}-${label}` },
              ),
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

  const notify = (markdown: string) => {
    if (suppressChange) return
    for (const fn of listeners) fn(markdown)
  }

  const historyWatch = new Plugin({
    view: () => ({
      update(currentView, prevState) {
        const { doc, selection } = currentView.state
        if (doc.eq(prevState.doc)) {
          if (!suppressChange && mode === 'preview' && !sameTopLevelChildren(doc, prevState.doc)) {
            notify(compose(doc))
          } else if (selection.eq(prevState.selection)) {
            return
          }
        }
        emitHistory()
      },
    }),
  })

  const tableCaretPlugin = new Plugin({
    filterTransaction(tr) {
      if (
        tr.docChanged ||
        !tr.selectionSet ||
        !(tr.selection instanceof NodeSelection) ||
        tr.selection.node.type.name !== 'paragraph' ||
        (tr.selection.$from.parent.type.name !== 'table_cell' &&
          tr.selection.$from.parent.type.name !== 'table_header')
      ) {
        return true
      }
      return false
    },
    view(editorView) {
      const onPointerDown = () => {
        previewRoot.classList.add('mgo-pointer-selecting')
        previewRoot.classList.remove('mgo-toolbar-dismissed')
      }
      const onKeyDown = (event: KeyboardEvent) => {
        if (event.key === 'Escape' && !editorView.state.selection.empty) {
          previewRoot.classList.add('mgo-toolbar-dismissed')
        }
      }
      const endPointerSelect = () => previewRoot.classList.remove('mgo-pointer-selecting')
      editorView.dom.addEventListener('pointerdown', onPointerDown, true)
      editorView.dom.addEventListener('keydown', onKeyDown)
      window.addEventListener('pointerup', endPointerSelect)
      window.addEventListener('pointercancel', endPointerSelect)
      window.addEventListener('blur', endPointerSelect)
      return {
        update(currentView, prevState) {
          if (!currentView.state.selection.eq(prevState.selection)) {
            previewRoot.classList.remove('mgo-toolbar-dismissed')
          }
        },
        destroy() {
          editorView.dom.removeEventListener('pointerdown', onPointerDown, true)
          editorView.dom.removeEventListener('keydown', onKeyDown)
          window.removeEventListener('pointerup', endPointerSelect)
          window.removeEventListener('pointercancel', endPointerSelect)
          window.removeEventListener('blur', endPointerSelect)
        },
      }
    },
  })

  const foldKey = options.docKey ? `mgo.folds:${options.docKey}` : null
  const persistedFolds = ((): PersistedFolds => {
    if (!foldKey) return { entries: [] }
    try {
      const raw = localStorage.getItem(foldKey)
      const parsed = raw ? (JSON.parse(raw) as { entries?: unknown }) : {}
      const entries = Array.isArray(parsed?.entries)
        ? parsed.entries.filter(
            (e): e is { base: string; idx: number; sig: string } =>
              typeof e === 'object' && e !== null &&
              typeof (e as { base?: unknown }).base === 'string' &&
              typeof (e as { idx?: unknown }).idx === 'number' &&
              typeof (e as { sig?: unknown }).sig === 'string',
          )
        : []
      return { entries }
    } catch {
      return { entries: [] }
    }
  })()
  const foldFeature = createFoldFeature(
    () => view,
    persistedFolds,
    foldKey
      ? (keys) => {
          try {
            localStorage.setItem(foldKey, JSON.stringify(keys))
          } catch {
          }
        }
      : undefined,
  )

  const tableColumnWidths = readTableColumnWidths(options.docKey)
  const tableColumnWidthPlugins = createTableColumnWidthPlugins(options.docKey, tableColumnWidths)
  const plugins = view.state.plugins.filter((plugin) => !isIdentityBreaking(plugin))
  plugins.push(
    selectionPlugin,
    lockedViewPlugin,
    historyWatch,
    tableCaretPlugin,
    ...tableColumnWidthPlugins,
    createCellDragPlugin(),
    ...foldFeature.plugins,
  )
  plugins.unshift(...foldFeature.earlyPlugins)

  const buildWithTableWidths = (text: string): BuiltDocument => {
    const candidate = buildDocument(text, schema, parser)
    const beforeRestore = candidate.doc
    candidate.doc = restoreTableColumnWidths(candidate.doc, tableColumnWidths)
    if (candidate.doc !== beforeRestore) {
      candidate.doc.forEach((node, _offset, index) => {
        const previous = beforeRestore.child(index)
        if (node === previous) return
        const origin = candidate.origins.get(previous)
        if (origin) {
          const updated = { ...origin, node }
          candidate.origins.set(node, updated)
          candidate.loaded[origin.segment.index] = updated
        }
      })
    }
    return candidate
  }

  const load = (text: string) => {
    composerScope.cancel()
    suppressChange = true
    try {
      built = buildWithTableWidths(text)
      builtSnapshots.length = 0
      recordBuiltSnapshot(built.doc, built)
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
    emitHistory()
  }

  const applyPreviewMarkdownInPlace = (text: string) => {
    composerScope.cancel()
    const prevDoc = view.state.doc
    const prevBuilt = resolveBuiltForDoc(prevDoc)
    recordBuiltSnapshot(prevDoc, prevBuilt)

    const nextBuilt = buildWithTableWidths(text)
    const nextDoc = nextBuilt.doc

    const prevCount = prevDoc.childCount
    const nextCount = nextDoc.childCount
    const minCount = Math.min(prevCount, nextCount)

    const canShareBlock = (prevIdx: number, nextIdx: number): boolean => {
      if (prevIdx === 0 && nextIdx === 0 && prevBuilt.prefix !== nextBuilt.prefix) return false
      const pNode = prevDoc.child(prevIdx)
      const nNode = nextDoc.child(nextIdx)
      if (!pNode.eq(nNode)) return false
      const pOrigin = prevBuilt.origins.get(pNode)
      const nOrigin = nextBuilt.origins.get(nNode)
      if (!pOrigin || !nOrigin) return false
      return (
        pOrigin.source === nOrigin.source &&
        pOrigin.segment.gapBefore === nOrigin.segment.gapBefore
      )
    }

    let prefixCount = 0
    while (prefixCount < minCount && canShareBlock(prefixCount, prefixCount)) {
      prefixCount += 1
    }

    let suffixCount = 0
    if (prevBuilt.trailing === nextBuilt.trailing) {
      while (
        suffixCount < minCount - prefixCount &&
        canShareBlock(prevCount - 1 - suffixCount, nextCount - 1 - suffixCount)
      ) {
        suffixCount += 1
      }
    }

    const shared: Array<[number, number]> = []
    for (let i = 0; i < prefixCount; i += 1) shared.push([i, i])
    const candidates = new Map<string, { indexes: number[]; cursor: number }>()
    for (let i = prefixCount; i < nextCount - suffixCount; i += 1) {
      const origin = nextBuilt.origins.get(nextDoc.child(i))
      if (!origin) continue
      const match = candidates.get(origin.source)
      if (match) match.indexes.push(i)
      else candidates.set(origin.source, { indexes: [i], cursor: 0 })
    }
    let nextIndex = prefixCount
    for (let i = prefixCount; i < prevCount - suffixCount; i += 1) {
      const origin = prevBuilt.origins.get(prevDoc.child(i))
      const match = origin && candidates.get(origin.source)
      if (!match) continue
      while (match.cursor < match.indexes.length && match.indexes[match.cursor] < nextIndex) {
        match.cursor += 1
      }
      const index = match.indexes[match.cursor]
      if (index !== undefined && canShareBlock(i, index)) {
        shared.push([i, index])
        nextIndex = index + 1
      }
    }
    for (let i = suffixCount; i > 0; i -= 1) {
      shared.push([prevCount - i, nextCount - i])
    }
    if (shared.length === prevCount && shared.length === nextCount) shared.length = 0

    const positions: number[] = []
    prevDoc.forEach((_node, pos) => positions.push(pos))
    positions.push(prevDoc.content.size)
    const tr = closeHistory(view.state.tr)
    let oldEnd = prevCount
    let newEnd = nextCount
    for (let index = shared.length - 1; index >= -1; index -= 1) {
      const [oldIndex, newIndex] = shared[index] ?? [-1, -1]
      if (oldIndex + 1 < oldEnd || newIndex + 1 < newEnd) {
        const replacements: PMNode[] = []
        for (let i = newIndex + 1; i < newEnd; i += 1) replacements.push(nextDoc.child(i))
        tr.replaceWith(positions[oldIndex + 1], positions[oldEnd], replacements)
      }
      if (oldIndex < 0) break
      const pNode = prevDoc.child(oldIndex)
      const nOrigin = nextBuilt.origins.get(nextDoc.child(newIndex))!
      const bound = { ...nOrigin, node: pNode }
      nextBuilt.origins.set(pNode, bound)
      nextBuilt.loaded[nOrigin.segment.index] = bound
      oldEnd = oldIndex
      newEnd = newIndex
    }

    built = nextBuilt
    composedCache = undefined
    view.dispatch(tr)
    view.dispatch(closeHistory(view.state.tr))
    recordBuiltSnapshot(view.state.doc, nextBuilt)
  }

  editor.action((ctx) => {
    ctx.get(listenerCtx).markdownUpdated(() => {
      if (mode === 'preview') notify(compose(view.state.doc))
    })
  })

  load(options.initialText ?? '')
  if (options.onChange) listeners.add(options.onChange)

  const land = (selection: Selection, block: ComposedBlock | undefined, pos: number) => {
    lastReportedOffset = undefined
    view.focus()
    view.dispatch(view.state.tr.setSelection(selection))
    const target = view.nodeDOM(pos)
    if (target instanceof HTMLElement) target.scrollIntoView({ block: 'start' })
    else view.dispatch(view.state.tr.scrollIntoView())
    if (block && lastReportedOffset !== block.start) emitSelection(block.start)
  }

  const reveal = (offset: number): boolean => {
    if (mode === 'source') return revealSource(offset, offset, true)
    if (!Number.isFinite(offset)) return false
    const { text, blocks } = composed(view.state.doc)
    if (offset < 0 || offset > text.length) return false

    const block = blockAtOffset(blocks, offset)
    const pos = block ? topLevelPosition(view.state.doc, block.node) : 1
    if (pos === undefined) return false

    foldFeature.expandAt(pos)

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

    foldFeature.expandRange(start, lastStart + last.node.nodeSize)

    const selection =
      first === last && first.node.type.name === LOCKED_NODE
        ? NodeSelection.create(doc, start)
        : TextSelection.between(doc.resolve(start), doc.resolve(lastStart + last.node.nodeSize))
    land(selection, first, start)
    return true
  }

  const setMode = (next: EditorMode) => {
    if (next === mode) return
    composerScope.cancel()

    if (next === 'source') {
      const text = compose(view.state.doc)
      const offset = selectionOffset(view.state) ?? 0
      mode = 'source'
      sourceBaseText = text
      previewRoot.hidden = true
      sourceRoot.hidden = false
      ensureSourceEditor(text)
      revealSource(offset, offset, true)
    } else {
      const text = sourceEditor?.getText() ?? ''
      const offset = sourceEditor?.getSelection() ?? 0
      mode = 'preview'
      sourceRoot.hidden = true
      previewRoot.hidden = false
      if (sourceBaseText === undefined || text !== sourceBaseText) {
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
    emitHistory()
  }

  const setReadonly = (value: boolean) => {
    if (readonly === value) return
    readonly = value
    if (value) composerScope.cancel()
    builder.setReadonly(value)
    sourceEditor?.setReadonly(value)
    view.dispatch(view.state.tr.setMeta('mgo-readonly', value))
  }

  return {
    getMode: () => mode,
    setMode,
    toggleMode: () => setMode(mode === 'preview' ? 'source' : 'preview'),
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
    applyMarkdown(expected, text) {
      const current = mode === 'source' ? sourceEditor?.getText() : compose(view.state.doc)
      if (readonly || current === undefined || current !== expected) return false
      if (current === text) return true
      if (mode === 'source') {
        suppressChange = true
        try {
          if (current !== compose(view.state.doc)) applyPreviewMarkdownInPlace(current)
          applyPreviewMarkdownInPlace(text)
        } finally {
          suppressChange = false
        }
        sourceBaseText = text
        sourceEditor!.replace(text)
        return true
      }
      applyPreviewMarkdownInPlace(text)
      if (sourceEditor) {
        sourceBaseText = text
        sourceEditor.setText(text)
      }
      return true
    },
    setMarkdown(text) {
      if (mode === 'source') {
        sourceBaseText = undefined
        const source = ensureSourceEditor(text)
        lastReportedOffset = undefined
        emitSelection(source.getSelection())
        emitHistory()
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
    toggleFold: (pos: number) => foldFeature.toggle(pos),
    foldAnchors: () => [...(foldPluginKey.getState(view.state)?.anchors ?? [])],
    select,
    addNote,
    onSelectionChange(fn) {
      selectionListeners.add(fn)
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
    canUndo() {
      if (mode === 'source') {
        if (sourceEditor?.canUndo()) return true
        return sourceEditor !== undefined && sourceEditor.getText() === sourceBaseText && undoDepth(view.state) > 0
      }
      return undoDepth(view.state) > 0
    },
    canRedo() {
      if (mode === 'source') {
        if (sourceEditor?.canRedo()) return true
        return sourceEditor !== undefined && sourceEditor.getText() === sourceBaseText && redoDepth(view.state) > 0
      }
      return redoDepth(view.state) > 0
    },
    undo() {
      if (mode === 'source') {
        if (sourceEditor?.canUndo()) {
          const syncedWithPreview = sourceEditor.getText() === sourceBaseText
          const ok = sourceEditor.undo()
          if (ok && syncedWithPreview && undoDepth(view.state) > 0) {
            suppressChange = true
            try {
              undoRich(view.state, view.dispatch)
            } finally {
              suppressChange = false
            }
            if (compose(view.state.doc) === sourceEditor.getText()) {
              sourceBaseText = sourceEditor.getText()
            }
          }
          return ok
        }
        if (sourceEditor && sourceEditor.getText() === sourceBaseText && undoDepth(view.state) > 0) {
          suppressChange = true
          let ok = false
          try {
            ok = undoRich(view.state, view.dispatch)
          } finally {
            suppressChange = false
          }
          if (ok) {
            const nextText = compose(view.state.doc)
            sourceBaseText = nextText
            sourceEditor.setText(nextText)
            notify(nextText)
            emitHistory()
          }
          return ok
        }
        return false
      }
      return undoRich(view.state, view.dispatch)
    },
    redo() {
      if (mode === 'source') {
        if (sourceEditor?.canRedo()) {
          const syncedWithPreview = sourceEditor.getText() === sourceBaseText
          const ok = sourceEditor.redo()
          if (ok && syncedWithPreview && redoDepth(view.state) > 0) {
            suppressChange = true
            try {
              redoRich(view.state, view.dispatch)
            } finally {
              suppressChange = false
            }
            if (compose(view.state.doc) === sourceEditor.getText()) {
              sourceBaseText = sourceEditor.getText()
            }
          }
          return ok
        }
        if (sourceEditor && sourceEditor.getText() === sourceBaseText && redoDepth(view.state) > 0) {
          suppressChange = true
          let ok = false
          try {
            ok = redoRich(view.state, view.dispatch)
          } finally {
            suppressChange = false
          }
          if (ok) {
            const nextText = compose(view.state.doc)
            sourceBaseText = nextText
            sourceEditor.setText(nextText)
            notify(nextText)
            emitHistory()
          }
          return ok
        }
        return false
      }
      return redoRich(view.state, view.dispatch)
    },
    onHistoryChange(fn) {
      historyListeners.add(fn)
      return {
        dispose() {
          historyListeners.delete(fn)
        },
      }
    },
    getView: () => view,
    async destroy() {
      composerScope.cancel()
      listeners.clear()
      selectionListeners.clear()
      modeListeners.clear()
      historyListeners.clear()
      sourceEditor?.destroy()
      sourceEditor = undefined
      await builder.destroy()
    },
  }
}

function sameTopLevelChildren(a: PMNode, b: PMNode): boolean {
  if (a === b) return true
  if (a.childCount !== b.childCount) return false
  for (let i = 0; i < a.childCount; i += 1) {
    if (a.child(i) !== b.child(i)) return false
  }
  return true
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
  return IDENTITY_BREAKING_PLUGIN_KEYS[key.replace(/\$\d*$/, '')] === true
}

function pluginKey(plugin: Plugin): string | undefined {
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
