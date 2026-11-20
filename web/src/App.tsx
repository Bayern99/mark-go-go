import { isMyGo, onPreferences, preferences, type ThemePreferences } from 'mygo-runtime'
import { Tooltip } from 'radix-ui'
import { useLayoutEffect } from 'react'
import type { AppPlugin } from './core/types'
import { wrapHostAsWorkspace } from './host/browserHost'
import type { HostAdapter, WorkspaceAdapter } from './host/types'
import { NARROW_WINDOW_WIDTH, WorkspaceShell } from './components/WorkspaceShell'
import './components/ui.css'

export { NARROW_WINDOW_WIDTH }

export function App({
  host,
  workspace,
  plugins = [],
}: {
  host?: HostAdapter
  workspace?: WorkspaceAdapter
  plugins?: AppPlugin[]
}): React.ReactElement {
  useLayoutEffect(() => {
    if (!isMyGo()) return
    const root = document.documentElement
    const apply = ({ reduceTransparency }: ThemePreferences) => {
      root.toggleAttribute('data-reduce-transparency', reduceTransparency)
    }
    apply(preferences())
    const unsubscribe = onPreferences(apply)
    return () => {
      unsubscribe()
      root.removeAttribute('data-reduce-transparency')
    }
  }, [])

  const resolved = workspace ?? (host ? wrapHostAsWorkspace(host) : null)
  if (!resolved) throw new Error('App needs a workspace or a document host')
  return (
    <Tooltip.Provider delayDuration={400} skipDelayDuration={250}>
      <WorkspaceShell workspace={resolved} plugins={plugins} />
    </Tooltip.Provider>
  )
}
