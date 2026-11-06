import type { Disposable } from '../host/types'

export interface DocumentSnapshot {
  text: string
  version: number
}

export interface ViewRegistration {
  id: string
  title: string
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
  subscriptions: Disposable[]
}

export interface AppPlugin {
  id: string
  title: string
  activate(ctx: PluginContext): void | Promise<void>
}

export type CommandHandler = (...args: unknown[]) => unknown
