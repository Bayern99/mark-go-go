import { extractSections, findSection, type MarkdownSection } from '../../markdown/sections'
import type { SourceRef } from './viewFile'

/** Source state is re-checked this long after the last document change, like the outline. */
export const SOURCE_DEBOUNCE_MS = 300

/**
 * What a component's sources say about it:
 * - `fresh`: every source section reads as when the view was made;
 * - `changed`: a source section was edited since;
 * - `missing`: a source heading is gone (renamed, deleted, or never there);
 *   this is the aggregate, even when another listed source is still there;
 * - `unknown`: no recorded hash to compare against (a hand-written view), or
 *   the component names no sources.
 */
export type SourceStatus = 'fresh' | 'changed' | 'missing' | 'unknown'

/** Where a component's source currently is in the document. */
export interface SourceTarget {
  start: number
  end: number
}

/** One recorded source of a component, resolved by its own id. */
export interface SourceReference {
  id: string
  /** The heading as the source table records it. */
  heading: string
  /** Which heading with that text, so repeated headings stay distinct. */
  occurrence: number
  /** Whether that exact heading occurrence is still in the document. */
  present: boolean
}

/**
 * The view file's source table, checked against the document. Sections are
 * re-read from the latest text after the debounce, or at once when asked
 * for a target, so navigation never uses a stale offset.
 */
export class SourceStore {
  private sections: MarkdownSection[]
  private text: string
  private timer: number | undefined
  private revision = 0
  private readonly listeners = new Set<() => void>()

  constructor(
    private readonly table: Readonly<Record<string, SourceRef>>,
    text: string,
  ) {
    this.text = text
    this.sections = extractSections(text)
  }

  readonly subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  /** Changes whenever statuses may have changed; a snapshot for React. */
  readonly getRevision = (): number => this.revision

  schedule(text: string): void {
    this.text = text
    if (this.timer !== undefined) window.clearTimeout(this.timer)
    this.timer = window.setTimeout(() => {
      this.timer = undefined
      this.refresh()
    }, SOURCE_DEBOUNCE_MS)
  }

  dispose(): void {
    if (this.timer !== undefined) window.clearTimeout(this.timer)
    this.timer = undefined
    this.listeners.clear()
  }

  status(ids: readonly string[] | undefined): SourceStatus {
    if (!ids || ids.length === 0) return 'unknown'
    let status: SourceStatus = 'fresh'
    for (const id of ids) {
      const ref = this.table[id]
      const section = ref && findSection(this.sections, ref.heading, ref.occurrence)
      if (!section) return 'missing'
      if (ref.hash === undefined) status = 'unknown'
      else if (status === 'fresh' && section.hash !== ref.hash) status = 'changed'
    }
    return status
  }

  /**
   * Whether any of a component's sources is still in the document.
   * Independent of `status`: a missing sibling does not hide the rest.
   */
  hasSource(ids: readonly string[] | undefined): boolean {
    for (const id of ids ?? []) {
      const ref = this.table[id]
      if (ref && findSection(this.sections, ref.heading, ref.occurrence)) return true
    }
    return false
  }

  /**
   * A component's sources one by one, each resolved by its own id: a heading
   * occurrence that is gone stays its own disabled reference instead of
   * falling back to a sibling.
   */
  references(ids: readonly string[] | undefined): SourceReference[] {
    return (ids ?? []).flatMap((id) => {
      const ref = this.table[id]
      if (!ref) return []
      return [
        {
          id,
          heading: ref.heading,
          occurrence: ref.occurrence,
          present: findSection(this.sections, ref.heading, ref.occurrence) !== undefined,
        },
      ]
    })
  }

  /** Headings of a component's sources, for its label. */
  headings(ids: readonly string[] | undefined): string[] {
    return (ids ?? []).flatMap((id) => {
      const ref = this.table[id]
      return ref ? [ref.heading] : []
    })
  }

  /** The notes now in a component's source sections, in document order. */
  notes(ids: readonly string[] | undefined): string[] {
    const notes = new Map<number, string>()
    for (const id of ids ?? []) {
      const ref = this.table[id]
      const section = ref && findSection(this.sections, ref.heading, ref.occurrence)
      // Sources can overlap: a section holds its sub-sections' notes too.
      for (const note of section?.notes ?? []) notes.set(note.start, note.text)
    }
    return [...notes.keys()].sort((a, b) => a - b).map((start) => notes.get(start) ?? '')
  }

  /**
   * The current range of `text` in the first remaining source that holds it
   * exactly, or else of the first source that is still in the document.
   * A missing occurrence is not replaced by another heading of the same text.
   */
  target(ids: readonly string[] | undefined, text?: string): SourceTarget | undefined {
    if (this.timer !== undefined) {
      window.clearTimeout(this.timer)
      this.timer = undefined
      this.refresh()
    }
    const found = (ids ?? []).flatMap((id) => {
      const ref = this.table[id]
      const section = ref && findSection(this.sections, ref.heading, ref.occurrence)
      return section ? [section] : []
    })
    const needle = text?.trim()
    if (needle) {
      for (const section of found) {
        const at = this.text.slice(section.start, section.end).indexOf(needle)
        if (at >= 0) return { start: section.start + at, end: section.start + at + needle.length }
      }
    }
    const first = found[0]
    return first && { start: first.start, end: first.end }
  }

  private refresh(): void {
    this.sections = extractSections(this.text)
    this.revision += 1
    for (const listener of this.listeners) listener()
  }
}
