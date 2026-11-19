import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'
import { page } from 'vitest/browser'
import { App } from '../src/App'
import { createBrowserHost } from '../src/host/browserHost'
import { createGitPlugin } from '../src/plugins/git'
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

it('opens Git checkpoints beside the current document when Feynman is disabled', async () => {
  const host = createBrowserHost('Draft', 'Untitled.md', { untitled: true })
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  try {
    root.render(<App host={host} plugins={[createGitPlugin(host.id)]} />)
    const git = page.getByRole('button', { name: 'Git checkpoints' })
    await git.click()
    const dialog = page.getByRole('dialog', { name: 'Git checkpoints' })
    await expect.element(dialog.getByText('Save this document to use Git checkpoints.')).toBeVisible()
    expect(git.element().closest('.docbar')).not.toBeNull()
    expect(container.querySelector('[data-testid="right-pane"]')).toBeNull()
  } finally {
    root.unmount()
    container.remove()
  }
})
