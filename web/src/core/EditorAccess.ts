import type { Disposable } from '../host/types'

export interface EditorSelectionSource {
  onSelectionChange(fn: (offset: number) => void): Disposable
}

export class EditorAccess implements EditorSelectionSource {
  private source: EditorSelectionSource | null = null

  attach(source: EditorSelectionSource): void {
    this.source = source
  }

  detach(): void {
    this.source = null
  }

  onSelectionChange(fn: (offset: number) => void): Disposable {
    if (!this.source) throw new Error('Editor is not attached yet')
    return this.source.onSelectionChange(fn)
  }
}
