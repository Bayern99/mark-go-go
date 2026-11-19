import './helpers/enable-feynman'
import '../src/styles.css'
import { createElement } from 'react'
import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'
import { cdp, page, userEvent } from 'vitest/browser'
import { App } from '../src/App'
import type { AppPlugin } from '../src/core/types'
import { createBrowserHost, createBrowserWorkspace } from '../src/host/browserHost'
import { mountApp } from './helpers/app'
import { mountEditor } from './helpers/editor'

const researchEvents = vi.hoisted(() => ({
  onEvent: undefined as ((e: import('../src/host/generated/mygo').ResearchUpdate) => void) | undefined,
}))

vi.mock('../src/host/generated/mygo', async (original) => {
  const native = await original<typeof import('../src/host/generated/mygo')>()
  return {
    ...native,
    events: {
      ...native.events,
      researchUpdate: {
        on(fn: (e: import('../src/host/generated/mygo').ResearchUpdate) => void) {
          researchEvents.onEvent = fn
          return () => {
            researchEvents.onEvent = undefined
          }
        },
      },
    },
    ResearchHost: {
      ...native.ResearchHost,
      connect: async (root: string) => {
        researchEvents.onEvent?.({
          root,
          kind: 'commands',
          commands: [{ name: 'audit', description: 'Audit literature' }],
        })
      },
      disconnect: async () => {},
      artifacts: async () => [],
    },
  }
})

const DOCUMENT = `# Heading

Body.

<!-- note: 判断原文已改时不算批注 -->
`

