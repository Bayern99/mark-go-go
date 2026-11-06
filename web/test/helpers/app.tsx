import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { vi } from 'vitest'
import { App } from '../../src/App'
import { createBrowserHost } from '../../src/host/browserHost'
import type { HostAdapter } from '../../src/host/types'

export interface MountedApp {
  host: HostAdapter
  container: HTMLDivElement
  unmount(): Promise<void>
}

export async function mountApp(
  text: string,
  host: HostAdapter = createBrowserHost(text),
): Promise<MountedApp> {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  flushSync(() => {
    root.render(<App host={host} />)
  })

  await vi.waitFor(
    () => {
      const modeToggle = container.querySelector<HTMLButtonElement>(
        '[data-testid="mode-toggle"]',
      )
      if (!modeToggle || modeToggle.disabled) throw new Error('editor not ready yet')
      if (!container.querySelector('[data-testid="editor-root"] .ProseMirror'))
        throw new Error('editor not mounted yet')
      if (!container.querySelector('.genui-heading-link')) throw new Error('GenUI headings missing')
    },
    { timeout: 5000 },
  )

  return {
    host,
    container,
    async unmount() {
      root.unmount()
      container.remove()
      await new Promise<void>((resolve) => setTimeout(resolve, 0))
    },
  }
}
