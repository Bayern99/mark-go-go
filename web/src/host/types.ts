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

/** One heading section of the text sent to the agent, in UTF-16 offsets of that text. */
export interface SectionRef {
  heading: string
  occurrence: number
  start: number
  end: number
}

export interface GenerateRequest {
  /** The editor text at the moment Generate was clicked; the agent reads only this. */
  text: string
  mode: 'full'
  /** What the reader asked for; kept in the view file and offered again next time. */
  request?: string
  /** The format document and the catalog definition, for the agent's prompt. */
  format: string
  /** The text's heading sections, which the host hashes into the view's sources. */
  sections: SectionRef[]
}

export type GeneratePhase = 'starting' | 'thinking' | 'reading' | 'writing' | 'checking'

export interface GenerateProgress {
  phase: GeneratePhase
  text: string
}

export type GenerateFailureKind =
  | 'unsaved'
  | 'busy'
  | 'agent-not-found'
  | 'auth-required'
  | 'model-invalid'
  | 'agent-message'
  | 'invalid-output'
  | 'agent-error'
  | 'timeout'
  | 'cancelled'

/** Why a generation ended without a new view, and what the reader can do. */
export interface GenerateFailure {
  kind: GenerateFailureKind
  /** What happened, in a sentence or two. */
  message: string
  /** What the reader can do about it. */
  hint?: string
  /** The agent's own words, verbatim. */
  detail?: string
  /** The models the agent offers, for `model-invalid`. */
  models?: string[]
}

export interface GenerationSettings {
  /** The model the agent is asked to use; empty leaves the agent's own choice. */
  model: string
  /** The agent's command, run as `<agentCommand> acp`. */
  agentCommand: string
}

/**
 * Making a view with the reader's agent. The host writes the view file when
 * the run succeeds, so the new view arrives through `watchView`; a failed
 * or cancelled run leaves the file as it was.
 */
export interface ViewGenerator {
  /**
   * Runs one generation, reporting progress, until it is done. Rejects with
   * a `GenerationError` whose failure tells the reader what happened.
   * Aborting `signal` cancels the run, which rejects as `cancelled`.
   */
  generate(
    req: GenerateRequest,
    onProgress: (progress: GenerateProgress) => void,
    signal: AbortSignal,
  ): Promise<void>
  getSettings(): Promise<GenerationSettings>
  setSettings(settings: GenerationSettings): Promise<void>
}

export interface HostAdapter extends ViewFileSource, ViewGenerator {
  /** Content of this window's document. */
  load(): Promise<HostDocSnapshot>
  /** Full Markdown text after an edit. */
  update(text: string): Promise<void>
  /** An external change was reloaded by MyGo. */
  onReload(fn: (snap: HostDocSnapshot) => void): Disposable
  /** edited / conflict / name changes. */
  onState(fn: (state: DocState) => void): Disposable
}
