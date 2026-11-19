import { useLayoutEffect, useRef, useState } from 'react'
function ResearchDivider() {
  const drag = useRef<{ x: number; width: number } | null>(null)
  const [width, setWidth] = useState(() => {
    const saved = Number(localStorage.getItem('mgo.researchWidth'))
    return saved >= 310 && saved <= 800 ? saved : 376
  })
  useLayoutEffect(() => {
    document.documentElement.style.setProperty('--research-width', width + 'px')
  }, [width])
  const resize = (size: number) => {
    const value = Math.min(800, Math.max(310, Math.min(window.innerWidth * 0.56, size)))
    setWidth(value)
    return value
  }
  return <div role="separator" aria-label="Resize research panel" aria-orientation="vertical"
    aria-valuemin={310} aria-valuemax={Math.min(800, Math.floor(window.innerWidth * 0.56))} aria-valuenow={Math.round(width)}
    tabIndex={0} className="fy-resize" onPointerDown={e => {
      if (e.button !== 0) return
      drag.current = { x: e.clientX, width: e.currentTarget.parentElement!.getBoundingClientRect().width }
      e.currentTarget.setPointerCapture(e.pointerId); e.preventDefault()
    }} onPointerMove={e => {
      if (drag.current) resize(drag.current.width + drag.current.x - e.clientX)
    }} onLostPointerCapture={() => { drag.current = null; localStorage.setItem('mgo.researchWidth', String(width)) }}
    onDoubleClick={() => localStorage.setItem('mgo.researchWidth', String(resize(376)))}
    onKeyDown={e => {
      if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
      e.preventDefault()
      localStorage.setItem('mgo.researchWidth', String(resize(e.currentTarget.parentElement!.getBoundingClientRect().width + (e.key === 'ArrowLeft' ? 24 : -24))))
    }} />
}

export function RightPane({ children, open, floating, onClose }: {
  children: React.ReactNode
  open: boolean
  floating: boolean
  onClose: () => void
}) {
  return (
    <aside id="feynman-sidebar" className={`right-pane fy-rail${floating ? ' r-float' : ''}`} data-testid="right-pane" aria-label="Feynman" hidden={!open}>
      {!floating && <ResearchDivider />}
      {floating && <button type="button" className="pill icon quiet rp-close" data-testid="right-pane-close"
        aria-label="Close Feynman" title="Close Feynman (Esc)" autoFocus onClick={onClose}>
        <i className="ph ph-x" aria-hidden="true" />
      </button>}
      {children}
    </aside>
  )
}
