import '../src/styles.css'
import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'
import { page } from 'vitest/browser'
import { App } from '../src/App'
import { RightPane } from '../src/components/RightPane'
import { ViewRegistry } from '../src/core/ViewRegistry'
import type { AppPlugin, ViewLocation, ViewRegistration } from '../src/core/types'
import { createBrowserHost, createBrowserWorkspace } from '../src/host/browserHost'

function makeView(
  id: string,
  location: ViewLocation,
  mounts: string[],
  title = id,
): ViewRegistration {
  return {
    id,
    title,
    location,
    mount(el) {
      mounts.push(id)
      el.textContent = title
      return { dispose() {} }
    },
  }
}

describe('RightPane & workspace plugin shell', () => {
  it('mounts only right-location views from ViewRegistry', () => {
    const views = new ViewRegistry()
    const mounts: string[] = []
    views.register(makeView('left.view', 'left', mounts, 'Left'))
    views.register(makeView('right.view', 'right', mounts, 'Inspector'))

    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    try {
      flushSync(() => {
        root.render(
          <RightPane views={views} open floating={false} onClose={() => {}} />,
        )
      })
      expect(mounts).toEqual(['right.view'])
      const pane = container.querySelector('[data-testid="right-pane"]')
      expect(pane?.getAttribute('aria-label')).toBe('Inspector')
      expect(pane?.querySelector('[data-view-id="right.view"]')?.textContent).toBe('Inspector')
      expect(pane?.querySelector('[data-view-id="left.view"]')).toBeNull()
    } finally {
      root.unmount()
      container.remove()
    }
  })

  it('renders custom workspace-scoped right-pane plugins in App without Feynman enabled', async () => {
    const host = createBrowserHost('# Title\n\nBody text.\n', 'notes.md')
    const workspace = createBrowserWorkspace([host], { root: '/ws', name: 'ws', kind: 'folder' })
    let disposed = false

    const inspectorPlugin: AppPlugin = {
      id: 'custom.inspector',
      title: 'Inspector',
      scope: 'workspace',
      activate(ctx) {
        ctx.views.register({
          id: 'custom.inspector.view',
          title: 'Inspector',
          location: 'right',
          mount(el) {
            const box = document.createElement('div')
            box.dataset.testid = 'custom-right-view'
            box.textContent = `Workspace:${ctx.workspace?.root()} Doc:${ctx.workspace?.activeDocumentId()}`
            el.append(box)
            return {
              dispose() {
                disposed = true
                box.remove()
              },
            }
          },
        })
      },
    }

    const container = document.createElement('div')
    container.style.height = '100%'
    document.body.append(container)
    const root = createRoot(container)
    try {
      flushSync(() => {
        root.render(<App workspace={workspace} plugins={[inspectorPlugin]} />)
      })

      await expect.poll(() => container.querySelector('[data-testid="custom-right-view"]')?.textContent).toContain('Workspace:/ws')
      const toggle = page.getByTestId('right-pane-toggle')
      await expect.element(toggle).toHaveAttribute('aria-label', 'Hide Inspector')
      await toggle.click()
      await expect.element(page.getByTestId('right-pane')).not.toBeVisible()
      await expect.element(toggle).toHaveAttribute('aria-label', 'Show Inspector')
    } finally {
      root.unmount()
      container.remove()
    }
    expect(disposed).toBe(true)
  })
})
