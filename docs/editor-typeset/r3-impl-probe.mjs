// Round 3 implementation verification. Run with vite dev server on :5173.
// node docs/editor-typeset/r3-impl-probe.mjs [--engine webkit|chromium]
import { createRequire } from 'node:module'
import fs from 'node:fs'
const require = createRequire(new URL('../../web/package.json', import.meta.url))
const ENGINE = (process.argv.find((a) => a.startsWith('--engine'))?.split('=')[1]) || 'webkit'
const pw = require('playwright')
const browser = await pw[ENGINE].launch({ headless: true })
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1 })
const page = await ctx.newPage()
const OUT = 'docs/research/editor-typeset/results/r3impl'
fs.mkdirSync(OUT, { recursive: true })
const report = { engine: ENGINE }

const url = (doc) => `http://localhost:5173/bench/typeset.html?doc=${doc}`
async function load(doc) {
  await page.goto(url(doc))
  await page.waitForSelector('.ProseMirror')
  await page.evaluate(() => document.fonts.ready)
  await page.waitForTimeout(500)
}
const ev = (fn) => page.evaluate(fn)

const helpers = `
function pmRoot(){ return document.querySelector('.crepe-root .ProseMirror') }
function pmView(){ return window.__pmView }
function sel(){ const s=pmView().state.selection; return {type:s.constructor.name,from:s.from,to:s.to} }
function toggle(pos){ pmView().dispatch(pmView().state.tr.setMeta('mgo-fold$',{toggle:pos}).setMeta('addToHistory',false)) }
function topNodes(){ const out=[]; pmView().state.doc.forEach((n,p)=>out.push({type:n.type.name,pos:p,text:n.textContent.slice(0,30)})); return out }
Object.assign(window,{pmRoot,pmView,sel,toggle,topNodes})
`

// ---------- 1. Table interactions ----------
await load('tech-report-tables')
await ev(`(()=>{ ${helpers}; return true })()`)
const cells = await ev(`(()=>{ const tb=[...document.querySelectorAll('.ProseMirror table')].find(t=>t.getBoundingClientRect().height>0); if(!tb) return null; tb.scrollIntoView({block:'center'}); const tds=[...tb.querySelectorAll('td,th')].filter(td=>td.getBoundingClientRect().height>0); return tds.slice(0,8).map(td=>{const r=td.getBoundingClientRect(); return {x:r.left+r.width/2,y:r.top+r.height/2,w:r.width,h:r.height,row:td.parentElement.rowIndex,text:td.textContent.slice(0,20)}}) })()`)
report.cellCount = cells?.length

const drag = async (a, b, label, steps = 10) => {
  await page.mouse.move(a.x, a.y)
  await page.mouse.down()
  for (let i = 1; i <= steps; i++) await page.mouse.move(a.x + (b.x - a.x) * i / steps, a.y + (b.y - a.y) * i / steps, { steps: 1 })
  await page.waitForTimeout(80)
  const during = await ev(`window.sel()`)
  await page.mouse.up()
  await page.waitForTimeout(250)
  const after = await ev(`window.sel()`)
  const selCells = await ev(`document.querySelectorAll('.ProseMirror .selectedCell').length`)
  report[label] = { during, after, selCells }
}

if (cells && cells.length >= 6) {
  // cross-cell drag: row 0 cell 0 → row 1 cell 2 (rectangular)
  const row = (ri) => cells.filter(c => c.row === ri)
  const r0 = row(0), r1 = row(1)
  if (r0.length && r1.length >= 3) {
    await drag(r0[0], r1[2], 'drag-cross')
  } else {
    await drag(cells[0], cells[2], 'drag-cross')
  }
  // intra-cell drag: text selection inside one cell
  const c = cells[0]
  await drag({ x: c.x - c.w/4, y: c.y }, { x: c.x + c.w/4, y: c.y }, 'drag-intra')
  // single click → caret
  await page.mouse.click(c.x, c.y)
  await page.waitForTimeout(250)
  report['click'] = await ev(`window.sel()`)
  // double click → word
  await page.mouse.click(c.x, c.y, { clickCount: 2 })
  await page.waitForTimeout(300)
  report['dblclick'] = await ev(`(()=>({...window.sel(), text:pmView().state.doc.textBetween(pmView().state.selection.from,pmView().state.selection.to)}))()`)
}

