import type { Disposable } from '../host/types'
import type { CommandRegistry } from './CommandRegistry'
import type { DocumentService } from './DocumentService'
import type { EditorSelectionSource } from './EditorAccess'
import type { ViewRegistry } from './ViewRegistry'
import type { AppPlugin, PluginContext } from './types'

interface PluginRecord {
  plugin: AppPlugin
  disposables: Set<Disposable> | null
  subscriptions: Disposable[] | null
}

export interface PluginInfo {
  id: string
  title: string
  enabled: boolean
}

/**
 * Owns plugin activation. Every registration a plugin makes goes through a
 * tracked context, so disabling a plugin disposes everything it created even
 * if the plugin itself forgot to.
 */
export class PluginRegistry {
  private readonly plugins = new Map<string, PluginRecord>()
  private readonly listeners = new Set<() => void>()
  private readonly documents: DocumentService
  private readonly views: ViewRegistry
  private readonly commands: CommandRegistry
  private readonly editor: EditorSelectionSource

  constructor(
    documents: DocumentService,
    views: ViewRegistry,
    commands: CommandRegistry,
    editor: EditorSelectionSource,
  ) {
    this.documents = documents
    this.views = views
    this.commands = commands
    this.editor = editor
  }

  register(plugin: AppPlugin): void {
    if (this.plugins.has(plugin.id)) throw new Error(`Plugin already registered: ${plugin.id}`)
    this.plugins.set(plugin.id, { plugin, disposables: null, subscriptions: null })
    this.emit()
  }

  list(): PluginInfo[] {
    return [...this.plugins.values()].map(({ plugin, disposables }) => ({
      id: plugin.id,
      title: plugin.title,
      enabled: disposables !== null,
    }))
  }

  isEnabled(id: string): boolean {
    return this.plugins.get(id)?.disposables !== null
  }

  async enable(id: string): Promise<void> {
    const record = this.plugins.get(id)
    if (!record) throw new Error(`Unknown plugin: ${id}`)
    if (record.disposables) return

    const disposables = new Set<Disposable>()
    const subscriptions: Disposable[] = []
    const track = (disposable: Disposable): Disposable => {
      const wrapped: Disposable = {
        dispose: () => {
          if (!disposables.delete(wrapped)) return
          disposable.dispose()
        },
      }
      disposables.add(wrapped)
      return wrapped
    }

    const ctx: PluginContext = {
      documents: {
        snapshot: () => this.documents.snapshot(),
        onDidChange: (fn) => track(this.documents.onDidChange(fn)),
      },
      views: {
        register: (view) => track(this.views.register(view)),
      },
      commands: {
        register: (commandId, fn) => track(this.commands.register(commandId, fn)),
        execute: (commandId, ...args) => this.commands.execute(commandId, ...args),
      },
      editor: {
        onSelectionChange: (fn) => track(this.editor.onSelectionChange(fn)),
      },
      subscriptions,
    }

    record.disposables = disposables
    record.subscriptions = subscriptions
    try {
      await record.plugin.activate(ctx)
    } catch (error) {
      this.disposeAll(disposables, subscriptions)
      record.disposables = null
      record.subscriptions = null
      this.emit()
      throw error
    }
    if (record.disposables !== disposables) {
      // Disabled while activate() was still pending (e.g. the app unmounted).
      // Registrations made after that await must not survive activation.
      this.disposeAll(disposables, subscriptions)
      return
    }
    this.emit()
  }

  disable(id: string): void {
    const record = this.plugins.get(id)
    if (!record?.disposables) return
    this.disposeAll(record.disposables, record.subscriptions ?? [])
    record.disposables = null
    record.subscriptions = null
    this.emit()
  }

  onDidChange(fn: () => void): Disposable {
    this.listeners.add(fn)
    return {
      dispose: () => {
        this.listeners.delete(fn)
      },
    }
  }

  private disposeAll(disposables: Set<Disposable>, subscriptions: Disposable[]): void {
    for (const disposable of [...subscriptions]) disposable.dispose()
    for (const disposable of [...disposables]) disposable.dispose()
    disposables.clear()
  }

  private emit(): void {
    for (const fn of this.listeners) fn()
  }
}
