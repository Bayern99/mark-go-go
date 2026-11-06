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
  it('keeps a readable text column with both sidebars open', async () => {
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

  it('keeps the locked frontmatter block readable', async () => {
    await page.viewport(WINDOW.width, WINDOW.height)
    mounted = await mountApp(DOCUMENT)

    const locked = mounted.container.querySelector('.mgo-locked-block')
    if (!locked) throw new Error('locked block not rendered')
    expect(locked.getBoundingClientRect().width).toBeGreaterThanOrEqual(280)
  })
})
