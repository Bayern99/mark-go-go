import { useEffect, useState, useSyncExternalStore } from 'react'
import { DropdownMenu } from 'radix-ui'
import { isMyGo } from 'mygo-runtime'
import type { PluginRegistry } from '../core/PluginRegistry'
import type { AppPlugin } from '../core/types'
import { createFeynmanPlugin } from './feynman'
import { ResearchSession } from './feynman/session'
import { createGitPlugin } from './git'
import './desktop-plugins.css'

type PluginID = 'feynman' | 'git'
const changed = 'mgo-plugins-changed'
const setting = (id: PluginID) => 'mgo.plugin.' + id

function subscribe(fn: () => void) {
  window.addEventListener(changed, fn)
  window.addEventListener('storage', fn)
  return () => {
    window.removeEventListener(changed, fn)
    window.removeEventListener('storage', fn)
  }
}

export function usePluginEnabled(id: PluginID) {
  return useSyncExternalStore(subscribe, () => isMyGo() && localStorage.getItem(setting(id)) === 'on')
}

function activate(registry: PluginRegistry, plugin: AppPlugin, onError: (message: string) => void) {
  const entry = registry.register(plugin)
  void registry.enable(plugin.id).catch(err => onError(String(err)))
  return () => entry.dispose()
}

export function DesktopPlugins({ registry, documentID, workspaceRoot, openDocument }: {
  registry: PluginRegistry
  documentID: string
  workspaceRoot: string
  openDocument: (path: string) => Promise<unknown>
}) {
  const research = usePluginEnabled('feynman')
  const git = usePluginEnabled('git')
  const [error, setError] = useState('')
  useEffect(() => {
    if (research) return activate(registry, createFeynmanPlugin(workspaceRoot, documentID, openDocument), setError)
  }, [registry, research, workspaceRoot, documentID, openDocument])
  useEffect(() => {
    if (git) return activate(registry, createGitPlugin(documentID), setError)
  }, [registry, git, documentID])

  return error ? <span role="alert">{error}</span> : null
}

function WorkspaceSession({ root }: { root: string }) {
  useEffect(() => {
    const session = ResearchSession.acquire(root)
    return () => session.release()
  }, [root])
  return null
}

export function PluginMenu({ workspaces }: { workspaces: readonly { root: string }[] }) {
  const research = usePluginEnabled('feynman')
  const git = usePluginEnabled('git')
  if (!isMyGo()) return null
  return <>
    {research && workspaces.filter(w => w.root).map(w => <WorkspaceSession key={w.root} root={w.root} />)}
    <DropdownMenu.Root modal={false}>
      <DropdownMenu.Trigger asChild>
        <button className="pill icon quiet" aria-label="Plugins" title="Plugins"><i className="ph ph-plugs" aria-hidden="true" /></button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content className="mmenu pop" aria-label="Plugins" align="end" sideOffset={6}>
          {(['feynman', 'git'] as const).map(id => (
            <DropdownMenu.CheckboxItem key={id} className="mi" checked={id === 'feynman' ? research : git}
              onCheckedChange={enabled => {
                localStorage.setItem(setting(id), enabled ? 'on' : 'off')
                window.dispatchEvent(new Event(changed))
              }}>
              {id === 'feynman' ? 'Feynman' : 'Git'}
              <DropdownMenu.ItemIndicator className="mi-check">✓</DropdownMenu.ItemIndicator>
            </DropdownMenu.CheckboxItem>
          ))}
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>

  </>
}
