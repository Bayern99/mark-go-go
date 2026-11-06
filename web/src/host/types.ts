/**
 * Contract between the web frontend and whatever owns the document
 * (MyGo on the desktop host, an in-memory shim in a plain browser).
 */
export interface DocState {
  name: string
  path: string
  edited: boolean
  conflict: boolean
  deleted: boolean
  readOnly: boolean
}

/** `version` increments every time content is (re)loaded from disk. */
export interface HostDocSnapshot {
  text: string
  version: number
  state: DocState
}

export interface Disposable {
  dispose(): void
}

export interface HostAdapter {
  /** Content of this window's document. */
  load(): Promise<HostDocSnapshot>
  /** Full Markdown text after an edit. */
  update(text: string): Promise<void>
  /** An external change was reloaded by MyGo. */
  onReload(fn: (snap: HostDocSnapshot) => void): Disposable
  /** edited / conflict / name changes. */
  onState(fn: (state: DocState) => void): Disposable
}
