import { createRequire } from 'node:module'
import fs from 'node:fs'
import path from 'node:path'
const require = createRequire(new URL('../../web/package.json', import.meta.url))
const ENGINE = (process.argv.find((a) => a.startsWith('--engine'))?.split('=')[1]) || 'webkit'
const SECTION = process.argv[2] && !process.argv[2].startsWith('--') ? process.argv[2] : 'all'
const pw = require('playwright')
const browser = await pw[ENGINE].launch({ headless: true })
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1 })
const page = await ctx.newPage()
const OUT = 'docs/research/editor-typeset/results/r3'
fs.mkdirSync(OUT, { recursive: true })
const report = {}

const url = (doc) => `http://localhost:5173/bench/typeset.html?doc=${doc}`
async function load(doc) {
  await page.goto(url(doc))
  await page.waitForSelector('.ProseMirror')
  await page.evaluate(() => document.fonts.ready)
  await page.waitForTimeout(400)
}
const ev = (fn, arg) => page.evaluate(fn, arg)

const helpers = `
function pmRoot(){ return document.querySelector('.crepe-root .ProseMirror') }
function pmView(){ return window.__pmView }
function scrollViewport(){ let n = pmRoot(); while(n && n!==document.body){ const s=getComputedStyle(n); if(/(auto|scroll)/.test(s.overflowY)&&n.scrollHeight>n.clientHeight+4) return n; n=n.parentElement } return document.scrollingElement||document.documentElement }
function selRect(){ const v=pmView(); const {from,to}=v.state.selection; const r=document.createRange(); const s=v.domAtPos(from); const e=v.domAtPos(to); r.setStart(s.node,s.offset); r.setEnd(e.node,e.offset); const rects=[...r.getClientRects()]; if(!rects.length) return null; const l=Math.min(...rects.map(r=>r.left)),t=Math.min(...rects.map(r=>r.top)),rr=Math.max(...rects.map(r=>r.right)),b=Math.max(...rects.map(r=>r.bottom)); return {top:t,bottom:b,left:l,right:rr,h:b-t,lines:rects.length} }
function toolbarRect(){ const el=document.querySelector('.milkdown-toolbar'); if(!el) return null; const r=el.getBoundingClientRect(); return {top:r.top,bottom:r.bottom,left:r.left,right:r.right,h:r.height,w:r.width,shown:el.dataset.show,cls:el.className.slice(0,60)} }
function prevLineBottom(){ const s=selRect(); if(!s) return null; return s.top }
function setSel(from,to){ const v=pmView(); v.dispatch(v.state.tr.setSelection(v.state.selection.constructor.between?v.state.selection:window.__PM.TextSelection.create(v.state.doc,from,to))) }
function textSel(from,to){ const {TextSelection}=window.__PM; const v=pmView(); v.dispatch(v.state.tr.setSelection(TextSelection.create(v.state.doc,from,to))) }
function findPara(txt){ let hit=null; pmRoot().querySelectorAll('p,h1,h2,h3,h4,h5,h6,li,blockquote').forEach(el=>{ if(!hit&&el.textContent.includes(txt)) hit=el }); return hit }
function posOf(el,off=0){ const v=pmView(); return v.posAtDOM(el,Math.min(off,el.childNodes.length)) }
Object.assign(window,{selRect,toolbarRect,textSel,findPara,posOf,scrollViewport,pmRoot,pmView})
`
await ev(`import('/node_modules/.vite/deps/@milkdown_prose_state.js').then(m=>{window.__PM=m}).catch(()=>{})`).catch(()=>{})
const hasPM = await ev(`(()=>!!window.__PM)()`)

async function issueA() {
  for (const doc of ['paper-english','tech-report-tables']) {
    await load(doc)
    await ev(`(()=>{ ${helpers}; window.__h={selRect,toolbarRect,findPara,posOf,scrollViewport,pmView}; return true })()`)
    const target = await ev(`(()=>{ const el=__h.findPara('${doc==='paper-english'?'typographic rhythm':'MarkSync'}'); if(!el) return null; el.scrollIntoView({block:'center'}); const r=el.getBoundingClientRect(); return {x:r.left+120,y:r.top+r.height/2,text:el.textContent.slice(0,40)} })()`)
    if (!target) { report[`A-${doc}`] = 'no target'; continue }
    await page.mouse.click(target.x, target.y, { clickCount: 3 })
    await page.waitForTimeout(700)
    const g1 = await ev(`(()=>({ sel:__h.selRect(), tb:__h.toolbarRect(), scrollTop:__h.scrollViewport().scrollTop }))()`)
    const g2 = await ev(`(()=>{ __h.scrollViewport().scrollTop += 60; return new Promise(r=>setTimeout(()=>r({ sel:__h.selRect(), tb:__h.toolbarRect(), scrollTop:__h.scrollViewport().scrollTop }),300)) })()`)
    const g3 = await ev(`(()=>{ __h.scrollViewport().scrollTop += 60; return new Promise(r=>setTimeout(()=>r({ sel:__h.selRect(), tb:__h.toolbarRect(), scrollTop:__h.scrollViewport().scrollTop }),300)) })()`)
    report[`A-${doc}`] = { target: target.text, settled: g1, afterScroll1: g2, afterScroll2: g3 }
  }
}

