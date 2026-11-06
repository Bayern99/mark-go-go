import type { Disposable } from '../host/types'
import type { DocumentSnapshot } from './types'

/** The single in-memory copy of the document text that plugins can read. */
export class DocumentService {
  private state: DocumentSnapshot
  private readonly listeners = new Set<(state: DocumentSnapshot) => void>()

  constructor(initial: Partial<DocumentSnapshot> = {}) {
    this.state = { text: '', version: 0, path: '', ...initial }
  }

  snapshot(): DocumentSnapshot {
    return { ...this.state }
  }

  /** A reload from the host: text and version both change. */
  setSnapshot(text: string, version: number): void {
    if (this.state.text === text && this.state.version === version) return
    this.state = { ...this.state, text, version }
    this.emit()
  }

  /** A local edit: the text changes, the base version does not. */
  setText(text: string): void {
    if (this.state.text === text) return
    this.state = { ...this.state, text }
    this.emit()
  }

  /** The document was saved somewhere new, or first saved; `''` while untitled. */
  setPath(path: string): void {
    if (this.state.path === path) return
    this.state = { ...this.state, path }
    this.emit()
  }

  onDidChange(fn: (state: DocumentSnapshot) => void): Disposable {
    this.listeners.add(fn)
    return {
      dispose: () => {
        this.listeners.delete(fn)
      },
    }
  }

  private emit(): void {
    const current = this.snapshot()
    for (const fn of this.listeners) fn(current)
  }
}
