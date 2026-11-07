import '../src/styles.css'
import '../src/editor/crepe-theme.css'
import { page } from 'vitest/browser'
import { afterEach, describe, expect, it } from 'vitest'
import {
  PORTRAIT_CONTENT_TOKENS,
  PORTRAIT_FONT_TOKENS,
  PORTRAIT_MOTION_TOKENS,
  PORTRAIT_OFFLINE_FONT_FAMILIES,
  PORTRAIT_STATUS_TOKENS,
  PORTRAIT_TYPE_TOKENS,
} from '../src/theme/portrait-tokens'
import { mountApp, type MountedApp } from './helpers/app'
import { mountEditor } from './helpers/editor'

const DOCUMENT = `---
title: Portrait
---

# 视图文件用 A2UI 1.0

修改走跳转：切到 Rich text，选中来源处的原文。

<!-- note: 判断原文已改时不算批注 -->

<div align="center"><img src="logo.png"></div>
`

let mounted: MountedApp | undefined

afterEach(async () => {
  await mounted?.unmount()
  mounted = undefined
  await page.viewport(1280, 900)
})

function declared(token: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(token).trim()
}

function fontFaceSources(): { family: string; src: string }[] {
  const faces: { family: string; src: string }[] = []
  for (const sheet of Array.from(document.styleSheets)) {
    let rules: CSSRuleList
    try {
      rules = sheet.cssRules
    } catch {
      continue
    }
    for (const rule of Array.from(rules)) {
      if (rule instanceof CSSFontFaceRule) {
        faces.push({
          family: rule.style.getPropertyValue('font-family').replaceAll(/["']/g, ''),
          src: rule.style.getPropertyValue('src'),
        })
      }
    }
  }
  return faces
}

function paintColor(cssColor: string): string {
  const probe = document.createElement('span')
  probe.style.color = cssColor
  document.body.append(probe)
  const value = getComputedStyle(probe).color
  probe.remove()
  return value
}

describe('Portrait light design contract', () => {
  it('defines shared tokens independently of content warning/caution', () => {
    for (const token of [
      ...PORTRAIT_FONT_TOKENS,
      ...PORTRAIT_TYPE_TOKENS,
      ...PORTRAIT_STATUS_TOKENS,
      ...PORTRAIT_CONTENT_TOKENS,
      ...PORTRAIT_MOTION_TOKENS,
    ]) {
      expect(declared(token), token).not.toBe('')
    }
    expect(declared('--warn')).toBe('#9a4d12')
    expect(declared('--danger')).toBe('#a83a3a')
    expect(declared('--content-warning')).toBe('#9a4d12')
    expect(declared('--content-caution')).toBe('#a83a3a')
    expect(declared('--accent')).toBe('#2f6f62')
    expect(declared('--content-note')).toBe('var(--accent)')
    expect(paintColor('var(--content-note)')).not.toBe(paintColor('var(--warn)'))
    expect(paintColor('var(--content-caution)')).toBe(paintColor('var(--danger)'))
  })

  it('ships @font-face locally for the Portrait families, not a CDN', async () => {
    await document.fonts.ready
    const faces = fontFaceSources()
    expect(faces.length).toBeGreaterThan(8)
    for (const face of faces) {
      expect(face.src).not.toMatch(/fonts\.googleapis|fonts\.gstatic|cdn\.jsdelivr|unpkg\.com/i)
    }
    const families = new Set(faces.map((face) => face.family))
    for (const family of PORTRAIT_OFFLINE_FONT_FAMILIES) {
      expect(families.has(family), family).toBe(true)
    }
  })

  it('maps Rich text onto Portrait type and drops Crepe classic faces', async () => {
    mounted = await mountApp(DOCUMENT)
    const heading = mounted.container.querySelector('.ProseMirror h1')
    const paragraph = Array.from(mounted.container.querySelectorAll('.ProseMirror p')).find((p) =>
      p.textContent?.includes('修改走跳转'),
    )
    if (!heading || !paragraph) throw new Error('heading or paragraph missing')
    const hStyle = getComputedStyle(heading)
    const pStyle = getComputedStyle(paragraph)
    expect(hStyle.fontFamily).toMatch(/MGG Condensed|Barlow Semi Condensed|IBM Plex Sans/)
    expect(hStyle.fontFamily).not.toMatch(/Georgia|Times New Roman|Open Sans|Fira Code/i)
    expect(hStyle.fontSize).toBe('28px')
    expect(hStyle.lineHeight).toBe('36px')
    expect(pStyle.fontSize).toBe('16px')
    expect(pStyle.lineHeight).toBe('28px')
    expect(pStyle.fontFamily).toMatch(/IBM Plex Sans/)
    const crepe = mounted.container.querySelector<HTMLElement>('.crepe-root')
    if (!crepe) throw new Error('crepe root missing')
    expect(getComputedStyle(crepe).getPropertyValue('--crepe-font-title').trim()).toBe(
      'var(--font-display)',
    )
    expect(getComputedStyle(crepe).getPropertyValue('--crepe-font-default').trim()).toBe(
      'var(--font-sans)',
    )
    expect(getComputedStyle(crepe).getPropertyValue('--crepe-color-background').trim()).toBe(
      'var(--paper)',
    )
  })

  it('renders Kept as written material and deleted banners as danger', async () => {
    mounted = await mountApp(DOCUMENT)
    const locked = mounted.container.querySelector('.mgo-locked-block')
    if (!locked) throw new Error('locked block missing')
    expect(getComputedStyle(locked, '::before').content).toMatch(/Kept as written/)
    const deleted = document.createElement('div')
    deleted.className = 'banner banner-deleted'
    mounted.container.append(deleted)
    expect(getComputedStyle(deleted).color).toBe(paintColor('var(--danger)'))
    expect(getComputedStyle(deleted).color).not.toBe(paintColor('var(--warn)'))
  })

  it('shows note secondary actions on keyboard focus, not only hover', async () => {
    const editor = await mountEditor(DOCUMENT)
    try {
      const note = editor.root.querySelector<HTMLElement>('.mgo-note')
      const actions = note?.querySelector<HTMLElement>('.mgo-note-actions')
      const edit = note?.querySelector<HTMLButtonElement>('[data-note-action="edit"]')
      if (!note || !actions || !edit) throw new Error('note actions missing')
      expect(getComputedStyle(actions).opacity).toBe('0')
      edit.focus()
      expect(getComputedStyle(actions).opacity).toBe('1')
      expect(edit.className).toContain('pill')
      expect(edit.className).toContain('xs')
    } finally {
      await editor.cleanup()
    }
  })
})
