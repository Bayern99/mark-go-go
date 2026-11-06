import type { Disposable } from '../host/types'
import type { ViewRegistration } from './types'

export class ViewRegistry {
  private readonly views = new Map<string, ViewRegistration>()
  private readonly listeners = new Set<() => void>()

  register(view: ViewRegistration): Disposable {
    if (this.views.has(view.id)) throw new Error(`View already registered: ${view.id}`)
    this.views.set(view.id, view)
    this.emit()
    return {
      dispose: () => {
        if (this.views.get(view.id) !== view) return
        this.views.delete(view.id)
        this.emit()
      },
    }
  }

  getAll(): ViewRegistration[] {
    return [...this.views.values()]
  }

  onDidChange(fn: () => void): Disposable {
    this.listeners.add(fn)
    return {
      dispose: () => {
        this.listeners.delete(fn)
      },
    }
  }

  private emit(): void {
    for (const fn of this.listeners) fn()
  }
}
