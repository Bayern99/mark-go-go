import { $nodeSchema } from '@milkdown/kit/utils'
import { parseNote } from '../../markdown/notes'
import type { LockKind } from './segment'

export const LOCKED_NODE = 'locked_block'

export interface LockedAttrs {
  source: string
  kind: LockKind
}

const KIND_LABEL: Record<LockKind, string> = {
  'yaml-frontmatter': 'frontmatter',
  'github-alert': 'alert',
  'footnote-definition': 'footnote definition',
  'link-reference-definition': 'link reference definition',
  'code-fence-meta': 'code fence with meta',
  'raw-html': 'raw HTML',
  note: 'note',
  math: 'math block',
  unsupported: 'unsupported block',
}

/**
 * An immutable top-level block that keeps its original Markdown source.
 * ProseMirror treats it as a single atom: it cannot be edited in place, and
 * composing writes `source` back byte for byte. A note is such a block too,
 * shown as its text with the actions the editor handles by `data-note-action`.
 */
export const lockedBlockSchema = $nodeSchema(LOCKED_NODE, () => ({
  group: 'block',
  atom: true,
  selectable: true,
  draggable: false,
  isolating: true,
  attrs: {
    source: { default: '' },
    kind: { default: 'unsupported' },
  },
  parseDOM: [
    {
      tag: 'aside[data-note]',
      getAttrs: (dom: HTMLElement) => ({ source: dom.dataset.source ?? '', kind: 'note' }),
    },
    {
      tag: 'pre[data-locked-block]',
      getAttrs: (dom: HTMLElement) => ({
        source: dom.querySelector('code')?.textContent ?? dom.textContent ?? '',
        kind: dom.dataset.kind ?? 'unsupported',
      }),
    },
  ],
  toDOM: (node) => {
    const attrs = node.attrs as LockedAttrs
    if (attrs.kind === 'note') {
      return [
        'aside',
        {
          class: 'mgo-note',
          'data-note': 'true',
          'data-locked-block': 'true',
          'data-kind': 'note',
          'data-source': attrs.source,
          'aria-label': 'Note',
          contenteditable: 'false',
        },
        ['span', { class: 'mgo-note-label' }, 'Note'],
        ['span', { class: 'mgo-note-text' }, parseNote(attrs.source)?.text ?? ''],
        [
          'span',
          { class: 'mgo-note-actions' },
          ['button', { type: 'button', class: 'pill xs', 'data-note-action': 'edit' }, 'Edit'],
          ['button', { type: 'button', class: 'pill xs', 'data-note-action': 'delete' }, 'Delete'],
        ],
      ]
    }
    const kind = (attrs.kind in KIND_LABEL ? attrs.kind : 'unsupported') as LockKind
    return [
      'pre',
      {
        class: 'mgo-locked-block',
        'data-locked-block': 'true',
        'data-kind': kind,
        'data-label': KIND_LABEL[kind],
        'aria-label': `Locked ${KIND_LABEL[kind]} (read only)`,
        contenteditable: 'false',
        spellcheck: 'false',
      },
      ['code', { class: 'mgo-locked-source' }, attrs.source],
    ]
  },
  parseMarkdown: {
    match: () => false,
    runner: () => {
      throw new Error('locked blocks are created from source segments, never parsed from Markdown')
    },
  },
  toMarkdown: {
    match: (node) => node.type.name === LOCKED_NODE,
    runner: (state, node) => {
      state.addNode('html', undefined, (node.attrs as LockedAttrs).source)
    },
  },
}))

export function lockedKindLabel(kind: LockKind): string {
  return KIND_LABEL[kind]
}
