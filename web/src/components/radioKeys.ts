import type { KeyboardEvent } from 'react'

export function radioKeys(event: KeyboardEvent<HTMLElement>) {
  const radios = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('button[role="radio"]:not(:disabled)'))
  const index = radios.indexOf(event.target as HTMLButtonElement)
  if (index === -1) return
  let next: number
  switch (event.key) {
    case 'Home': next = 0; break
    case 'End': next = radios.length - 1; break
    case 'ArrowDown':
    case 'ArrowRight': next = (index + 1) % radios.length; break
    case 'ArrowUp':
    case 'ArrowLeft': next = (index - 1 + radios.length) % radios.length; break
    default: return
  }
  event.preventDefault()
  radios[next].click()
  radios[next].focus()
}