async function issueB() {
  await load('tech-report-tables')
  await ev(`(()=>{ ${helpers}; window.__h={selRect,toolbarRect,scrollViewport,pmView}; window.__selLog=[]; const v=pmView(); const iv=setInterval(()=>{ const s=v.state.selection; __selLog.push(s.constructor.name+':'+s.from+'-'+s.to) },40); return true })()`)
  const cells = await ev(`(()=>{ const tb=[...document.querySelectorAll('.crepe-root .ProseMirror table')].find(t=>t.getBoundingClientRect().height>0); if(!tb) return null; tb.scrollIntoView({block:'center'}); const tds=[...tb.querySelectorAll('td,th')].filter(td=>td.getBoundingClientRect().height>0); return tds.slice(0,6).map(td=>{const r=td.getBoundingClientRect(); return {x:r.left+r.width/2,y:r.top+r.height/2,w:r.width,h:r.height}}) })()`)
  if (!cells) { report.B = 'no table'; return }
  const a = cells[0], b = cells[2]
  await page.mouse.move(a.x - a.w/4, a.y)
  await page.mouse.down()
  for (let i = 1; i <= 8; i++) await page.mouse.move(a.x - a.w/4 + (b.x + b.w/4 - (a.x - a.w/4)) * i / 8, a.y, { steps: 2 })
  await page.waitForTimeout(150)
  const during = await ev(`window.__selLog.slice(-12)`)
  await page.mouse.up()
  await page.waitForTimeout(300)
  const after = await ev(`(()=>({ sel: pmView().state.selection.constructor.name, from: pmView().state.selection.from, to: pmView().state.selection.to, log: window.__selLog.slice(-6) }))()`)
  report.B = { during, after }
}

