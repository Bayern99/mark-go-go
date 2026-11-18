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
      ctx.editor.onSelectionChange((offset) => store.setCursorOffset(offset))

      ctx.views.register({
        id: 'outline.panel',
        title: 'Outline',
        location: 'right',
        mount(el: HTMLElement) {
          const root = createRoot(el)
          root.render(<OutlinePanel ctx={ctx} store={store} />)
          return {
            dispose() {
              queueMicrotask(() => root.unmount())
            },
          }
        },
      })
    },
  }
}
