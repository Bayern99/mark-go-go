import type {
  Disposable,
  GenerateProgress,
  GenerateRequest,
  GenerationSettings,
  SettingsPatch,
  ViewSnapshot,
} from '../host/types'

export interface DocumentSnapshot {
  text: string
  version: number
  /** Where the document is saved; `''` while it is untitled. */
  path: string
}

/**
 * Where a view shows: `left` is the workspace sidebar; `center` makes the view
 * a view mode the user picks instead of the editor, mounted only while it is
 * shown; `right` is the document panel on the right side of the window.
 */
export type ViewLocation = 'left' | 'center' | 'right'

export type ViewStatusDot = {
  tone: 'warn' | 'danger'
  title?: string
} | null

export interface ViewStatusSource {
  subscribe(fn: () => void): () => void
  getSnapshot(): ViewStatusDot
}

export interface ViewRegistration {
  id: string
  title: string
  location: ViewLocation
  status?: ViewStatusSource
  /** `scrollElement` is the viewport when the host uses an overlay scrollbar. */
  mount(el: HTMLElement, scrollElement?: HTMLElement): Disposable
}

export interface PluginContext {
  documents: {
    snapshot(): DocumentSnapshot
    onDidChange(fn: (s: DocumentSnapshot) => void): Disposable
  }
  views: {
    register(v: ViewRegistration): Disposable
  }
  commands: {
    register(id: string, fn: (...a: unknown[]) => unknown): Disposable
    execute(id: string, ...a: unknown[]): Promise<unknown>
  }
  editor: {
    onSelectionChange(fn: (offset: number) => void): Disposable
  }
  /** The document's view file: following it starts the host's watch; generating writes it. */
  viewFile: {
    watch(fn: (snap: ViewSnapshot) => void): Disposable
    generate(
      req: GenerateRequest,
      onProgress: (progress: GenerateProgress) => void,
      signal: AbortSignal,
    ): Promise<void>
    /** Puts back the view the last generation replaced, once. */
    undo(): Promise<void>
    getSettings(): Promise<GenerationSettings>
    /** Writes only the patch's fields; resolves with the committed settings. */
    setSettings(patch: SettingsPatch): Promise<GenerationSettings>
    /** The one settings store changed, from this document or any other window. */
    onSettings(fn: (settings: GenerationSettings) => void): Disposable
  }
  subscriptions: Disposable[]
}

export interface AppPlugin {
  id: string
  title: string
  activate(ctx: PluginContext): void | Promise<void>
}

export type CommandHandler = (...args: unknown[]) => unknown
