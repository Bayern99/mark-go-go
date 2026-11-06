import { extractSections, findSection, type MarkdownSection } from '../../markdown/sections'
import type { SourceRef } from './viewFile'

/** Source state is re-checked this long after the last document change, like the outline. */
export const SOURCE_DEBOUNCE_MS = 300

/**
 * What a component's sources say about it:
 * - `fresh`: every source section reads as when the view was made;
 * - `changed`: a source section was edited since;
 * - `missing`: a source heading is gone (renamed, deleted, or never there);
 * - `unknown`: no recorded hash to compare against (a hand-written view), or
 *   the component names no sources.
 */
export type SourceStatus = 'fresh' | 'changed' | 'missing' | 'unknown'

/** Where a component's first source currently is in the document. */
export interface SourceTarget {
  start: number
  end: number
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

  /** Headings of a component's sources, for its label. */
  headings(ids: readonly string[] | undefined): string[] {
    return (ids ?? []).flatMap((id) => {
      const ref = this.table[id]
      return ref ? [ref.heading] : []
    })
  }

  /** The current range of the first source that is in the document. */
  target(ids: readonly string[] | undefined): SourceTarget | undefined {
    if (this.timer !== undefined) {
      window.clearTimeout(this.timer)
      this.timer = undefined
      this.refresh()
    }
    for (const id of ids ?? []) {
      const ref = this.table[id]
      const section = ref && findSection(this.sections, ref.heading, ref.occurrence)
      if (section) return { start: section.start, end: section.end }
    }
    return undefined
  }

  private refresh(): void {
    this.sections = extractSections(this.text)
    this.revision += 1
    for (const listener of this.listeners) listener()
  }
}