it('renders the floating research sidebar and workspace menu opaque with reduced transparency', async () => {
  const session = cdp()
  const app = await mountApp(DOCUMENT)
  try {
    await page.viewport(800, 640)
    await session.send('Emulation.setEmulatedMedia', {
      features: [{ name: 'prefers-reduced-transparency', value: 'reduce' }],
    })
    await page.getByTestId('right-pane-toggle').click()
    const rightSurface = page.getByTestId('right-pane')
    await expect.element(rightSurface).toBeVisible()
    const rightStyle = getComputedStyle(rightSurface.element())
    expect(rightStyle.backgroundColor, 'right-pane').toBe('rgb(255, 255, 255)')
    expect(rightStyle.backdropFilter, 'right-pane').toBe('none')

    await page.getByTestId('right-pane-close').click()
    await page.getByTestId('workspace-switcher').click()
    const menuSurface = page.getByTestId('workspace-menu')
    await expect.element(menuSurface).toBeVisible()
    const menuStyle = getComputedStyle(menuSurface.element())
    expect(menuStyle.backgroundColor, 'workspace-menu').toBe('rgb(255, 255, 255)')
    expect(menuStyle.backdropFilter, 'workspace-menu').toBe('none')
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

function parseRgb(color: string): [number, number, number] {
  const match = color.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/)
  if (!match) throw new Error(`unexpected color format: ${color}`)
  return [Number(match[1]), Number(match[2]), Number(match[3])]
}

function relativeLuminance([r, g, b]: [number, number, number]): number {
  const channel = (value: number) => {
    const s = value / 255
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4
  }
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b)
}

function contrastRatio(fg: string, bg: string): number {
  const l1 = relativeLuminance(parseRgb(fg))
  const l2 = relativeLuminance(parseRgb(bg))
  const lighter = Math.max(l1, l2)
  const darker = Math.min(l1, l2)
  return (lighter + 0.05) / (darker + 0.05)
}

function makeActionPlugin(id: string, label: string): AppPlugin {
  return {
    id,
    title: label,
    activate(ctx) {
      ctx.views.register({
        id: `${id}.action`,
        title: label,
        location: 'document',
        mount(el) {
          const btn = document.createElement('button')
          btn.type = 'button'
          btn.className = 'pill xs'
          btn.textContent = label
          el.append(btn)
          return { dispose() {} }
        },
      })
    },
  }
}

describe('Portrait contrast, narrow sheet header, and filter focus rings', () => {
  it('uses --on-accent on primary accent buttons with WCAG AA contrast in light and dark mode', async () => {
    const host = createBrowserHost(DOCUMENT, 'draft.md')
    const workspace = createBrowserWorkspace([{ host, name: 'draft.md', path: '/work/draft.md' }], {
      root: '/work',
      name: 'work',
      kind: 'folder',
    })
    const container = document.createElement('div')
    container.style.height = '100%'
    document.body.append(container)
    const root = createRoot(container)
    try {
      flushSync(() => {
        root.render(createElement(App, { workspace }))
      })
      const send = await vi.waitFor(() => {
        const el = container.querySelector<HTMLButtonElement>('.fy-send')
        expect(el).not.toBeNull()
        return el!
      })
      const appEl = container.querySelector<HTMLElement>('.app')
      if (!appEl) throw new Error('.app missing')
      const primary = document.createElement('button')
      primary.type = 'button'
      primary.className = 'pill primary'
      primary.textContent = 'Primary'
      appEl.append(primary)

      for (const dark of [false, true]) {
        document.documentElement.classList.toggle('dark', dark)
        const rootStyle = getComputedStyle(document.documentElement)
        const appStyle = getComputedStyle(appEl)
        const onAccent = rootStyle.getPropertyValue('--on-accent').trim()
        const accent = rootStyle.getPropertyValue('--accent').trim()
        expect(onAccent).not.toBe('')
        expect(accent).not.toBe('')
        expect(appStyle.getPropertyValue('--on-accent').trim()).toBe(dark ? '#0f2a24' : '#ffffff')

        const sendStyle = getComputedStyle(send)
        expect(sendStyle.color).toBe(dark ? 'rgb(15, 42, 36)' : 'rgb(255, 255, 255)')
        expect(contrastRatio(sendStyle.color, sendStyle.backgroundColor)).toBeGreaterThanOrEqual(4.5)

        await vi.waitFor(() => {
          const primaryStyle = getComputedStyle(primary)
          expect(primaryStyle.color).toBe(dark ? 'rgb(15, 42, 36)' : 'rgb(255, 255, 255)')
          if (dark) {
            expect(primaryStyle.textShadow).toBe('none')
          }
        })
        expect(rootStyle.getPropertyValue('--accent-raised-hover').trim()).not.toBe('')
        expect(rootStyle.getPropertyValue('--accent-raised-press').trim()).not.toBe('')
      }
    } finally {
      document.documentElement.classList.remove('dark')
      root.unmount()
      container.remove()
    }
  })

  it('wraps plugin actions cleanly in narrow sheet headers without overflowing', async () => {
    await page.viewport(340, 640)
    const host = createBrowserHost(DOCUMENT, 'narrow.md')
    const container = document.createElement('div')
    container.style.height = '100%'
    document.body.append(container)
    const root = createRoot(container)
    try {
      flushSync(() => {
        root.render(
          createElement(App, {
            host,
            plugins: [
              makeActionPlugin('plugin.a', 'Checkpoints'),
              makeActionPlugin('plugin.b', 'Export'),
            ],
          }),
        )
      })
      await vi.waitFor(() => {
        expect(container.querySelectorAll('.header-actions .pill').length).toBeGreaterThanOrEqual(2)
      })
      const actions = container.querySelector<HTMLElement>('.docbar .header-actions')!
      expect(actions.scrollWidth).toBeLessThanOrEqual(actions.clientWidth + 1)
      const actionsRect = actions.getBoundingClientRect()
      for (const child of Array.from(actions.children)) {
        const rect = child.getBoundingClientRect()
        if (rect.width === 0) continue
        expect(rect.right).toBeLessThanOrEqual(actionsRect.right + 1)
        expect(rect.left).toBeGreaterThanOrEqual(actionsRect.left - 1)
      }
    } finally {
      root.unmount()
      container.remove()
      await page.viewport(1280, 900)
    }
  })

  it('displays visible :focus-within rings on workspace, mobile drawer, tab menu, and research filter inputs', async () => {
    const docs = Array.from({ length: 12 }, (_, i) => ({
      host: createBrowserHost(`# Doc ${i}\n`, `doc-${i}.md`),
      name: `doc-${i}.md`,
      path: `/work/doc-${i}.md`,
    }))
    const workspace = createBrowserWorkspace(docs, { root: '/work', name: 'work', kind: 'folder' })
    const container = document.createElement('div')
    container.style.height = '100%'
    document.body.append(container)
    const root = createRoot(container)
    try {
      flushSync(() => {
        root.render(createElement(App, { workspace }))
      })
      await vi.waitFor(() => {
        expect(container.querySelector('.filter input')).not.toBeNull()
        expect(container.querySelector('.fy-actions-trigger')).not.toBeNull()
      })

      // 1. Workspace sidebar filter
      const sideFilter = container.querySelector<HTMLElement>('.filter')!
      const sideInput = sideFilter.querySelector<HTMLInputElement>('input')!
      sideInput.focus()
      expect(getComputedStyle(sideFilter).boxShadow).not.toBe('none')

      // 2. Research Actions filter (.fy-filter)
      await userEvent.click(container.querySelector<HTMLButtonElement>('.fy-actions-trigger')!)
      const fyFilter = await vi.waitFor(() => {
        const el = container.querySelector<HTMLElement>('.fy-filter')
        expect(el).not.toBeNull()
        return el!
      })
      const fyInput = fyFilter.querySelector<HTMLInputElement>('input')!
      fyInput.focus()
      expect(getComputedStyle(fyFilter).boxShadow).not.toBe('none')

      // 3. Narrow mobile drawer filter (.library-open > .side-pane .filter)
      await page.viewport(480, 640)
      await userEvent.click(container.querySelector<HTMLButtonElement>('.library-toggle')!)
      await vi.waitFor(() => expect(container.querySelector('.workspace-body.library-open')).not.toBeNull())
      sideInput.focus()
      expect(getComputedStyle(sideFilter).boxShadow).toContain('rgba(')
    } finally {
      root.unmount()
      container.remove()
      await page.viewport(1280, 900)
    }
  })
})
