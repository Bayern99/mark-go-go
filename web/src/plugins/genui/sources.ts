import { extractSections, findSection, type MarkdownSection } from '../../markdown/sections'
import type { SourceRef } from './viewFile'

export const SOURCE_DEBOUNCE_MS = 300

export type SourceStatus = 'fresh' | 'changed' | 'missing' | 'unknown'

export interface SourceTarget {
  start: number
  end: number
}

export interface SourceReference {
  id: string
  heading: string
  occurrence: number
  present: boolean
}

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

  hasSource(ids: readonly string[] | undefined): boolean {
    for (const id of ids ?? []) {
      const ref = this.table[id]
      if (ref && findSection(this.sections, ref.heading, ref.occurrence)) return true
    }
    return false
  }

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

  headings(ids: readonly string[] | undefined): string[] {
    return (ids ?? []).flatMap((id) => {
      const ref = this.table[id]
      return ref ? [ref.heading] : []
    })
  }

  notes(ids: readonly string[] | undefined): string[] {
    const notes = new Map<number, string>()
    for (const id of ids ?? []) {
      const ref = this.table[id]
      const section = ref && findSection(this.sections, ref.heading, ref.occurrence)
      for (const note of section?.notes ?? []) notes.set(note.start, note.text)
    }
    return [...notes.keys()].sort((a, b) => a - b).map((start) => notes.get(start) ?? '')
  }

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
