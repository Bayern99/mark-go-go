import type { AppPlugin, PluginContext } from '../../core/types'
import type { Disposable } from '../../host/types'
import type { GenUIMemory } from './GenUIView'
import { ReadingStore } from './reading'

export function createGenUIPlugin(): AppPlugin {
  return {
    id: 'genui',
    title: 'GenUI',
    activate(ctx: PluginContext) {
      const memory: GenUIMemory = {
        text: null,
        top: 0,
        generation: undefined,
        draft: undefined,
        session: undefined,
        reading: new ReadingStore(),
        statusListeners: new Set(),
      }
      ctx.views.register({
        id: 'genui',
        title: 'GenUI',
        location: 'center',
        mount: (el, scrollElement) => mountLazily(el, ctx, memory, 'center', scrollElement),
      })
      ctx.views.register({
        id: 'genui.panel',
        title: 'GenUI',
        location: 'right',
        status: {
          subscribe(fn) {
            memory.statusListeners.add(fn)
            return () => {
              memory.statusListeners.delete(fn)
            }
          },
          getSnapshot() {
            return memory.session?.getStatusDot() ?? null
          },
        },
        mount: (el) => mountLazily(el, ctx, memory, 'right'),
      })
      ctx.subscriptions.push({
        dispose() {
          memory.session?.dispose()
          memory.session = undefined
          memory.generation?.dispose()
          memory.generation = undefined
          memory.statusListeners.clear()
        },
      })
    },
  }
}

function mountLazily(
  el: HTMLElement,
  ctx: PluginContext,
  memory: GenUIMemory,
  target: 'center' | 'right',
  scrollElement?: HTMLElement,
): Disposable {
  let inner: Disposable | undefined
  let disposed = false
  void import('./GenUIView').then(
    ({ mountGenUIView, mountGenUIPanel }) => {
      if (disposed) return
      inner =
        target === 'center'
          ? mountGenUIView(el, ctx, memory, scrollElement)
          : mountGenUIPanel(el, ctx, memory)
    },
    (error: unknown) => {
      if (disposed) return
      const message = error instanceof Error ? error.message : String(error)
      const alert = document.createElement('p')
      alert.className = 'genui-file-error'
      alert.setAttribute('role', 'alert')
      alert.textContent = `GenUI could not load: ${message}`
      el.replaceChildren(alert)
    },
  )
  return {
    dispose() {
      disposed = true
      inner?.dispose()
    },
  }
}
