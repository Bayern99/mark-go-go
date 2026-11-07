import '../src/styles.css'
import { page } from 'vitest/browser'
import { afterEach, describe, expect, it } from 'vitest'
import { mountApp, type MountedApp } from './helpers/app'

/** The packaged app opens document windows at 940×720 (documenthost.go). */
const WINDOW = { width: 940, height: 720 }

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
  await page.viewport(1280, 900)
})

describe('layout at the default window size', () => {
  it('keeps a readable text column with the outline open', async () => {
    await page.viewport(WINDOW.width, WINDOW.height)
    mounted = await mountApp(DOCUMENT)

    const paragraph = Array.from(mounted.container.querySelectorAll('.ProseMirror p')).find((p) =>
      p.textContent?.startsWith('Intro paragraph'),
    )
    if (!paragraph) throw new Error('intro paragraph not rendered')
    expect(paragraph.getBoundingClientRect().width).toBeGreaterThanOrEqual(280)

    const heading = mounted.container.querySelector('.ProseMirror h1')
    if (!heading) throw new Error('heading not rendered')
    // A one-line heading: CJK text must not wrap a glyph per line.
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

  it('auto-collapses the right pane below 1180px, opens it as .r-float on toggle, and closes on center click', async () => {
    await page.viewport(WINDOW.width, WINDOW.height)
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

    await page.getByText('结论').first().click()
    expect(rightPane.hidden).toBe(true)
    expect(rightPane.classList.contains('r-float')).toBe(false)
    expect(toggle.getAttribute('aria-expanded')).toBe('false')
  })

  it('exposes Portrait semantic tokens in light and dark mode', async () => {
    mounted = await mountApp(DOCUMENT)
    const appEl = mounted.container.querySelector<HTMLElement>('.app.mgg')
    if (!appEl) throw new Error('app root missing')

    const lightStyle = getComputedStyle(appEl)
    expect(lightStyle.getPropertyValue('--chrome').trim()).toBe('#f2f2f4')
    expect(lightStyle.getPropertyValue('--paper').trim()).toBe('#ffffff')
    expect(lightStyle.getPropertyValue('--accent').trim()).toBe('#2f6f62')
    expect(lightStyle.getPropertyValue('--warn').trim()).toBe('#9a4d12')
    expect(lightStyle.getPropertyValue('--danger').trim()).toBe('#a83a3a')
    expect(lightStyle.getPropertyValue('--raised-shadow').trim()).not.toBe('')
    expect(lightStyle.getPropertyValue('--sunken-shadow').trim()).not.toBe('')
    expect(lightStyle.getPropertyValue('--pop-shadow').trim()).not.toBe('')

    appEl.classList.add('dark')
    const darkStyle = getComputedStyle(appEl)
    expect(darkStyle.getPropertyValue('--chrome').trim()).toBe('#1e1e20')
    expect(darkStyle.getPropertyValue('--paper').trim()).toBe('#19191b')
    expect(darkStyle.getPropertyValue('--accent').trim()).toBe('#7cc4b2')
    expect(darkStyle.getPropertyValue('--warn').trim()).toBe('#f0a560')
    expect(darkStyle.getPropertyValue('--danger').trim()).toBe('#ef8a80')
  })
})
