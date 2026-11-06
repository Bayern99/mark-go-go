import { createRoot } from 'react-dom/client'
import type { AppPlugin, PluginContext } from '../../core/types'
import { OutlinePanel, OutlineStore } from './OutlinePanel'
import './outline.css'

export function createOutlinePlugin(): AppPlugin {
  return {
    id: 'outline',
    title: 'Outline',
    activate(ctx: PluginContext) {
      const store = new OutlineStore(ctx.documents.snapshot().text)

      ctx.subscriptions.push(ctx.documents.onDidChange((snapshot) => store.schedule(snapshot.text)))
      ctx.subscriptions.push({ dispose: () => store.cancelPending() })
      // Tracked by the registry: disabling the plugin removes this listener.
      ctx.editor.onSelectionChange((offset) => store.setCursorOffset(offset))

      ctx.views.register({
        id: 'outline.panel',
        title: 'Outline',
        location: 'left',
        mount(el: HTMLElement) {
          const root = createRoot(el)
          root.render(<OutlinePanel ctx={ctx} store={store} />)
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
