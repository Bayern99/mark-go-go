import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'
import { SidePane } from '../src/components/SidePane'
import { ViewRegistry } from '../src/core/ViewRegistry'
import type { ViewLocation, ViewRegistration } from '../src/core/types'

function makeView(id: string, location: ViewLocation, mounts: string[]): ViewRegistration {
  return {
    id,
    title: id,
    location,
    mount(el) {
      mounts.push(id)
      el.textContent = id
      return { dispose() {} }
    },
  }
}

function renderPane(views: ViewRegistry) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  flushSync(() => {
    root.render(<SidePane views={views} />)
  })
  return {
    container,
    cleanup() {
      root.unmount()
      container.remove()
    },
  }
}

describe('SidePane', () => {
  it('mounts left views once and never mounts center views', () => {
    const views = new ViewRegistry()
    const mounts: string[] = []
    views.register(makeView('left.view', 'left', mounts))
    views.register(makeView('center.view', 'center', mounts))
    const rendered = renderPane(views)
    try {
      expect(mounts).toEqual(['left.view'])
      const left = rendered.container.querySelector('[data-testid="left-pane"]')
      expect(left?.querySelectorAll('[data-view-id]')).toHaveLength(1)
      expect(left?.querySelector('[data-view-id="left.view"]')).not.toBeNull()
      expect(rendered.container.querySelector('[data-view-id="center.view"]')).toBeNull()
    } finally {
      rendered.cleanup()
    }
  })

  it('renders no pane when only center views are registered', () => {
    const views = new ViewRegistry()
    const mounts: string[] = []
    views.register(makeView('center.view', 'center', mounts))
    const rendered = renderPane(views)
    try {
      expect(mounts).toEqual([])
      expect(rendered.container.querySelector('[data-testid="left-pane"]')).toBeNull()
    } finally {
      rendered.cleanup()
    }
  })
})