async function continuity() {
  const results = {}
  await load('paper-english')
  await ev(`(()=>{ ${helpers}; window.__h={selRect,toolbarRect,scrollViewport,pmView,findPara,posOf}; return true })()`)

  const t1 = await ev(`(()=>{ const el=__h.findPara('typographic rhythm'); el.scrollIntoView({block:'center'}); const r=el.getBoundingClientRect(); return {x:r.left+200,y:r.top+r.height/2} })()`)
  await page.mouse.click(t1.x, t1.y)
  await page.waitForTimeout(200)
  results.click = await ev(`(()=>({ empty: pmView().state.selection.empty, type: pmView().state.selection.constructor.name }))()`)

  await page.mouse.click(t1.x, t1.y, { clickCount: 2 })
  await page.waitForTimeout(250)
  results.dbl = await ev(`(()=>({ sel: pmView().state.doc.textBetween(pmView().state.selection.from, pmView().state.selection.to), tb: __h.toolbarRect()?.shown }))()`)

  await page.mouse.click(t1.x, t1.y, { clickCount: 3 })
  await page.waitForTimeout(700)
  const tbBtn = await ev(`(()=>{ const tb=[...document.querySelectorAll('.crepe-root [data-show="true"]')][0]; if(!tb) return null; const b=tb.querySelector('button'); if(!b) return null; const r=b.getBoundingClientRect(); return {x:r.left+r.width/2,y:r.top+r.height/2,label:b.textContent||b.getAttribute('aria-label')} })()`)
  results.toolbarBtn = tbBtn
  if (tbBtn) {
    await page.mouse.click(tbBtn.x, tbBtn.y)
    await page.waitForTimeout(300)
    results.afterToolbarClick = await ev(`(()=>({ focus: pmView().hasFocus(), selType: pmView().state.selection.constructor.name, empty: pmView().state.selection.empty }))()`)
    await page.keyboard.type('ZZ')
    await page.waitForTimeout(200)
    results.typeAfterToolbar = await ev(`(()=>({ focus: pmView().hasFocus(), text: pmView().state.doc.textContent.includes('ZZ') }))()`)
    await page.keyboard.press('Escape')
    await page.keyboard.down('Meta'); await page.keyboard.press('z'); await page.keyboard.up('Meta')
    await page.keyboard.down('Meta'); await page.keyboard.press('z'); await page.keyboard.up('Meta')
  }

  await page.mouse.click(t1.x, t1.y)
  for (let i=0;i<5;i++) await page.keyboard.press('Shift+ArrowRight')
  results.shiftSel = await ev(`(()=>pmView().state.doc.textBetween(pmView().state.selection.from,pmView().state.selection.to))()`)
  await page.keyboard.down('Meta'); await page.keyboard.press('a'); await page.keyboard.up('Meta')
  await page.waitForTimeout(200)
  results.cmdA = await ev(`(()=>pmView().state.selection.constructor.name+':'+(pmView().state.selection.to-pmView().state.selection.from))()`)
  await page.keyboard.press('Escape'); await page.mouse.click(t1.x, t1.y)

  await ev(`(()=>{ const s=__h.scrollViewport(); s.scrollTop = s.scrollHeight/2; return true })()`)
  await page.waitForTimeout(300)
  const mid = await ev(`(()=>{ const ps=[...pmRoot().children].filter(e=>{const r=e.getBoundingClientRect(); return r.top>300&&r.bottom<600&&/^P$/.test(e.tagName)}); if(!ps.length) return null; const r=ps[0].getBoundingClientRect(); return {x:r.left+150,y:r.top+r.height/2,idx:[...pmRoot().children].indexOf(ps[0])} })()`)
  if (mid) {
    await page.mouse.click(mid.x, mid.y)
    await page.waitForTimeout(150)
    const st1 = await ev(`(()=>({ scrollTop:__h.scrollViewport().scrollTop, caret:(()=>{const v=pmView(); const c=v.coordsAtPos(v.state.selection.from); return {top:c.top,bottom:c.bottom}})() }))()`)
    await page.keyboard.type('QQ')
    await page.waitForTimeout(150)
    const st2 = await ev(`(()=>({ scrollTop:__h.scrollViewport().scrollTop, caret:(()=>{const v=pmView(); const c=v.coordsAtPos(v.state.selection.from); return {top:c.top,bottom:c.bottom}})() }))()`)
    results.midScrollType = { before: st1, after: st2, drift: Math.abs(st2.scrollTop - st1.scrollTop) }
    await page.keyboard.down('Meta'); await page.keyboard.press('z'); await page.keyboard.up('Meta'); await page.keyboard.down('Meta'); await page.keyboard.press('z'); await page.keyboard.up('Meta')
  }

  await page.mouse.click(t1.x, t1.y)
  await page.evaluate(() => navigator.clipboard.writeText('PASTED-TOKEN')).catch(()=>{})
  const clip = await ctx.grantPermissions(['clipboard-read','clipboard-write']).then(()=>true).catch(()=>false)
  if (clip) { await page.evaluate(() => navigator.clipboard.writeText('PASTED-TOKEN')); await page.keyboard.down('Meta'); await page.keyboard.press('v'); await page.keyboard.up('Meta'); await page.waitForTimeout(300) }
  results.paste = await ev(`(()=>({ has: pmView().state.doc.textContent.includes('PASTED-TOKEN'), focus: pmView().hasFocus() }))()`)
  await page.keyboard.down('Meta'); await page.keyboard.press('z'); await page.keyboard.up('Meta')
  await page.waitForTimeout(200)
  results.undo = await ev(`(()=>({ removed: !pmView().state.doc.textContent.includes('PASTED-TOKEN'), empty: pmView().state.selection.empty }))()`)

  const t2 = await ev(`(()=>{ const el=__h.findPara('Markdown editors'); if(!el) return null; el.scrollIntoView({block:'center'}); const r=el.getBoundingClientRect(); return {x:r.left+20,y:r.top+14,w:r.width} })()`)
  if (t2) {
    await page.mouse.move(t2.x, t2.y); await page.mouse.down()
    for (let i=1;i<=6;i++) await page.mouse.move(t2.x + t2.w*0.6*i/6, t2.y + 20*i/6, {steps:2})
    const midDrag = await ev(`(()=>__h.toolbarRect()?.shown ?? 'none')()`)
    await page.mouse.up(); await page.waitForTimeout(500)
    const settled = await ev(`(()=>__h.toolbarRect()?.shown ?? 'none')()`)
    results.dragToolbar = { midDrag, settled }
  }
  results.markdown = await ev(`(()=>window.__bench.host.load().then(s=>({len:s.text.length})))()`)
  report.continuity = results
}

if (SECTION === 'all' || SECTION === 'A') await issueA()
if (SECTION === 'all' || SECTION === 'B') await issueB()
if (SECTION === 'all' || SECTION === 'C') await continuity()
fs.writeFileSync(path.join(OUT, `${SECTION}-${ENGINE}.json`), JSON.stringify(report, null, 1))
console.log(JSON.stringify(report, null, 1).slice(0, 6000))
await browser.close()
