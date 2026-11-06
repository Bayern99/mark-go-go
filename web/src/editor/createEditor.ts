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
import { buildDocument } from './blocks/build'
import {
  blockAtOffset,
  composeDocument,
  composeWithRanges,
  type ComposeContext,
} from './blocks/compose'
import { LOCKED_NODE, lockedBlockSchema } from './blocks/locked'
import { languages } from './languages'

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

export interface MarkdownEditor {
  /** Composed Markdown for the whole document. */
  getMarkdown(): string
  /** Replace the editor content (external reload). Undo history is reset. */
  setMarkdown(text: string): void
  /** Fires after every user edit with freshly composed Markdown. */
  onChange(fn: (markdown: string) => void): Disposable
  /**
   * Scroll a Markdown source offset into view and place the cursor at the
   * start of the top-level block containing it. Returns false when the offset
   * falls outside the current composed text.
   */
  reveal(offset: number): boolean
  /**
   * Fires with the source start offset of the cursor's top-level block whenever
   * the selection changes block or an edit moves that offset.
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
  const builder = new CrepeBuilder({ root, defaultValue: '' })
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
  let readonly = false
  const listeners = new Set<(markdown: string) => void>()
  const selectionListeners = new Set<(offset: number) => void>()
  let lastReportedOffset: number | undefined

  const composeContext = (current: typeof built): ComposeContext => ({
    schema,
    serializer,
    origins: current.origins,
    segments: current.segments,
    prefix: current.prefix,
    trailing: current.trailing,
  })

  const compose = (doc: PMNode): string => composeDocument(doc, composeContext(built))

  /** Source start offset of the top-level block holding the current selection. */
  const selectionOffset = (state: EditorState): number | undefined => {
    const { blocks } = composeWithRanges(state.doc, composeContext(built))
    if (blocks.length === 0) return undefined

    const { $from } = state.selection
    if ($from.depth >= 1) {
      const top = $from.node(1)
      const match = blocks.find((block) => block.node === top)
      if (match) return match.start
    }

    const selection = state.selection
    if ('node' in selection) {
      const selected = selection.node
      const match = blocks.find((block) => block.node === selected)
      if (match) return match.start
    }

    return blockAtOffset(blocks, state.selection.from)?.start
  }

  const reportSelection = (state: EditorState) => {
    if (selectionListeners.size === 0) return
    const offset = selectionOffset(state)
    if (offset === undefined || offset === lastReportedOffset) return
    lastReportedOffset = offset
    for (const fn of selectionListeners) fn(offset)
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
    if (!Number.isFinite(offset)) return false
    const { text, blocks } = composeWithRanges(view.state.doc, composeContext(built))
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

    view.dispatch(view.state.tr.setSelection(selection).scrollIntoView())
    view.focus()
    return true
  }

  const setReadonly = (value: boolean) => {
    if (readonly === value) return
    readonly = value
    builder.setReadonly(value)
  }

  return {
    getMarkdown: () => compose(view.state.doc),
    setMarkdown(text) {
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
      const offset = selectionOffset(view.state)
      if (offset !== undefined) {
        lastReportedOffset = offset
        fn(offset)
      }
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
