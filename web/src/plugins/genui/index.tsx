import { createRoot } from 'react-dom/client'
import type { AppPlugin, PluginContext } from '../../core/types'
import fixture from './fixture.json'
import { GenUIPanel, SnapshotStore } from './GenUIPanel'
import { parseGenUIDocument } from './schema'
import './genui.css'

export function createGenUIPlugin(): AppPlugin {
  return {
    id: 'genui',
    title: 'GenUI',
    activate(ctx: PluginContext) {
      const document = parseGenUIDocument(fixture)
      const store = new SnapshotStore(ctx.documents.snapshot())

      ctx.subscriptions.push(ctx.documents.onDidChange((snapshot) => store.set(snapshot)))

      ctx.views.register({
        id: 'genui.panel',
        title: 'Generated UI',
        mount(el: HTMLElement) {
          const root = createRoot(el)
          root.render(<GenUIPanel ctx={ctx} store={store} document={document} />)
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
