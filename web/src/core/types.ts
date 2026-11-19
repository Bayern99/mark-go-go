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
  path: string
}

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
  workspaceScoped?: boolean
  status?: ViewStatusSource
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
  viewFile: {
    watch(fn: (snap: ViewSnapshot) => void): Disposable
    generate(
      req: GenerateRequest,
      onProgress: (progress: GenerateProgress) => void,
      signal: AbortSignal,
    ): Promise<void>
    undo(): Promise<void>
    getSettings(): Promise<GenerationSettings>
    setSettings(patch: SettingsPatch): Promise<GenerationSettings>
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
