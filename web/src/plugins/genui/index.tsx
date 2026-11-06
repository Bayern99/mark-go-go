import { createRoot } from 'react-dom/client'
import type { AppPlugin, PluginContext } from '../../core/types'
import { GenUIPanel, GenUIStore } from './GenUIPanel'
import './genui.css'

export function createGenUIPlugin(): AppPlugin {
  return {
    id: 'genui',
    title: 'GenUI',
    activate(ctx: PluginContext) {
      const store = new GenUIStore(ctx.documents.snapshot().text)

      ctx.subscriptions.push(ctx.documents.onDidChange((snapshot) => store.schedule(snapshot.text)))
      ctx.subscriptions.push({ dispose: () => store.cancelPending() })
      // Tracked by the registry: disabling the plugin stops the host's watch.
      ctx.viewFile.watch((snap) => store.setView(snap))

      ctx.views.register({
        id: 'genui.panel',
        title: 'Generated UI',
        mount(el: HTMLElement) {
          const root = createRoot(el)
          root.render(<GenUIPanel ctx={ctx} store={store} />)
          return {
            dispose() {
              // Defer: React may be committing the host tree when the panel is
              // removed, and a nested root must not unmount during that render.
              queueMicrotask(() => root.unmount())
            },
          }
        },
      })
    },
  }
}
