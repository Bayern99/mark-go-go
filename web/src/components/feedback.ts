const TICK_MS = 1200

let lastPress: EventTarget | null = null

if (typeof document !== 'undefined') {
  document.addEventListener(
    'pointerdown',
    (e) => {
      lastPress = e.target
    },
    { capture: true },
  )
}

export function notifyCopied(_kind: 'Code' | 'Link'): void {
  const active = document.activeElement
  const button =
    active instanceof HTMLElement && active.tagName === 'BUTTON'
      ? active
      : lastPress instanceof Element
        ? lastPress.closest('button')
        : null
  const icon = button?.firstElementChild
  if (!button || !(icon instanceof HTMLElement) || button.dataset.copyBusy) return
  button.dataset.copyBusy = '1'
  const original = icon.innerHTML
  icon.innerHTML = '<i class="ph ph-check" aria-hidden="true"></i>'
  window.setTimeout(() => {
    icon.innerHTML = original
    delete button.dataset.copyBusy
  }, TICK_MS)
}
