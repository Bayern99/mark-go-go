import '../src/styles.css'
import { page } from 'vitest/browser'
import { afterEach, describe, expect, it } from 'vitest'
import { mountApp, type MountedApp } from './helpers/app'

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

    await page.getByText('结论').first().click()
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

    await page.getByTestId('right-pane-toggle').click()
    expect(rightPane.hidden).toBe(false)

    const outlineItem = mounted.container.querySelector<HTMLButtonElement>('.outline-link')
    if (!outlineItem) throw new Error('outline item missing')
    outlineItem.click()
    await expect.poll(() => rightPane.hidden).toBe(true)
  })

})
