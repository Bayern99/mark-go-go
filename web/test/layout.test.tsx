import './helpers/enable-feynman'
import '../src/styles.css'
import { page, userEvent } from 'vitest/browser'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mountApp, type MountedApp } from './helpers/app'
import { MOD } from './helpers/platform'

const WINDOW = { width: 1280, height: 820 }

const DOCUMENT = `---
title: Smoke Report
tags: [a, b]
---

# 冒烟测试报告

Intro paragraph with *emphasis* and a link.

## 结论

中文段落，包含 English mixed 内容。
`

let mounted: MountedApp | undefined

afterEach(async () => {
  await mounted?.unmount()
  mounted = undefined
  localStorage.removeItem('mgo.researchWidth')
  await page.viewport(1280, 900)
})

describe('layout at the default window size', () => {
  it('keeps a readable text column with research docked', async () => {
    await page.viewport(WINDOW.width, WINDOW.height)
    mounted = await mountApp(DOCUMENT)

    const paragraph = Array.from(mounted.container.querySelectorAll('.ProseMirror p')).find((p) =>
      p.textContent?.startsWith('Intro paragraph'),
    )
    if (!paragraph) throw new Error('intro paragraph not rendered')
    expect(paragraph.getBoundingClientRect().width).toBeGreaterThanOrEqual(280)

    const heading = mounted.container.querySelector('.ProseMirror h1')
    if (!heading) throw new Error('heading not rendered')
    const lineHeight = Number.parseFloat(getComputedStyle(heading).lineHeight)
    expect(heading.getBoundingClientRect().height).toBeLessThan(lineHeight * 2)
  })

  it('keeps the frontmatter block readable', async () => {
    await page.viewport(WINDOW.width, WINDOW.height)
    mounted = await mountApp(DOCUMENT)

    const frontmatter = mounted.container.querySelector('.mgo-frontmatter-block, .mgo-frontmatter')
    if (!frontmatter) throw new Error('frontmatter block not rendered')
    expect(frontmatter.getBoundingClientRect().width).toBeGreaterThanOrEqual(280)
  })

  it('keeps fixed workspace regions free of gradient backgrounds', async () => {
    await page.viewport(WINDOW.width, WINDOW.height)
    mounted = await mountApp(DOCUMENT)

    const selectors = ['.app', '.side-pane', '.right-pane', '.doc-tabs-row', '.status-bar', '.sheet']
    for (const selector of selectors) {
      const elements = mounted.container.querySelectorAll(selector)
      expect(elements.length, selector).toBeGreaterThan(0)
      for (const element of elements) {
        expect(getComputedStyle(element).backgroundImage, selector).toBe('none')
        for (const pseudo of ['::before', '::after']) {
          if (element.matches('.side-pane, .right-pane')) continue
          expect(getComputedStyle(element, pseudo).backgroundImage, `${selector}${pseudo}`).toBe('none')
        }
      }
    }
  })

  it('auto-collapses the right pane below 1180px, opens it as .r-float on toggle, and closes on center click', async () => {
    await page.viewport(940, 720)
    mounted = await mountApp(DOCUMENT)

    const rightPane = mounted.container.querySelector<HTMLElement>('[data-testid="right-pane"]')
    const toggle = mounted.container.querySelector<HTMLButtonElement>(
      '[data-testid="right-pane-toggle"]',
    )
    if (!rightPane || !toggle) throw new Error('right pane controls missing')

    expect(rightPane.hidden).toBe(true)
    expect(rightPane.classList.contains('r-float')).toBe(false)
    expect(toggle.getAttribute('aria-expanded')).toBe('false')

    await page.getByTestId('right-pane-toggle').click()
    expect(rightPane.hidden).toBe(false)
    expect(rightPane.classList.contains('r-float')).toBe(true)
    expect(toggle.getAttribute('aria-expanded')).toBe('true')

    await page.getByTestId('right-scrim').click({ position: { x: 12, y: 120 } })
    expect(rightPane.hidden).toBe(true)
    expect(rightPane.classList.contains('r-float')).toBe(false)
    expect(toggle.getAttribute('aria-expanded')).toBe('false')
  })

  it('provides a close button on .r-float and auto-closes it when jumping via Outline', async () => {
    await page.viewport(940, 720)
    mounted = await mountApp(DOCUMENT)

    const rightPane = mounted.container.querySelector<HTMLElement>('[data-testid="right-pane"]')
    if (!rightPane) throw new Error('right pane missing')

    await page.getByTestId('right-pane-toggle').click()
    expect(rightPane.hidden).toBe(false)
    expect(rightPane.classList.contains('r-float')).toBe(true)

    await page.getByTestId('right-pane-close').click()
    expect(rightPane.hidden).toBe(true)

    await page.getByRole('tab', { name: 'Outline' }).click()
    await expect.element(page.getByRole('navigation', { name: 'Document outline' })).toBeVisible()
    await page.getByTestId('right-pane-toggle').click()
    expect(rightPane.hidden).toBe(false)

    const outlineItem = mounted.container.querySelector<HTMLButtonElement>('.outline-link')
    if (!outlineItem) throw new Error('outline item missing')
    outlineItem.click()
    await expect.poll(() => rightPane.hidden).toBe(true)
  })

  it('moves keyboard focus into the floating panel and back on Escape', async () => {
    await page.viewport(480, 360)
    mounted = await mountApp(DOCUMENT)
    const editor = mounted.container.querySelector<HTMLElement>('.ProseMirror')!
    const original = editor.textContent
    editor.focus()

    await userEvent.keyboard(`{${MOD}>}{Alt>}[Digit0]{/Alt}{/${MOD}}`)
    await expect.element(page.getByTestId('right-pane-close')).toHaveFocus()
    expect(editor.contains(window.getSelection()?.anchorNode ?? null)).toBe(false)
    await userEvent.keyboard('Q{Escape}')
    await expect.element(page.getByTestId('right-pane')).not.toBeVisible()
    await expect.element(page.getByTestId('right-pane-toggle')).toHaveFocus()
    expect(editor.textContent).toBe(original)
  })

  it('reclaims reading space at the native minimum and resets the file drawer on resize', async () => {
    await page.viewport(480, 360)
    mounted = await mountApp(DOCUMENT)
    const files = page.getByRole('button', { name: 'Show navigation' })
    const library = page.getByTestId('left-pane')
    await expect.element(library).not.toBeVisible()
    const paragraph = mounted.container.querySelector('.ProseMirror > p')!
    expect(paragraph.getBoundingClientRect().width).toBeGreaterThan(280)

    await files.click()
    await expect.element(library).toBeVisible()
    await expect.element(page.getByTestId('library-close')).toHaveFocus()
    await userEvent.keyboard('{Escape}')
    await expect.element(files).toHaveFocus()
    await files.click()
    await page.viewport(800, 640)
    await expect.element(library).toBeVisible()
    await expect.element(page.getByTestId('library-close')).not.toBeInTheDocument()
    await page.viewport(480, 360)
    await expect.element(library).not.toBeVisible()
    await expect.element(files).toBeVisible()
  })

  it('enforces a joint width budget across SidePane and RightPane at 1180px, 641px, and on resize', async () => {
    await page.viewport(1180, 820)
    mounted = await mountApp(DOCUMENT)

    const leftPane = mounted.container.querySelector<HTMLElement>('[data-testid="left-pane"]')!
    const rightPane = mounted.container.querySelector<HTMLElement>('[data-testid="right-pane"]')!
    const leftSplitter = mounted.container.querySelector<HTMLElement>('[data-testid="sidebar-splitter"]')!
    const rightSplitter = mounted.container.querySelector<HTMLElement>('.fy-resize')!
    const workspaceMain = mounted.container.querySelector<HTMLElement>('.workspace-main')!
    const paragraph = () =>
      Array.from(mounted!.container.querySelectorAll('.ProseMirror p')).find((p) =>
        p.textContent?.startsWith('Intro paragraph'),
      )!

    // Drag RightPane aggressively wider (attempting 56vw = 660px) while SidePane is docked
    rightSplitter.focus()
    for (let i = 0; i < 16; i += 1) {
      await userEvent.keyboard('{ArrowLeft}')
    }
    expect(workspaceMain.getBoundingClientRect().width).toBeGreaterThanOrEqual(412)
    expect(paragraph().getBoundingClientRect().width).toBeGreaterThanOrEqual(280)

    // Drag SidePane aggressively wider while RightPane is wide
    leftSplitter.focus()
    for (let i = 0; i < 16; i += 1) {
      await userEvent.keyboard('{ArrowRight}')
    }
    expect(workspaceMain.getBoundingClientRect().width).toBeGreaterThanOrEqual(412)
    expect(paragraph().getBoundingClientRect().width).toBeGreaterThanOrEqual(280)

    // Grow to 1440px, widen both panes, then shrink back to 1180px and grow back to 1440px
    await page.viewport(1440, 820)
    await vi.waitFor(() => expect(leftSplitter.getAttribute('aria-valuemax')).toBe('420'))
    leftSplitter.focus()
    await userEvent.keyboard('{End}')
    await vi.waitFor(() => expect(Math.round(leftPane.getBoundingClientRect().width)).toBe(420))
    rightSplitter.focus()
    for (let i = 0; i < 8; i += 1) {
      await userEvent.keyboard('{ArrowLeft}')
    }
    const wideLeft = Math.round(leftPane.getBoundingClientRect().width)
    const wideRight = Math.round(rightPane.getBoundingClientRect().width)
    expect(wideLeft).toBeGreaterThanOrEqual(400)
    expect(wideRight).toBeGreaterThan(450)

    await page.viewport(1180, 820)
    await vi.waitFor(() => {
      expect(workspaceMain.getBoundingClientRect().width).toBeGreaterThanOrEqual(412)
      expect(paragraph().getBoundingClientRect().width).toBeGreaterThanOrEqual(280)
    })

    // Restores saved target widths when window grows back to 1440px
    await page.viewport(1440, 820)
    await vi.waitFor(() => {
      expect(Math.round(leftPane.getBoundingClientRect().width)).toBe(wideLeft)
      expect(Math.round(rightPane.getBoundingClientRect().width)).toBe(wideRight)
    })

    // Narrowest docked-SidePane width (641px) still keeps center Document readable
    await page.viewport(641, 720)
    await vi.waitFor(() => {
      expect(workspaceMain.getBoundingClientRect().width).toBeGreaterThanOrEqual(412)
      expect(paragraph().getBoundingClientRect().width).toBeGreaterThanOrEqual(280)
    })
  })

  it('renders a dismissal scrim for the right floating drawer matching the left drawer and restores focus on outside click', async () => {
    await page.viewport(940, 720)
    mounted = await mountApp(DOCUMENT)

    expect(mounted.container.querySelector('[data-testid="right-scrim"]')).toBeNull()

    await page.getByTestId('right-pane-toggle').click()
    const scrim = mounted.container.querySelector<HTMLElement>('[data-testid="right-scrim"]')
    const rightPane = mounted.container.querySelector<HTMLElement>('[data-testid="right-pane"]')!
    if (!scrim) throw new Error('right scrim missing')

    await expect.element(page.getByTestId('right-scrim')).toBeVisible()
    const scrimStyle = getComputedStyle(scrim)
    const rightStyle = getComputedStyle(rightPane)
    expect(scrimStyle.backgroundColor).toBe('rgba(29, 29, 31, 0.08)')
    expect(Number(scrimStyle.zIndex)).toBeLessThan(Number(rightStyle.zIndex))

    // Clicking outside over the tab bar region hits the scrim and dismisses the drawer
    await page.getByTestId('right-scrim').click({ position: { x: 240, y: 12 } })
    expect(rightPane.hidden).toBe(true)
    expect(mounted.container.querySelector('[data-testid="right-scrim"]')).toBeNull()
    await expect.element(page.getByTestId('right-pane-toggle')).toHaveFocus()
  })
})
