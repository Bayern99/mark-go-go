import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'
import { RightPane } from '../src/components/RightPane'
import { SidePane } from '../src/components/SidePane'
import { ViewRegistry } from '../src/core/ViewRegistry'
import type { ViewLocation, ViewRegistration, ViewStatusDot } from '../src/core/types'

function makeView(
  id: string,
  location: ViewLocation,
  mounts: string[],
  title = id,
  dot: ViewStatusDot = null,
): ViewRegistration {
  return {
    id,
    title,
    location,
    status: dot
      ? {
          subscribe: () => () => {},
          getSnapshot: () => dot,
        }
      : undefined,
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

function renderRightPane(
  views: ViewRegistry,
  props: Partial<React.ComponentProps<typeof RightPane>> = {},
) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  flushSync(() => {
    root.render(<RightPane views={views} {...props} />)
  })
  return {
    container,
    cleanup() {
      root.unmount()
      container.remove()
    },
  }
}

describe('RightPane', () => {
  it('mounts the first right view by default and ignores left and center views', () => {
    const views = new ViewRegistry()
    const mounts: string[] = []
    views.register(makeView('left.view', 'left', mounts))
    views.register(makeView('center.view', 'center', mounts))
    views.register(makeView('outline.panel', 'right', mounts, 'Outline'))
    views.register(makeView('genui.panel', 'right', mounts, 'GenUI'))

    const rendered = renderRightPane(views)
    try {
      expect(mounts).toEqual(['outline.panel'])
      const right = rendered.container.querySelector('[data-testid="right-pane"]')
      expect(right).not.toBeNull()
      const tabs = right?.querySelectorAll('[role="tab"]')
      expect(tabs).toHaveLength(2)
      expect(tabs?.[0]?.getAttribute('aria-selected')).toBe('true')
      expect(tabs?.[1]?.getAttribute('aria-selected')).toBe('false')
      expect(right?.querySelector('[data-view-id="outline.panel"]')).not.toBeNull()
      expect(right?.querySelector('[data-view-id="genui.panel"]')).toBeNull()
    } finally {
      rendered.cleanup()
    }
  })

  it('switches between right views when clicking segmented tabs and shows status dots', () => {
    const views = new ViewRegistry()
    const mounts: string[] = []
    views.register(makeView('outline.panel', 'right', mounts, 'Outline'))
    views.register(
      makeView('genui.panel', 'right', mounts, 'GenUI', {
        tone: 'warn',
        title: 'A source changed',
      }),
    )

    const rendered = renderRightPane(views)
    try {
      const right = rendered.container.querySelector('[data-testid="right-pane"]')
      const dot = right?.querySelector('[data-testid="right-tab-dot-genui.panel"]')
      expect(dot).not.toBeNull()
      expect(dot?.getAttribute('data-tone')).toBe('warn')

      const genuiTab = right?.querySelector<HTMLButtonElement>(
        '[data-testid="right-tab-genui.panel"]',
      )
      expect(genuiTab).not.toBeNull()
      flushSync(() => {
        genuiTab?.click()
      })

      expect(mounts).toEqual(['outline.panel', 'genui.panel'])
      expect(genuiTab?.getAttribute('aria-selected')).toBe('true')
      expect(right?.querySelector('[data-view-id="genui.panel"]')).not.toBeNull()
      expect(right?.querySelector('[data-view-id="outline.panel"]')).toBeNull()
    } finally {
      rendered.cleanup()
    }
  })
})
