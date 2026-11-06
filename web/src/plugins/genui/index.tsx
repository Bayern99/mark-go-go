import type { AppPlugin, PluginContext } from '../../core/types'
import type { Disposable } from '../../host/types'

/**
 * The GenUI view mode. Nothing of it loads until the view is shown: the
 * module with A2UI is imported on first mount, and the view file is only
 * watched while the view is mounted.
 */
export function createGenUIPlugin(): AppPlugin {
  return {
    id: 'genui',
    title: 'GenUI',
    activate(ctx: PluginContext) {
      ctx.views.register({
        id: 'genui',
        title: 'GenUI',
        location: 'center',
        mount: (el) => mountLazily(el, ctx),
      })
    },
  }
}

function mountLazily(el: HTMLElement, ctx: PluginContext): Disposable {
  let inner: Disposable | undefined
  let disposed = false
  void import('./GenUIView').then(
    ({ mountGenUIView }) => {
      if (!disposed) inner = mountGenUIView(el, ctx)
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
