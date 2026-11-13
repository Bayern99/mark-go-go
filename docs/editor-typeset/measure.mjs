#!/usr/bin/env node
/**
 * Editor typesetting measurement harness.
 *
 * Usage:
 *   node measure.mjs --label <baseline|after> [--engine webkit|chromium|both]
 *
 * Requires the dev server:  cd web && npx vite   (http://localhost:5173)
 * Writes results to docs/research/editor-typeset/results/<label>/<engine>/ (gitignored)
 */
import { createRequire } from 'node:module'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const HERE = path.dirname(fileURLToPath(import.meta.url))
const require = createRequire(path.join(HERE, '../../web/package.json'))
const { chromium, webkit } = require('playwright')

const DOCS = ['paper-bilingual', 'tech-report-tables', 'llm-research-report']
const BASE_URL = 'http://localhost:5173'
const BENCH_PAGE = `${BASE_URL}/bench/typeset.html`
const VIEWPORT = { width: 1440, height: 900 }
const SCALE = 2

function parseArgs(argv) {
  const args = { label: 'baseline', engine: 'both', variant: '' }
  for (let i = 2; i < argv.length; i++) {
    if (argv[i] === '--label') args.label = argv[++i]
    else if (argv[i] === '--engine') args.engine = argv[++i]
    else if (argv[i] === '--variant') args.variant = argv[++i]
  }
  return args
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

// ---------------------------------------------------------------------------
// In-page helpers (serialised into the browser context)
// ---------------------------------------------------------------------------

const PAGE_HELPERS = `
function pmRoot() { return document.querySelector('.crepe-root .ProseMirror') || document.querySelector('.ProseMirror') }
function pmView() { return window.__pmView || null }
function visibleTables() {
  return [...pmRoot().querySelectorAll('table')].filter((t) => {
    const r = t.getBoundingClientRect()
    return r.width > 0 && r.height > 0 && !t.closest('.drag-preview')
  })
}
function scrollViewport() {
  let n = pmRoot()
  while (n && n !== document.body) {
    const s = getComputedStyle(n)
    if (/(auto|scroll)/.test(s.overflowY) && n.scrollHeight > n.clientHeight + 4) return n
    n = n.parentElement
  }
  return document.scrollingElement || document.documentElement
}
function describeEl(el) {
  if (!el) return null
  const cls = typeof el.className === 'string' ? el.className.trim().split(/\\s+/).join('.') : ''
  return { tag: el.tagName.toLowerCase(), class: cls || null, id: el.id || null }
}
function blockKind(el) {
  const t = el.tagName.toLowerCase()
  if (/^h[1-6]$/.test(t)) return t
  if (t === 'p' || t === 'ul' || t === 'ol' || t === 'blockquote' || t === 'pre' || t === 'hr' || t === 'table') {
    return (t === 'ul' || t === 'ol') ? 'list' : t
  }
  if (el.matches('.milkdown-table-block, .table-wrapper')) return 'table'
  if (el.matches('[data-locked-block], [data-kind], .mgo-locked-block, aside.mgo-note')) {
    return 'locked:' + (el.dataset.kind || 'unknown')
  }
  if (el.querySelector('.cm-editor')) return 'code(cm)'
  const cls = typeof el.className === 'string' ? el.className.trim().split(/\\s+/)[0] : ''
  return t + (cls ? '.' + cls : '')
}
function rectOf(el) { const r = el.getBoundingClientRect(); return { x: r.x, y: r.y, w: r.width, h: r.height, top: r.top, right: r.right, bottom: r.bottom, left: r.left } }
function median(arr) {
  if (!arr.length) return null
  const s = [...arr].sort((a, b) => a - b)
  const m = s.length >> 1
  return s.length % 2 ? s[m] : (s[m - 1] + s[m]) / 2
}
function summarize(nums) {
  if (!nums.length) return null
  return { min: Math.min(...nums), median: median(nums), max: Math.max(...nums), n: nums.length }
}
function readMarkdown() {
  return window.__bench.host.load().then((s) => s.text)
}
`

async function evalPage(page, body) {
  return page.evaluate(`(() => { ${PAGE_HELPERS}; return (${body}) })()`)
}

// ---------------------------------------------------------------------------
// A. computed styles
// ---------------------------------------------------------------------------

async function measureStyles(page) {
  return evalPage(page, `(() => {
    const root = pmRoot()
    const SPEC = [
      ['p', 'p'],
      ['h1', 'h1'], ['h2', 'h2'], ['h3', 'h3'], ['h4', 'h4'],
      ['li', 'li'], ['li p', 'li p'],
      ['blockquote', 'blockquote'], ['blockquote p', 'blockquote p'],
      ['pre', 'pre'], ['pre code', 'pre code'],
      ['.cm-content', '.cm-content'], ['.cm-line', '.cm-line'], ['.cm-editor', '.cm-editor'],
      ['inline code', 'code:not(pre code)'],
      ['table', 'table'], ['.milkdown-table-block', '.milkdown-table-block'], ['.table-wrapper', '.table-wrapper'],
      ['th', 'th'], ['td', 'td'], ['th p', 'th p'], ['td p', 'td p'],
      ['img', 'img'],
      ['list label', '.milkdown-list-item-block .label-wrapper'],
      ['locked/math block', '[data-locked-block], [data-kind], .mgo-locked-block'],
      ['footnote ref', '.mgo-fnref'],
    ]
    const PROPS = [
      'font-family', 'font-size', 'font-weight', 'line-height', 'letter-spacing',
      'margin-top', 'margin-bottom',
      'padding-top', 'padding-right', 'padding-bottom', 'padding-left',
      'border-top-width', 'border-top-style', 'border-top-color',
      'border-right-width', 'border-right-style', 'border-right-color',
      'border-bottom-width', 'border-bottom-style', 'border-bottom-color',
      'border-left-width', 'border-left-style', 'border-left-color',
      'box-shadow', 'width', 'max-width', 'table-layout', 'overflow-x',
      'white-space', 'word-break', 'line-break',
      'text-autospace', 'text-spacing-trim', 'hanging-punctuation',
    ]
    const camel = (p) => p.replace(/-([a-z])/g, (_, c) => c.toUpperCase())
    const proto = document.documentElement.style
    const out = {}
    for (const [key, sel] of SPEC) {
      let el = null
      try { el = root.querySelector(sel) } catch (e) { /* selector unsupported */ }
      if (key === 'inline code') {
        const codes = root.querySelectorAll('code')
        el = [...codes].find((c) => !c.closest('pre') && !c.closest('.cm-editor')) || null
      }
      if (!el) { out[key] = { found: false, selector: sel }; continue }
      const cs = getComputedStyle(el)
      const styles = {}
      for (const p of PROPS) {
        const supported = camel(p) in proto
        let v = supported ? cs.getPropertyValue(p) : ''
        if (!supported || v === '') v = supported ? (cs.getPropertyValue(p) || 'normal/empty') : 'unsupported'
        styles[p] = v
      }
      out[key] = { found: true, selector: sel, dom: describeEl(el), styles }
    }
    return out
  })()`)
}

// ---------------------------------------------------------------------------
// B. block rhythm
// ---------------------------------------------------------------------------

async function measureRhythm(page) {
  return evalPage(page, `(() => {
    const root = pmRoot()
    const children = [...root.children]
    const pairs = []
    const meta = children.map((el) => ({ tag: el.tagName.toLowerCase(), class: String(el.className || '').trim() || null, kind: blockKind(el) }))
    for (let i = 0; i < children.length - 1; i++) {
      const a = children[i].getBoundingClientRect()
      const b = children[i + 1].getBoundingClientRect()
      pairs.push({ prev: meta[i], next: meta[i + 1], gapPx: +(b.top - a.bottom).toFixed(2) })
    }
    const groups = {}
    for (const p of pairs) {
      const key = p.prev.kind + ' → ' + p.next.kind
      ;(groups[key] = groups[key] || []).push(p.gapPx)
    }
    const summary = {}
    for (const [k, v] of Object.entries(groups)) summary[k] = summarize(v)
    return { topLevelChildren: meta, pairs, summary }
  })()`)
}

// ---------------------------------------------------------------------------
// C. measure (width / lines / chars per line)
// ---------------------------------------------------------------------------

async function measureText(page) {
  return evalPage(page, `(() => {
    const root = pmRoot()
    const paras = [...root.querySelectorAll(':scope > p')]
    const CJK = /[\\u3400-\\u4dbf\\u4e00-\\u9fff\\uf900-\\ufaff]/
    function counts(el) {
      const t = el.textContent || ''
      let cjk = 0, latin = 0
      for (const ch of t) { if (CJK.test(ch)) cjk++; else if (/[A-Za-z]/.test(ch)) latin++ }
      return { len: t.length, cjk, latin }
    }
    function measure(el) {
      const cs = getComputedStyle(el)
      const r = el.getBoundingClientRect()
      let lh = parseFloat(cs.lineHeight)
      const lhRaw = cs.lineHeight
      if (!Number.isFinite(lh)) lh = parseFloat(cs.fontSize) * 1.2
      const range = document.createRange()
      range.selectNodeContents(el)
      const lineRects = [...range.getClientRects()].filter((x) => x.height > 1)
      const linesByRect = lineRects.length
      const linesByLh = r.height / lh
      const { len } = counts(el)
      return {
        widthPx: +r.width.toFixed(2), heightPx: +r.height.toFixed(2),
        lineHeight: lhRaw, fontSize: cs.fontSize,
        linesByRect, linesByLh: +linesByLh.toFixed(2),
        chars: len, charsPerLine: +(len / Math.max(1, linesByRect)).toFixed(1),
        text: (el.textContent || '').slice(0, 80),
      }
    }
    const en = paras.find((p) => { const c = counts(p); return c.cjk === 0 && c.len >= 150 })
    const zh = paras.find((p) => { const c = counts(p); return c.cjk > 0 && c.cjk >= c.latin && c.len >= 100 })
    return { englishLong: en ? measure(en) : null, cjkDominant: zh ? measure(zh) : null }
  })()`)
}

// ---------------------------------------------------------------------------
// D. tables
// ---------------------------------------------------------------------------

async function measureTables(page) {
  return evalPage(page, `(() => {
    const root = pmRoot()
    const tables = visibleTables()
    return tables.map((t, idx) => {
      const rows = [...t.querySelectorAll('tr')]
      const cellCount = (tr) => tr.querySelectorAll('th,td').length
      const cols = Math.max(...rows.map(cellCount), 0)
      const tRect = t.getBoundingClientRect()
      const headerH = rows.length ? rows[0].getBoundingClientRect().height : null
      const bodyHeights = rows.slice(1).map((r) => +r.getBoundingClientRect().height.toFixed(2))
      const med = median(bodyHeights)
      const wrapper = t.closest('.table-wrapper') || t.closest('[class*="wrapper"]') || t.closest('.milkdown-table-block') || t.parentElement
      const wRect = wrapper.getBoundingClientRect()
      let clippedCells = 0, totalCells = 0
      for (const cell of t.querySelectorAll('th,td')) {
        totalCells++
        if (cell.scrollWidth > cell.clientWidth + 1) clippedCells++
      }
      const colWidths = rows.length ? [...rows[0].querySelectorAll('th,td')].map((c) => +c.getBoundingClientRect().width.toFixed(2)) : []
      // scroll test
      let scrollable = false, scrollLeftAfter = null
      try {
        wrapper.scrollLeft = 0
        wrapper.scrollLeft = 100
        scrollLeftAfter = wrapper.scrollLeft
        scrollable = wrapper.scrollLeft > 0
        wrapper.scrollLeft = 0
      } catch (e) { /* non-scrollable element */ }
      const block = t.closest('.milkdown-table-block')
      return {
        index: idx, rows: rows.length, cols,
        rect: { w: +tRect.width.toFixed(2), h: +tRect.height.toFixed(2) },
        headerRowHeight: headerH == null ? null : +headerH.toFixed(2),
        bodyRowHeights: bodyHeights, medianBodyRowHeight: med,
        rowsPer600px: med ? +(600 / med).toFixed(2) : null,
        wrapper: { ...describeEl(wrapper), clientWidth: wrapper.clientWidth },
        tableScrollWidth: t.scrollWidth, tableClientWidth: t.clientWidth,
        wrapperScrollWidth: wrapper.scrollWidth,
        clippedCells, totalCells,
        overflowing: wRect.width < tRect.width - 1 || wrapper.scrollWidth > wrapper.clientWidth + 1,
        colWidths,
        horizontallyScrollable: scrollable, scrollLeftAfter100: scrollLeftAfter,
        blockClass: block ? String(block.className) : null,
      }
    })
  })()`)
}

// ---------------------------------------------------------------------------
// F. screenshots
// ---------------------------------------------------------------------------

async function takeScreenshots(page, outDir, docName) {
  const shotDir = path.join(outDir, 'shots', docName)
  fs.mkdirSync(shotDir, { recursive: true })
  const files = []
  const shot = async (name, fn) => {
    const p = path.join(shotDir, name)
    try { await fn(p); files.push(p) } catch (e) { files.push(`${name} FAILED: ${e.message}`) }
  }

  const scrollerSel = await evalPage(page, `(() => { const s = scrollViewport(); s.scrollTop = 0; return true })()`)
  void scrollerSel
  await page.waitForTimeout(250)
  await shot('viewport-top.png', (p) => page.screenshot({ path: p }))

  const nTables = await evalPage(page, `(() => visibleTables().length)()`)
  for (let i = 0; i < nTables; i++) {
    await shot(`table-${i}.png`, async (p) => {
      const handle = await page.evaluateHandle(`(() => { ${PAGE_HELPERS}; return visibleTables()[${i}] })()`)
      const el = handle.asElement()
      await el.scrollIntoViewIfNeeded()
      await el.screenshot({ path: p })
    })
    await shot(`table-${i}-context.png`, async (p) => {
      const clip = await evalPage(page, `(() => {
        const t = visibleTables()[${i}]
        t.scrollIntoView({ block: 'center' })
        const r = t.getBoundingClientRect()
        const pad = 40
        return {
          x: Math.max(0, r.left - pad), y: Math.max(0, r.top - pad),
          width: Math.min(innerWidth, r.right + pad) - Math.max(0, r.left - pad),
          height: Math.min(innerHeight, r.bottom + pad) - Math.max(0, r.top - pad),
        }
      })()`)
      await page.waitForTimeout(120)
      await page.screenshot({ path: p, clip })
    })
  }

  await shot('list.png', async (p) => {
    const clip = await evalPage(page, `(() => {
      const el = pmRoot().querySelector('ul, ol')
      if (!el) return null
      el.scrollIntoView({ block: 'center' })
      const r = el.getBoundingClientRect()
      const pad = 40
      return { x: Math.max(0, r.left - pad), y: Math.max(0, r.top - pad),
        width: Math.min(innerWidth, r.right + pad) - Math.max(0, r.left - pad),
        height: Math.min(innerHeight, r.bottom + pad) - Math.max(0, r.top - pad) }
    })()`)
    if (!clip) throw new Error('no list')
    await page.waitForTimeout(120)
    await page.screenshot({ path: p, clip })
  })
  await shot('blockquote.png', async (p) => {
    const clip = await evalPage(page, `(() => {
      const el = pmRoot().querySelector('blockquote')
      if (!el) return null
      el.scrollIntoView({ block: 'center' })
      const r = el.getBoundingClientRect()
      const pad = 40
      return { x: Math.max(0, r.left - pad), y: Math.max(0, r.top - pad),
        width: Math.min(innerWidth, r.right + pad) - Math.max(0, r.left - pad),
        height: Math.min(innerHeight, r.bottom + pad) - Math.max(0, r.top - pad) }
    })()`)
    if (!clip) throw new Error('no blockquote')
    await page.waitForTimeout(120)
    await page.screenshot({ path: p, clip })
  })
  await shot('code-block.png', async (p) => {
    const loc = page.locator('.crepe-root .ProseMirror .milkdown-code-block').first()
    await loc.scrollIntoViewIfNeeded()
    await page.waitForTimeout(300)
    await loc.screenshot({ path: p })
  })
  await shot('math-block.png', async (p) => {
    const loc = page.locator('.crepe-root .ProseMirror [data-kind="math"]').first()
    await loc.scrollIntoViewIfNeeded()
    await loc.screenshot({ path: p })
  })
  if (docName === 'tech-report-tables') {
    await shot('table-T3-viewport.png', async (p) => {
      await evalPage(page, `(() => { const t = visibleTables()[2]; t.scrollIntoView({ block: 'center' }); return true })()`)
      await page.waitForTimeout(150)
      await page.screenshot({ path: p })
    })
  }
  return files
}

// ---------------------------------------------------------------------------
// Shared probe utilities
// ---------------------------------------------------------------------------

async function readMarkdownNow(page) {
  await page.waitForTimeout(450) // > App's 250ms host.update debounce
  return evalPage(page, `readMarkdown()`)
}

function firstDiff(a, b) {
  const n = Math.min(a.length, b.length)
  for (let i = 0; i < n; i++) {
    if (a[i] !== b[i]) {
      return { at: i, a: JSON.stringify(a.slice(Math.max(0, i - 40), i + 40)), b: JSON.stringify(b.slice(Math.max(0, i - 40), i + 40)) }
    }
  }
  if (a.length !== b.length) return { at: n, a: `len ${a.length}`, b: `len ${b.length}` }
  return null
}

// Click at the centre of the first non-empty text node's rect inside the
// element produced by `expr` (a JS expression evaluated in the page).
async function clickInText(page, expr) {
  const pt = await evalPage(page, `(() => {
    const el = (${expr})
    if (!el || !el.getBoundingClientRect) return null
    el.scrollIntoView({ block: 'center' })
    const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT)
    let node = walker.nextNode()
    while (node && !node.textContent.trim()) node = walker.nextNode()
    let r
    if (node) {
      const range = document.createRange()
      const off = Math.min(3, Math.max(0, node.textContent.length - 1))
      range.setStart(node, off)
      range.setEnd(node, Math.min(off + 1, node.textContent.length))
      r = range.getBoundingClientRect()
      if (!r.width) r = el.getBoundingClientRect()
    } else r = el.getBoundingClientRect()
    return { x: r.left + r.width / 2, y: r.top + r.height / 2 }
  })()`)
  if (!pt) return null
  if (pt.y < 0 || pt.y > VIEWPORT.height || pt.x < 0 || pt.x > VIEWPORT.width) return null
  await page.mouse.click(pt.x, pt.y)
  return pt
}

async function selectionInfo(page) {
  return evalPage(page, `(() => {
    const view = pmView()
    const domSel = window.getSelection()
    const dom = domSel.rangeCount ? { anchorNode: domSel.anchorNode && domSel.anchorNode.nodeName, anchorOffset: domSel.anchorOffset, focusOffset: domSel.focusOffset, collapsed: domSel.isCollapsed } : null
    if (!view) return { viewFound: false, domSelection: dom, selectedNodeClass: !!document.querySelector('.ProseMirror-selectednode') }
    const sel = view.state.selection
    return {
      viewFound: true,
      selectionClass: sel.constructor.name,
      from: sel.from, to: sel.to, empty: sel.empty,
      domSelection: dom,
      selectedNodeClass: !!document.querySelector('.ProseMirror-selectednode'),
    }
  })()`)
}

// ---------------------------------------------------------------------------
// G1. table click -> caret (tech-report)
// ---------------------------------------------------------------------------

async function probeTableClick(page, cellDescr, cellExpr) {
  const res = { target: cellDescr, cellExpr }
  const before = await readMarkdownNow(page)
  res.markdownBeforeLen = before.length
  let clicks = 0, info = null
  for (; clicks < 3; clicks++) {
    const pt = await clickInText(page, cellExpr)
    if (!pt) { res.error = 'no such cell'; return res }
    res['click' + (clicks + 1) + 'Point'] = pt
    await page.waitForTimeout(300)
    info = await selectionInfo(page)
    if (info && info.empty) break
  }
  res.clicksForCollapsedCaret = clicks + 1
  res.afterClick = info
  await page.keyboard.type('X')
  await page.waitForTimeout(150)
  const afterType = await selectionInfo(page)
  res.afterType = afterType
  const mdAfterType = await readMarkdownNow(page)
  res.typeDiff = firstDiff(before, mdAfterType)
  res.lengthDelta = mdAfterType.length - before.length
  // locate the typed 'X': positions where mdAfterType has X that before lacks
  const xPositions = []
  for (let i = 0; i < mdAfterType.length; i++) {
    if (mdAfterType[i] === 'X' && before[i] !== 'X') xPositions.push(i)
  }
  res.xInsertedAt = xPositions
  res.xContext = xPositions.length ? JSON.stringify(mdAfterType.slice(Math.max(0, xPositions[0] - 30), xPositions[0] + 30)) : null
  await page.keyboard.press('Meta+z')
  await page.waitForTimeout(300)
  const mdAfterUndo = await readMarkdownNow(page)
  res.byteIdenticalAfterUndo = mdAfterUndo === before
  if (!res.byteIdenticalAfterUndo) res.undoDiff = firstDiff(before, mdAfterUndo)
  return res
}

// ---------------------------------------------------------------------------
// G2. toolbar during drag (paper-bilingual)
// ---------------------------------------------------------------------------

async function probeToolbarDrag(page, shotDir) {
  const res = { shotDir: shotDir || null }
  // choose a mid-document paragraph with a decent length
  const target = await evalPage(page, `(() => {
    const paras = [...pmRoot().querySelectorAll(':scope > p')].filter((p) => (p.textContent || '').length > 200)
    const el = paras[Math.floor(paras.length / 2)] || paras[0]
    if (!el) return null
    el.scrollIntoView({ block: 'center' })
    const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT)
    let node = walker.nextNode()
    while (node && node.textContent.trim().length < 8) node = walker.nextNode()
    const range = document.createRange()
    const start = Math.min(10, node.textContent.length - 2)
    range.setStart(node, start); range.setEnd(node, start + 1)
    const r = range.getBoundingClientRect()
    return { x: r.left + r.width / 2, y: r.top + r.height / 2, paraText: el.textContent.slice(0, 60) }
  })()`)
  if (!target) return { error: 'no paragraph' }
  res.target = target
  await evalPage(page, `(() => {
    window.__samples = []
    window.__sampleStop = false
    window.__marks = {}
    const loop = (t) => {
      const el = document.querySelector('.milkdown-toolbar')
      const r = el ? el.getBoundingClientRect() : null
      window.__samples.push({
        t,
        show: el ? (el.dataset.show !== undefined ? el.dataset.show : (el.offsetParent !== null)) : null,
        vis: el ? getComputedStyle(el).visibility : null,
        ptr: !!document.querySelector('.mgo-wysiwyg-root.mgo-pointer-selecting'),
        rect: r ? { x: +r.x.toFixed(1), y: +r.y.toFixed(1), w: +r.width.toFixed(1), h: +r.height.toFixed(1) } : null,
      })
      if (!window.__sampleStop) requestAnimationFrame(loop)
    }
    requestAnimationFrame(loop)
    return true
  })()`)
  await page.mouse.move(target.x, target.y)
  await evalPage(page, `(() => { window.__marks.dragStart = performance.now(); window.__samples.push({ mark: 'down' }); return true })()`)
  await page.mouse.down()
  for (let i = 1; i <= 10; i++) {
    await page.mouse.move(target.x + i * 20, target.y + i * 0.8, { steps: 1 })
    await page.waitForTimeout(30)
    if (i === 5 && res.shotDir) await page.screenshot({ path: path.join(res.shotDir, 'toolbar-during-drag.png') }).catch(() => {})
  }
  await evalPage(page, `(() => { window.__marks.dragEnd = performance.now(); window.__samples.push({ mark: 'up-before' }); return true })()`)
  await page.mouse.up()
  await page.waitForTimeout(150)
  if (res.shotDir) await page.screenshot({ path: path.join(res.shotDir, 'toolbar-after-drag.png') }).catch(() => {})
  await evalPage(page, `(() => { window.__samples.push({ mark: 'up' }); return true })()`)
  await page.waitForTimeout(300)
  res.viewSelectionAfterDrag = await selectionInfo(page)
  const data = await evalPage(page, `(() => {
    window.__sampleStop = true
    const sel = window.getSelection()
    let selRect = null
    if (sel.rangeCount) {
      const rects = [...sel.getRangeAt(0).getClientRects()]
      selRect = rects.length ? { first: { x: rects[0].x, y: rects[0].y, w: rects[0].width, h: rects[0].height }, last: { x: rects[rects.length-1].x, y: rects[rects.length-1].y, w: rects[rects.length-1].width, h: rects[rects.length-1].height }, n: rects.length } : null
    }
    const tb = document.querySelector('.milkdown-toolbar')
    const items = tb ? [...tb.querySelectorAll('button, [role="button"], li')] : []
    const itemRects = items.slice(0, 12).map((b) => { const r = b.getBoundingClientRect(); return { tag: b.tagName, label: b.getAttribute('aria-label') || b.getAttribute('title') || b.textContent.trim().slice(0, 20) || null, w: +r.width.toFixed(1), h: +r.height.toFixed(1) } })
    return { marks: window.__marks, samples: window.__samples, selRect, toolbarItems: itemRects, toolbarHtmlSnippet: tb ? tb.outerHTML.slice(0, 400) : null }
  })()`)
  res.samples = data.samples.length
  const upT = data.marks.dragEnd
  const downT = data.marks.dragStart
  const during = data.samples.filter((s) => s.t !== undefined && s.t >= downT && s.t <= upT && s.show === 'true')
  const after = data.samples.filter((s) => s.t !== undefined && s.t > upT && s.show === 'true')
  res.firstShowDuringDrag = during.length ? during[0].t - downT : null
  res.firstShowAfterMouseup = after.length ? after[0].t - upT : null
  // visible = data-show true AND computed visibility not hidden (the
  // mgo-pointer-selecting class hides the toolbar while a drag is in flight)
  const visible = (s) => s.show === 'true' && s.vis !== 'hidden'
  const duringAll = data.samples.filter((s) => s.t !== undefined && s.t >= downT && s.t <= upT)
  const afterAll = data.samples.filter((s) => s.t !== undefined && s.t > upT)
  const visDuring = duringAll.filter(visible)
  const visAfter = afterAll.filter(visible)
  res.firstVisibleDuringDrag = visDuring.length ? visDuring[0].t - downT : null
  res.firstVisibleAfterMouseup = visAfter.length ? visAfter[0].t - upT : null
  res.pointerSelectingSamplesDuringDrag = duringAll.filter((s) => s.ptr).length
  res.dragSamples = duringAll.length
  res.hiddenWhileSelecting = visDuring.length === 0 && res.pointerSelectingSamplesDuringDrag > 0
  const pos = new Set(during.map((s) => s.rect && `${Math.round(s.rect.x)},${Math.round(s.rect.y)}`))
  res.distinctPositionsDuringDrag = pos.size
  const last = [...data.samples].reverse().find((s) => s.rect)
  res.finalRect = last ? last.rect : null
  res.selectionRect = data.selRect
  res.toolbarItems = data.toolbarItems
  res.toolbarHtmlSnippet = data.toolbarHtmlSnippet
  // overlap: toolbar vs the text line above the selection start
  res.overlap = await evalPage(page, `(() => {
    const tb = document.querySelector('.milkdown-toolbar')
    if (!tb) return { visible: false }
    const tr = tb.getBoundingClientRect()
    const sel = window.getSelection()
    if (!sel.rangeCount) return { visible: true, note: 'no selection' }
    const rects = [...sel.getRangeAt(0).getClientRects()]
    if (!rects.length) return { visible: true, note: 'no selection rects' }
    const first = rects[0]
    const node = sel.getRangeAt(0).startContainer
    const el = node.nodeType === 1 ? node : node.parentElement
    const lh = parseFloat(getComputedStyle(el).lineHeight) || first.height
    const above = { top: first.top - lh, bottom: first.top }
    const px = Math.min(tr.bottom, above.bottom) - Math.max(tr.top, above.top)
    return { visible: true, overlapsLineAbove: px > 0, overlapPx: +px.toFixed(1), lineHeightUsed: lh }
  })()`)
  delete res.shotDir
  return res
}

// ---------------------------------------------------------------------------
// G3. toolbar focus (paper-bilingual)
// ---------------------------------------------------------------------------

async function probeToolbarFocus(page) {
  const res = {}
  const mdBefore = await readMarkdownNow(page)
  // Create a real selection first: double-click selects a word, then extend.
  const wordPt = await evalPage(page, `(() => {
    const p = [...pmRoot().querySelectorAll(':scope > p')].find((x) => (x.textContent || '').length > 120)
    if (!p) return null
    p.scrollIntoView({ block: 'center' })
    const walker = document.createTreeWalker(p, NodeFilter.SHOW_TEXT)
    const node = walker.nextNode()
    const range = document.createRange()
    range.setStart(node, 2); range.setEnd(node, 3)
    const r = range.getBoundingClientRect()
    return { x: r.left + r.width / 2, y: r.top + r.height / 2 }
  })()`)
  if (!wordPt) return { error: 'no paragraph for selection' }
  await page.mouse.dblclick(wordPt.x, wordPt.y)
  await page.waitForTimeout(250)
  for (let i = 0; i < 6; i++) await page.keyboard.press('Shift+ArrowRight')
  await page.waitForTimeout(350)
  res.pre = await evalPage(page, `(() => {
    const view = pmView()
    const sel = view.state.selection
    const tb = document.querySelector('.milkdown-toolbar')
    const btns = tb ? [...tb.querySelectorAll('button, [role="button"], li')].map((b) => ({ tag: b.tagName, label: b.getAttribute('aria-label') || b.getAttribute('title') || (b.textContent || '').trim().slice(0, 30) || null })) : []
    return {
      activeElement: describeEl(document.activeElement),
      selection: { class: sel.constructor.name, from: sel.from, to: sel.to, empty: sel.empty },
      toolbarShow: tb ? tb.dataset.show ?? null : null,
      buttons: btns,
    }
  })()`)
  // click the bold button (first icon matching bold/strong, else first button)
  const boldPt = await evalPage(page, `(() => {
    const tb = document.querySelector('.milkdown-toolbar')
    if (!tb) return null
    const items = [...tb.querySelectorAll('button, [role="button"], li')]
    let el = items.find((b) => /bold|strong/i.test((b.getAttribute('aria-label') || '') + (b.getAttribute('title') || '') + (b.textContent || '')))
    if (!el) el = items[0]
    if (!el) return null
    const r = el.getBoundingClientRect()
    return { x: r.left + r.width / 2, y: r.top + r.height / 2 }
  })()`)
  res.boldButtonPoint = boldPt
  if (!boldPt) { res.error = 'no toolbar button'; return res }
  await page.mouse.click(boldPt.x, boldPt.y)
  await page.waitForTimeout(300)
  res.post = await evalPage(page, `(() => {
    const view = pmView()
    const sel = view.state.selection
    const tb = document.querySelector('.milkdown-toolbar')
    return {
      activeElement: describeEl(document.activeElement),
      activeIsProseMirror: document.activeElement === pmRoot() || (document.activeElement && document.activeElement.closest && !!document.activeElement.closest('.ProseMirror')),
      selection: { class: sel.constructor.name, from: sel.from, to: sel.to, empty: sel.empty },
      toolbarShow: tb ? tb.dataset.show ?? null : null,
    }
  })()`)
  const mdAfter = await readMarkdownNow(page)
  res.markdownDiff = firstDiff(mdBefore, mdAfter)
  await page.keyboard.press('Meta+z')
  await page.waitForTimeout(300)
  const mdUndo = await readMarkdownNow(page)
  res.byteIdenticalAfterUndo = mdUndo === mdBefore
  return res
}

// ---------------------------------------------------------------------------
// G4. table handles (tech-report T2)
// ---------------------------------------------------------------------------

async function probeTableHandles(page) {
  const res = { phases: [] }
  const positions = await evalPage(page, `(() => {
    const t = visibleTables()[1]
    if (!t) return null
    t.scrollIntoView({ block: 'center' })
    const cell = t.querySelectorAll('tr')[4].querySelectorAll('td,th')[2]
    const r = cell.getBoundingClientRect()
    return {
      interior: { x: r.left + r.width / 2, y: r.top + r.height / 2 },
      rightBorder: { x: r.right - 2, y: r.top + r.height / 2 },
      bottomBorder: { x: r.left + r.width / 2, y: r.bottom - 2 },
    }
  })()`)
  if (!positions) return { error: 'T2 not found' }
  for (const phase of ['interior', 'rightBorder', 'bottomBorder']) {
    const p = positions[phase]
    const t0 = Date.now()
    await page.mouse.move(p.x, p.y, { steps: 2 })
    let seen = null, showMs = null
    for (let i = 0; i < 40; i++) {
      seen = await evalPage(page, `(() => {
        const hs = [...document.querySelectorAll('.crepe-root [class*="handle"], .crepe-root [data-role], .crepe-root [data-show]')]
          .map((h) => {
            const r = h.getBoundingClientRect()
            return {
              class: String(h.className || '').slice(0, 60), role: h.getAttribute('data-role'),
              show: h.getAttribute('data-show'), visible: r.width > 0 && r.height > 0,
              rect: { x: +r.x.toFixed(1), y: +r.y.toFixed(1), w: +r.width.toFixed(1), h: +r.height.toFixed(1) },
            }
          }).filter((h) => h.visible)
        return hs
      })()`)
      const showing = seen.filter((h) => h.show === 'true' || h.show === '')
      if (showing.length && showMs === null) showMs = Date.now() - t0
      if (showing.length && i > 6) break
      await page.waitForTimeout(25)
    }
    res.phases.push({ phase, point: p, msToShow: showMs, handles: seen, shownHandles: seen.filter((h) => h.show === 'true') })
  }
  return res
}

// ---------------------------------------------------------------------------
// G5. layout shift on focus
// ---------------------------------------------------------------------------

async function probeLayoutShift(page, label, expr) {
  const res = { target: label, expr }
  const setup = await evalPage(page, `(() => {
    const el = (${expr})
    if (!el) return null
    el.scrollIntoView({ block: 'center' })
    const s = scrollViewport()
    return {
      docScrollHeight: document.documentElement.scrollHeight,
      scrollerScrollHeight: s.scrollHeight,
      scrollerScrollTop: s.scrollTop,
      blockHeight: el.getBoundingClientRect().height,
    }
  })()`)
  if (!setup) return { target: label, error: 'element not found' }
  res.before = setup
  const pt = await clickInText(page, expr)
  if (!pt) {
    const c = await evalPage(page, `(() => { const el = (${expr}); const r = el.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2 } })()`)
    await page.mouse.click(c.x, c.y)
  }
  await page.waitForTimeout(350)
  res.afterClickInside = await evalPage(page, `(() => {
    const el = (${expr})
    const s = scrollViewport()
    return {
      docScrollHeight: document.documentElement.scrollHeight,
      scrollerScrollHeight: s.scrollHeight,
      scrollerScrollTop: s.scrollTop,
      blockHeight: el ? el.getBoundingClientRect().height : null,
    }
  })()`)
  // click back into a paragraph
  const pPt = await evalPage(page, `(() => {
    const p = pmRoot().querySelector(':scope > p')
    if (!p) return null
    const r = p.getBoundingClientRect()
    return { x: r.left + Math.min(80, r.width / 2), y: r.top + r.height / 2 }
  })()`)
  if (pPt) await page.mouse.click(pPt.x, pPt.y)
  await page.waitForTimeout(350)
  res.afterClickParagraph = await evalPage(page, `(() => {
    const el = (${expr})
    const s = scrollViewport()
    return {
      docScrollHeight: document.documentElement.scrollHeight,
      scrollerScrollHeight: s.scrollHeight,
      scrollerScrollTop: s.scrollTop,
      blockHeight: el ? el.getBoundingClientRect().height : null,
    }
  })()`)
  res.delta = {
    docScrollHeight: res.afterClickParagraph.docScrollHeight - res.before.docScrollHeight,
    scrollerScrollHeight: res.afterClickParagraph.scrollerScrollHeight - res.before.scrollerScrollHeight,
    blockHeight: +(res.afterClickParagraph.blockHeight - res.before.blockHeight).toFixed(2),
    scrollTopClickIn: +(res.afterClickInside.scrollerScrollTop - res.before.scrollerScrollTop).toFixed(2),
    scrollTopClickOut: +(res.afterClickParagraph.scrollerScrollTop - res.afterClickInside.scrollerScrollTop).toFixed(2),
  }
  return res
}

// ---------------------------------------------------------------------------
// G6. IME (Chromium only)
// ---------------------------------------------------------------------------

async function probeIME(page, label, expr) {
  const res = { target: label }
  const mdBefore = await readMarkdownNow(page)
  const pt = await clickInText(page, expr)
  if (!pt) return { target: label, error: 'no click target' }
  await page.waitForTimeout(250)
  await evalPage(page, `(() => {
    window.__imeSamples = []
    window.__imeStop = false
    const loop = () => {
      const tb = document.querySelector('.milkdown-toolbar')
      window.__imeSamples.push(tb ? (tb.dataset.show ?? null) : null)
      if (!window.__imeStop) requestAnimationFrame(loop)
    }
    requestAnimationFrame(loop)
    return true
  })()`)
  const cdp = await page.context().newCDPSession(page)
  try {
    await cdp.send('Input.imeSetComposition', { text: 'zhongwen', selectionStart: 8, selectionEnd: 8 })
    await page.waitForTimeout(120)
    await cdp.send('Input.insertText', { text: '中文' })
    await page.waitForTimeout(300)
    res.compositionOk = true
  } catch (e) {
    res.compositionOk = false
    res.error = String(e.message || e)
  }
  const flash = await evalPage(page, `(() => {
    window.__imeStop = true
    return { anyToolbarShow: window.__imeSamples.some((s) => s === 'true'), samples: window.__imeSamples.length }
  })()`)
  res.toolbarFlashedDuringComposition = flash.anyToolbarShow
  res.toolbarSamples = flash.samples
  const mdAfter = await readMarkdownNow(page)
  res.markdownDiff = firstDiff(mdBefore, mdAfter)
  await page.keyboard.press('Meta+z')
  await page.waitForTimeout(300)
  const mdUndo = await readMarkdownNow(page)
  res.oneUndoRemovesAll = mdUndo === mdBefore
  if (!res.oneUndoRemovesAll) res.undoDiff = firstDiff(mdBefore, mdUndo)
  return res
}

// ---------------------------------------------------------------------------
// per-document run
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Variant probes (V2 rules / V3 dense): scrolled handles, add buttons, caret,
// row hit height — run on tech-report-tables only.
// ---------------------------------------------------------------------------

async function probeVariantTable(page) {
  const res = {}
  const T2 = `visibleTables()[1]`
  const T3 = `visibleTables()[2]`
  // (a) scroll the wide T3 wrapper 200px, hover a cell, compare handle vs cell rect
  res.scrollLeftSet = await evalPage(page, `(() => {
    const w = ${T3}.closest('.table-wrapper')
    if (!w) return null
    w.scrollLeft = 0
    w.scrollLeft = 200
    return w.scrollLeft
  })()`)
  const hoverPt = await evalPage(page, `(() => {
    const c = ${T3}.querySelectorAll('tr')[3].querySelectorAll('td,th')[4]
    if (!c) return null
    // vertical scroll only: bring the cell into the viewport without
    // touching the wrapper's scrollLeft
    const scroller = scrollViewport()
    const r0 = c.getBoundingClientRect()
    scroller.scrollTop += r0.top + r0.height / 2 - innerHeight / 2
    const w = ${T3}.closest('.table-wrapper')
    const wr = w.getBoundingClientRect()
    const r = c.getBoundingClientRect()
    const x = Math.min(Math.max(r.left + r.width / 2, wr.left + 8), wr.right - 8)
    if (x < r.left || x > r.right) return null
    return { x, y: r.top + r.height / 2 }
  })()`)
  if (hoverPt) {
    await page.mouse.move(hoverPt.x, hoverPt.y, { steps: 2 })
    await page.waitForTimeout(300)
  }
  res.handlesAfterScroll = await evalPage(page, `(() => {
    const c = ${T3}.querySelectorAll('tr')[3].querySelectorAll('td,th')[4]
    const cr = c ? c.getBoundingClientRect() : null
    const shown = [...document.querySelectorAll('.handle[data-show="true"]')].map((h) => {
      const r = h.getBoundingClientRect()
      return { role: h.getAttribute('data-role'), x: +r.x.toFixed(1), y: +r.y.toFixed(1), w: +r.width.toFixed(1), h: +r.height.toFixed(1) }
    })
    return { cell: cr ? { x: +cr.x.toFixed(1), y: +cr.y.toFixed(1), w: +cr.width.toFixed(1), h: +cr.height.toFixed(1) } : null, shown }
  })()`)
  // (b) add-row / add-column buttons visible and not clipped
  // show the line handles first by hovering the table's bottom/right edges
  const edge = await evalPage(page, `(() => {
    const t = ${T3}
    if (!t) return null
    const r = t.getBoundingClientRect()
    return { x: r.left + r.width / 2, y: r.bottom - 2 }
  })()`)
  if (edge) {
    await page.mouse.move(edge.x, edge.y, { steps: 2 })
    await page.waitForTimeout(300)
  }
  res.addButtons = await evalPage(page, `(() => {
    return [...document.querySelectorAll('.handle.line-handle .add-button, .handle.line-handle')].map((h) => {
      const r = h.getBoundingClientRect()
      const cx = r.left + r.width / 2
      const cy = r.top + r.height / 2
      const hit = document.elementFromPoint(cx, cy)
      const isButton = h.classList.contains('add-button')
      return {
        role: h.getAttribute('data-role') || (isButton ? 'add-button' : null),
        cls: String(h.className).slice(0, 50),
        show: h.getAttribute('data-show'),
        rect: { x: +r.x.toFixed(1), y: +r.y.toFixed(1), w: +r.width.toFixed(1), h: +r.height.toFixed(1) },
        inViewport: r.left >= 0 && r.top >= 0 && r.right <= innerWidth && r.bottom <= innerHeight,
        hitAtCenter: hit ? (hit === h || h.contains(hit)) : null,
      }
    })
  })()`)
  // (c) single click -> caret in a cell scrolled into view
  const cellExpr = `${T2}.querySelectorAll('tr')[4].querySelectorAll('td,th')[1]`
  const pt = await clickInText(page, cellExpr)
  res.singleClickPoint = pt
  await page.waitForTimeout(300)
  res.singleClickSelection = await selectionInfo(page)
  // (d) hit height of a one-line body row
  res.oneLineRowHeight = await evalPage(page, `(() => {
    const rows = [...${T2}.querySelectorAll('tr')].slice(1)
    const heights = rows.map((r) => r.getBoundingClientRect().height)
    return { heights, min: Math.min(...heights), firstRowHeight: heights[0] ?? null }
  })()`)
  return res
}

// Separate page load with IntersectionObserver disabled so the lazy
// CodeMirror mount never happens: lets us photograph the placeholder state.
async function takeCodePlaceholderShot(context, url, outDir, docName) {
  const p = path.join(outDir, 'shots', docName, 'code-block-placeholder.png')
  const page = await context.newPage()
  try {
    await page.addInitScript(`(() => {
      window.IntersectionObserver = class {
        constructor() {}
        observe() {}
        unobserve() {}
        disconnect() {}
        takeRecords() { return [] }
      }
    })()`)
    await page.goto(url, { waitUntil: 'load' })
    await page.waitForSelector('.ProseMirror', { timeout: 30000 })
    await page.evaluate(() => document.fonts.ready)
    await page.waitForTimeout(400)
    const loc = page.locator('.crepe-root .ProseMirror .milkdown-code-block-placeholder').first()
    if ((await loc.count()) === 0) throw new Error('no placeholder')
    await loc.scrollIntoViewIfNeeded()
    await page.waitForTimeout(120)
    await loc.screenshot({ path: p })
    return p
  } catch (e) {
    return `code-block-placeholder.png FAILED: ${e.message}`
  } finally {
    await page.close()
  }
}

// E3b: scroll a mounted code block >200px above the viewport, wait past
// Crepe's teardown delay, and confirm scrollTop + a reference paragraph's
// viewport position do not move when the block collapses to a placeholder.
async function probeCodeTeardown(page) {
  const res = {}
  await evalPage(page, `(() => {
    const block = pmRoot().querySelector('.milkdown-code-block')
    block.scrollIntoView({ block: 'center' })
    return true
  })()`)
  await page.waitForTimeout(800) // let CodeMirror mount
  const before = await evalPage(page, `(() => {
    const block = pmRoot().querySelector('.milkdown-code-block')
    const mounted = !!block.querySelector('.cm-editor')
    // reference = last top-level child that is not the code block itself;
    // tracked by index because teardown can re-render sibling DOM
    const kids = [...pmRoot().children]
    let refIndex = kids.length - 1
    while (refIndex > 0 && kids[refIndex].classList.contains('milkdown-code-block')) refIndex--
    const ref = kids[refIndex]
    ref.scrollIntoView({ block: 'center' })
    const scroller = scrollViewport()
    // make sure the block is fully above the viewport incl. its IO margin
    const need = block.getBoundingClientRect().bottom + 260
    if (need > 0) scroller.scrollTop += need
    return { mounted, refIndex, refTag: ref.tagName, refTop: ref.getBoundingClientRect().top, scrollTop: scroller.scrollTop, blockH: block.getBoundingClientRect().height }
  })()`)
  Object.assign(res, { before })
  await page.waitForTimeout(5600) // teardown delay (5s) + margin
  const after = await evalPage(page, `(() => {
    const block = pmRoot().querySelector('.milkdown-code-block')
    const ref = pmRoot().children[${before.refIndex}]
    const scroller = scrollViewport()
    return { placeholder: !!block.querySelector('.milkdown-code-block-placeholder'), mounted: !!block.querySelector('.cm-editor'),
      refTop: ref ? ref.getBoundingClientRect().top : null, scrollTop: scroller.scrollTop, blockH: block.getBoundingClientRect().height }
  })()`)
  Object.assign(res, { after })
  res.refTopDelta = after.refTop == null ? null : +(after.refTop - before.refTop).toFixed(2)
  res.scrollTopDelta = +(after.scrollTop - before.scrollTop).toFixed(2)
  res.blockHeightDelta = +(after.blockH - before.blockH).toFixed(2)
  return res
}

async function runDoc(page, context, docName, outDir, engineName, engineKey, variant) {
  const result = { doc: docName, variant: variant || null }
  const url = variant ? `${BENCH_PAGE}?doc=${docName}&variant=${variant}` : `${BENCH_PAGE}?doc=${docName}`
  await page.goto(url, { waitUntil: 'load' })
  await page.waitForSelector('.ProseMirror', { timeout: 30000 })
  await page.evaluate(() => document.fonts.ready)
  await page.waitForTimeout(500)

  // G7 round-trip (before any mutation)
  const seeded = await evalPage(page, `window.__bench.fixture`)
  const rt = await readMarkdownNow(page)
  result.roundTrip = { equal: rt === seeded, diff: rt === seeded ? null : firstDiff(seeded, rt) }

  result.computedStyles = await measureStyles(page)
  result.blockRhythm = await measureRhythm(page)
  result.measure = await measureText(page)
  result.tables = await measureTables(page)
  result.proseMirrorScrollHeight = await evalPage(page, `pmRoot().scrollHeight`)
  result.screenshots = await takeScreenshots(page, outDir, docName)
  if (await evalPage(page, `!!pmRoot().querySelector('.milkdown-code-block')`)) {
    result.screenshots.push(await takeCodePlaceholderShot(context, url, outDir, docName))
  }

  result.probes = {}
  const probe = async (name, fn) => {
    try { result.probes[name] = await fn() }
    catch (e) { result.probes[name] = { error: String(e && e.message || e) } }
  }

  if (docName === 'tech-report-tables') {
    // T2 = tables[1] (6 cols x 12 body rows). Body row 3 = tr index 3
    // (tr[0] is the header row), col 2 = cell index 1.
    const T2 = `visibleTables()[1]`
    const T2cell = `${T2}.querySelectorAll('tr')[3].querySelectorAll('td,th')[1]`
    // T4 = tables[3] (3 cols, prose column); pick body row 2, prose column cell.
    const T4cell = `visibleTables()[3].querySelectorAll('tr')[2].querySelectorAll('td,th')[2]`
    await probe('tableClick-T2-r3c2', () => probeTableClick(page, 'T2 body-row-3 col-2', T2cell))
    await probe('tableClick-T4-prose', () => probeTableClick(page, 'T4 body-row-2 prose-col', T4cell))
    await probe('tableHandles-T2', () => probeTableHandles(page))
    await probe('layoutShift-code', () => probeLayoutShift(page, 'first code block', `pmRoot().querySelector('.cm-editor') || pmRoot().querySelector('pre')`))
    await probe('codeTeardown', () => probeCodeTeardown(page))
    await probe('layoutShift-T2', () => probeLayoutShift(page, 'T2 table', `${T2}`))
    if (variant === 'rules' || variant === 'dense') {
      await probe('variantTable', () => probeVariantTable(page))
    }
    if (engineKey === 'chromium') {
      await probe('ime-T4-cell', () => probeIME(page, 'T4 prose cell', T4cell))
    } else {
      result.probes['ime-T4-cell'] = { blocked: 'IME composition requires Chromium CDP (Input.imeSetComposition); not available on WebKit' }
    }
  }
  if (docName === 'paper-bilingual') {
    await probe('toolbarDrag', () => probeToolbarDrag(page, path.join(outDir, 'shots', docName)))
    await probe('toolbarFocus', () => probeToolbarFocus(page))
    await probe('layoutShift-math', () => probeLayoutShift(page, 'first display-math/locked block', `pmRoot().querySelector('[data-kind="math"]') || pmRoot().querySelector('[data-locked-block]')`))
    if (engineKey === 'chromium') {
      await probe('ime-paragraph', () => probeIME(page, 'body paragraph', `pmRoot().querySelector(':scope > p')`))
    } else {
      result.probes['ime-paragraph'] = { blocked: 'IME composition requires Chromium CDP (Input.imeSetComposition); not available on WebKit' }
    }
  }
  void engineName
  return result
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

async function main() {
  const args = parseArgs(process.argv)
  const outRoot = args.variant
    ? path.join(HERE, '../research/editor-typeset/results', `variant-${args.variant}`)
    : path.join(HERE, '../research/editor-typeset/results', args.label)
  const engines = args.engine === 'both' ? ['webkit', 'chromium'] : [args.engine]
  for (const engineKey of engines) {
    const launcher = engineKey === 'webkit' ? webkit : chromium
    let browser
    try {
      browser = await launcher.launch({ headless: true })
    } catch (e) {
      console.error(`[${engineKey}] launch failed: ${e.message}`)
      continue
    }
    const outDir = path.join(outRoot, engineKey)
    fs.mkdirSync(outDir, { recursive: true })
    const context = await browser.newContext({ viewport: VIEWPORT, deviceScaleFactor: SCALE })
    const page = await context.newPage()
    const metrics = {
      engine: engineKey,
      engineVersion: browser.version(),
      viewport: VIEWPORT, deviceScaleFactor: SCALE,
      runAt: new Date().toISOString(),
      variant: args.variant || null,
    }
    for (const docName of DOCS) {
      console.log(`[${engineKey}] ${docName} ...`)
      try {
        const r = await runDoc(page, context, docName, outDir, engineKey === 'webkit' ? 'WebKit' : 'Chromium', engineKey, args.variant)
        if (!metrics.scrollViewport) {
          const sv = await evalPage(page, `(() => { const s = scrollViewport(); return { ...describeEl(s), clientHeight: s.clientHeight, clientWidth: s.clientWidth } })()`)
          const col = await evalPage(page, `(() => { const pm = pmRoot(); const r = pm.getBoundingClientRect(); return { editorColumnWidth: +r.width.toFixed(2), pmClass: pm.className } })()`)
          metrics.scrollViewport = sv
          metrics.editorColumn = col
        }
        fs.writeFileSync(path.join(outDir, `${docName}.json`), JSON.stringify(r, null, 2))
        console.log(`  -> ${path.join(outDir, docName + '.json')}`)
      } catch (e) {
        console.error(`  FAILED ${docName}: ${e.message}`)
        fs.writeFileSync(path.join(outDir, `${docName}.error.txt`), String(e.stack || e))
      }
    }
    fs.writeFileSync(path.join(outDir, 'metrics.json'), JSON.stringify(metrics, null, 2))
    await browser.close()
  }
  console.log('done')
}

main().catch((e) => { console.error(e); process.exit(1) })
