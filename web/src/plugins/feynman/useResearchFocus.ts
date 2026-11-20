import { useEffect } from 'react'
import { events } from '../../host/generated/mygo'

export function useResearchFocus(root: string | undefined, refresh: () => void | Promise<void>) {
  useEffect(() => {
    if (!root) return
    const onWindowFocus = () => { void refresh() }
    window.addEventListener('focus', onWindowFocus)
    let offFocus: (() => void) | undefined
    try {
      offFocus = events.researchFocus?.on(ev => {
        if (ev.root === root) onWindowFocus()
      })
    } catch {
      offFocus = undefined
    }
    return () => {
      window.removeEventListener('focus', onWindowFocus)
      offFocus?.()
    }
  }, [root, refresh])
}