// ---------- 2. Folding ----------
await load('tech-report-tables')
await ev(`(()=>{ ${helpers}; return true })()`)
const tops = await ev(`window.topNodes()`)
report.tops = tops.slice(0, 12)
const h2 = tops.find(t => t.type === 'heading' && t.text.includes('3'))
const code = tops.find(t => t.type === 'code_block')
report.h2 = h2; report.code = code

if (h2) {
  const before = await ev(`(()=>({ md: pmView().state.doc.toString(), hidden: document.querySelectorAll('.mgo-fold-hidden').length }))()`)
  await ev(`window.toggle(${h2.pos})`)
  await page.waitForTimeout(150)
  const folded = await ev(`(()=>({ md: pmView().state.doc.toString(), hidden: document.querySelectorAll('.mgo-fold-hidden').length, toggleVisible: !!document.querySelector('.mgo-is-folded .mgo-fold-toggle') }))()`)
  report.foldH2 = { docUnchanged: before.md === folded.md, hidden: folded.hidden, chevron: folded.toggleVisible }
  const el = await ev(`(()=>{ const h=document.querySelector('.mgo-is-folded'); if(!h) return null; h.scrollIntoView({block:'center'}); const r=h.getBoundingClientRect(); return {x:r.left+r.width/2,y:r.top+r.height/2} })()`)
  await page.screenshot({ path: `${OUT}/fold-section.png`, clip: { x: 0, y: Math.max(0, el.y - 120), width: 1440, height: 320 } })
  // expand again
  await ev(`window.toggle(${h2.pos})`)
  await page.waitForTimeout(150)
  report.unfoldH2 = await ev(`document.querySelectorAll('.mgo-fold-hidden').length`)
}
if (code) {
  await ev(`(()=>{ document.querySelectorAll('.milkdown-code-block')[0]?.scrollIntoView({block:'center'}) })()`)
  await page.waitForTimeout(200)
  const hasBtn = await ev(`!!document.querySelector('.milkdown-code-block .mgo-code-fold-btn')`)
  await ev(`window.toggle(${code.pos})`)
  await page.waitForTimeout(200)
  const stub = await ev(`(()=>{ const b=document.querySelector('.mgo-code-folded'); if(!b) return null; const r=b.getBoundingClientRect(); return {h:r.height, stub:getComputedStyle(b,'::after').content, childVisible:[...b.children].some(c=>getComputedStyle(c).display!=='none')} })()`)
  report.codeFold = { hasBtn, ...stub }
  const r = await ev(`(()=>{ const b=document.querySelector('.mgo-code-folded'); if(!b) return null; const r=b.getBoundingClientRect(); return {y:r.top} })()`)
  if (r) await page.screenshot({ path: `${OUT}/fold-code.png`, clip: { x: 0, y: Math.max(0, r.y - 80), width: 1440, height: 200 } })
}

// ---------- 3. Toolbar placement data (for the design decision) ----------
await load('paper-english')
await ev(`(()=>{ ${helpers}; window.__selRect=function(){ const v=pmView(); const r=document.createRange(); const s=v.domAtPos(v.state.selection.from); const e=v.domAtPos(v.state.selection.to); r.setStart(s.node,s.offset); r.setEnd(e.node,e.offset); const rects=[...r.getClientRects()]; const l=Math.min(...rects.map(r=>r.left)),t=Math.min(...rects.map(r=>r.top)),rr=Math.max(...rects.map(r=>r.right)),b=Math.max(...rects.map(r=>r.bottom)); return {top:t,bottom:b,left:l,right:rr} }; return true })()`)
const para = await ev(`(()=>{ const p=[...document.querySelectorAll('.ProseMirror p')].find(p=>p.getBoundingClientRect().height>0&&p.textContent.length>80); if(!p) return null; p.scrollIntoView({block:'center'}); const r=p.getBoundingClientRect(); return {x:r.left+100,y:r.top+10} })()`)
if (para) {
  await page.mouse.click(para.x, para.y, { clickCount: 3 })
  await page.waitForTimeout(600)
  report.toolbar = await ev(`(()=>{ const tb=document.querySelector('.milkdown-toolbar'); if(!tb) return null; const tr=tb.getBoundingClientRect(); const pm=document.querySelector('.ProseMirror').getBoundingClientRect(); const col=pm; const sel=window.__selRect(); return {tb:{top:tr.top,left:tr.left,w:tr.width,h:tr.height}, sel, pm:{left:pm.left,right:pm.right,w:pm.width}, overlapAbove: sel.top - tr.bottom } })()`)
}

console.log(JSON.stringify(report, null, 1))
await browser.close()
