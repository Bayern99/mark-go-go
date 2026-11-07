import { createContext, useContext } from 'react'

/**
 * Local reading interactions for one document’s GenUI view: KeyPoints fold,
 * ComparisonTable sort and horizontal scroll, and Tabs selection. Kept outside
 * the mounted surface so leaving GenUI and coming back, or replacing one card,
 * does not reset the rest. It never writes the document.
 */
export type TableSort = { column: number; direction: 'ascending' | 'descending' } | null

export class ReadingStore {
  private readonly keypoints = new Map<string, boolean>()
  private readonly sorts = new Map<string, TableSort>()
  private readonly tableScroll = new Map<string, number>()
  private readonly tabs = new Map<string, number>()

  keypointsExpanded(id: string): boolean {
    return this.keypoints.get(id) === true
  }

  setKeypointsExpanded(id: string, expanded: boolean): void {
    if (expanded) this.keypoints.set(id, true)
    else this.keypoints.delete(id)
  }

  tableSort(id: string): TableSort {
    return this.sorts.get(id) ?? null
  }

  setTableSort(id: string, sort: TableSort): void {
    if (sort) this.sorts.set(id, sort)
    else this.sorts.delete(id)
  }

  tableScrollLeft(id: string): number {
    return this.tableScroll.get(id) ?? 0
  }

  setTableScrollLeft(id: string, left: number): void {
    if (left > 0) this.tableScroll.set(id, left)
    else this.tableScroll.delete(id)
  }

  tab(key: string): number | undefined {
    return this.tabs.get(key)
  }

  setTab(key: string, index: number): void {
    if (index > 0) this.tabs.set(key, index)
    else this.tabs.delete(key)
  }
}

export const ReadingContext = createContext<ReadingStore | null>(null)

