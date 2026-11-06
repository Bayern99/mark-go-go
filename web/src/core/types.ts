import type { Disposable, ViewSnapshot } from '../host/types'

export interface DocumentSnapshot {
  text: string
  version: number
}

/**
 * Where a view shows: `left` is the sidebar; `center` makes the view a view
 * mode the user picks instead of the editor, mounted only while it is shown.
 */
export type ViewLocation = 'left' | 'center'

export interface ViewRegistration {
  id: string
  title: string
  location: ViewLocation
  mount(el: HTMLElement): Disposable
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
  /** The document's view file; following it starts the host's watch. */
  viewFile: {
    watch(fn: (snap: ViewSnapshot) => void): Disposable
  }
  subscriptions: Disposable[]
}

export interface AppPlugin {
  id: string
  title: string
  activate(ctx: PluginContext): void | Promise<void>
}

export type CommandHandler = (...args: unknown[]) => unknown
