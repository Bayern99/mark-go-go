import '../src/styles.css'
import { describe, expect, it, vi } from 'vitest'
import { cdp, page } from 'vitest/browser'
import { mountApp } from './helpers/app'
import { mountEditor } from './helpers/editor'

const DOCUMENT = `# Heading

Body.

<!-- note: 判断原文已改时不算批注 -->
`

it('renders the floating document panel and workspace menu opaque with reduced transparency', async () => {
  const session = cdp()
  const app = await mountApp(DOCUMENT)
  try {
    await page.viewport(800, 640)
    await session.send('Emulation.setEmulatedMedia', {
      features: [{ name: 'prefers-reduced-transparency', value: 'reduce' }],
    })
    await page.getByTestId('right-pane-toggle').click()
    await page.getByTestId('workspace-switcher').click()
    for (const id of ['right-pane', 'workspace-menu']) {
      const surface = page.getByTestId(id)
      await expect.element(surface).toBeVisible()
      const style = getComputedStyle(surface.element())
      expect(style.backgroundColor, id).toBe('rgb(255, 255, 255)')
      expect(style.backdropFilter, id).toBe('none')
    }
  } finally {
    await session.send('Emulation.setEmulatedMedia', { features: [] })
    await page.viewport(1280, 900)
    await app.unmount()
  }
})

describe('Portrait notes', () => {
  it('shows note secondary actions on keyboard focus, not only hover', async () => {
    const editor = await mountEditor(DOCUMENT)
    try {
      const note = editor.root.querySelector<HTMLElement>('.mgo-note')
      const actions = note?.querySelector<HTMLElement>('.mgo-note-actions')
      const edit = note?.querySelector<HTMLButtonElement>('[data-note-action="edit"]')
      if (!note || !actions || !edit) throw new Error('note actions missing')
      expect(getComputedStyle(actions).opacity).toBe('0')
      edit.focus()
      await vi.waitFor(() => expect(getComputedStyle(actions).opacity).toBe('1'))
    } finally {
      await editor.cleanup()
    }
  })
})
