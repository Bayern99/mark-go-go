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

/**
 * The document's view file (`.<name>.md.genui.json` next to it), as the host
 * read it. `exists` is false when there is none, including for an untitled
 * document; `error` says why an existing file could not be read.
 */
export interface ViewSnapshot {
  exists: boolean
  text: string
  error: string
}

/** What plugins may do with the view file: follow it. */
export interface ViewFileSource {
  /**
   * Follow the view file on disk: `fn` gets the file as it is once the watch
   * runs, then every change, including the document moving to another path.
   * Disposing stops the host's watch once no one else follows.
   */
  watchView(fn: (snap: ViewSnapshot) => void): Disposable
}

export interface HostAdapter extends ViewFileSource {
  /** Content of this window's document. */
  load(): Promise<HostDocSnapshot>
  /** Full Markdown text after an edit. */
  update(text: string): Promise<void>
  /** An external change was reloaded by MyGo. */
  onReload(fn: (snap: HostDocSnapshot) => void): Disposable
  /** edited / conflict / name changes. */
  onState(fn: (state: DocState) => void): Disposable
}
