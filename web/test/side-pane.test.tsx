import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'
import { SidePane } from '../src/components/SidePane'
import { ViewRegistry } from '../src/core/ViewRegistry'
import type { ViewRegistration } from '../src/core/types'

function makeView(id: string, location: 'left' | 'right' | undefined, mounts: string[]): ViewRegistration {
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

function renderPanes(views: ViewRegistry) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  flushSync(() => {
    root.render(
      <>
        <SidePane views={views} location="left" />
        <SidePane views={views} location="right" />
      </>,
    )
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
  it('mounts each pre-registered view exactly once in its own pane', () => {
    const views = new ViewRegistry()
    const mounts: string[] = []
    views.register(makeView('left.view', 'left', mounts))
    views.register(makeView('right.view', undefined, mounts))
    const rendered = renderPanes(views)
    try {
      expect([...mounts].sort()).toEqual(['left.view', 'right.view'])

      const left = rendered.container.querySelector('[data-testid="left-pane"]')
      const right = rendered.container.querySelector('[data-testid="side-pane"]')
      expect(left?.querySelectorAll('[data-view-id]')).toHaveLength(1)
      expect(left?.querySelector('[data-view-id="left.view"]')).not.toBeNull()
      expect(left?.querySelector('[data-view-id="right.view"]')).toBeNull()
      expect(right?.querySelectorAll('[data-view-id]')).toHaveLength(1)
      expect(right?.querySelector('[data-view-id="right.view"]')).not.toBeNull()
      expect(right?.querySelector('[data-view-id="left.view"]')).toBeNull()
    } finally {
      rendered.cleanup()
    }
  })

  it('renders no left pane when only right views are registered', () => {
    const views = new ViewRegistry()
    const mounts: string[] = []
    views.register(makeView('right.view', 'right', mounts))
    const rendered = renderPanes(views)
    try {
      expect(mounts).toEqual(['right.view'])
      expect(rendered.container.querySelector('[data-testid="left-pane"]')).toBeNull()
      expect(rendered.container.querySelector('[data-view-id="right.view"]')).not.toBeNull()
    } finally {
      rendered.cleanup()
    }
  })
})
