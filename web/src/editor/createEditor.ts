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
  Selection,
  TextSelection,
  type Plugin,
} from '@milkdown/kit/prose/state'
import type { EditorView } from '@milkdown/kit/prose/view'
import { listenerCtx } from '@milkdown/kit/plugin/listener'
import type { Disposable } from '../host/types'
import { buildDocument } from './blocks/build'
import { composeDocument } from './blocks/compose'
import { lockedBlockSchema } from './blocks/locked'
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
  /** Scroll to and select the first heading with matching text. */
  revealHeading(heading: string): boolean
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

  const plugins = view.state.plugins.filter((plugin) => !isIdentityBreaking(plugin))

  let built = buildDocument(options.initialText ?? '', schema, parser)
  let suppressChange = false
  let readonly = false
  const listeners = new Set<(markdown: string) => void>()

  const compose = (doc: PMNode, current: typeof built): string =>
    composeDocument(doc, {
      schema,
      serializer,
      origins: current.origins,
      segments: current.segments,
      prefix: current.prefix,
      trailing: current.trailing,
    })

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
      notify(compose(view.state.doc, built))
    })
  })

  load(options.initialText ?? '')
  if (options.onChange) listeners.add(options.onChange)

  const revealHeading = (heading: string): boolean => {
    const found = findHeading(view.state.doc, heading)
    if (!found) return false
    const from = found.pos + 1
    const tr = view.state.tr
      .setSelection(TextSelection.create(view.state.doc, from, from + found.size))
      .scrollIntoView()
    view.dispatch(tr)
    view.focus()
    return true
  }

  const setReadonly = (value: boolean) => {
    if (readonly === value) return
    readonly = value
    builder.setReadonly(value)
  }

  return {
    getMarkdown: () => compose(view.state.doc, built),
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
    revealHeading,
    setReadonly,
    getView: () => view,
    async destroy() {
      listeners.clear()
      await builder.destroy()
    },
  }
}

const HEADING_ESCAPES = /[#*_`[\]]/g

function normalizeHeading(value: string): string {
  return value.replace(HEADING_ESCAPES, '').replace(/\s+/g, ' ').trim().toLowerCase()
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

function findHeading(doc: PMNode, heading: string): { pos: number; size: number } | null {
  const wanted = normalizeHeading(heading)
  if (wanted === '') return null
  let result: { pos: number; size: number } | null = null
  doc.descendants((node, pos) => {
    if (result) return false
    if (node.type.name !== 'heading') return true
    if (normalizeHeading(node.textContent) !== wanted) return true
    result = { pos, size: node.content.size }
    return false
  })
  return result
}
