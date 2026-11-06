import type { AppPlugin, PluginContext } from '../../core/types'
import type { Disposable } from '../../host/types'
import type { ScrollMemory } from './GenUIView'

/**
 * The GenUI view mode. Nothing of it loads until the view is shown: the
 * module with A2UI is imported on first mount, and the view file and the
 * document are only followed while the view is mounted. Where the reader
 * was in the view is kept across mounts, so editing the source and coming
 * back lands at the same place.
 */
export function createGenUIPlugin(): AppPlugin {
  return {
    id: 'genui',
    title: 'GenUI',
    activate(ctx: PluginContext) {
      const memory: ScrollMemory = { text: null, top: 0 }
      ctx.views.register({
        id: 'genui',
        title: 'GenUI',
        location: 'center',
        mount: (el) => mountLazily(el, ctx, memory),
      })
    },
  }
}

function mountLazily(el: HTMLElement, ctx: PluginContext, memory: ScrollMemory): Disposable {
  let inner: Disposable | undefined
  let disposed = false
  void import('./GenUIView').then(
    ({ mountGenUIView }) => {
      if (!disposed) inner = mountGenUIView(el, ctx, memory)
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
