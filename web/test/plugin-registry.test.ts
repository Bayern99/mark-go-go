import { describe, expect, it } from 'vitest'
import { CommandRegistry } from '../src/core/CommandRegistry'
import { DocumentService } from '../src/core/DocumentService'
import { EditorAccess } from '../src/core/EditorAccess'
import { PluginRegistry } from '../src/core/PluginRegistry'
import { ViewRegistry } from '../src/core/ViewRegistry'
import type { AppPlugin, PluginContext } from '../src/core/types'
import type { ViewFileSource, ViewGenerator, ViewSnapshot } from '../src/host/types'

const noGeneration: ViewGenerator & Pick<ViewFileSource, 'undoView'> = {
  generate: () => Promise.reject(new Error('not in these tests')),
  undoView: () => Promise.reject(new Error('not in these tests')),
  getSettings: () => Promise.resolve({ model: '', agentCommand: 'agy' }),
  setSettings: () => Promise.resolve({ model: '', agentCommand: 'agy' }),
  onSettings: () => ({ dispose() {} }),
}

interface Effects {
  subscriptionCalls: number
  subscriptionDisposals: number
  viewMounts: number
  viewDisposals: number
  selectionCalls: number
  selectionDisposals: number
  viewWatchDisposals: number
}

function fakeViewFile(
  effects: Effects,
): ViewFileSource & ViewGenerator & { followers: Set<(snap: ViewSnapshot) => void> } {
  const followers = new Set<(snap: ViewSnapshot) => void>()
  return {
    ...noGeneration,
    followers,
    watchView(fn) {
      followers.add(fn)
      return {
        dispose() {
          if (followers.delete(fn)) effects.viewWatchDisposals += 1
        },
      }
    },
  }
}

function setup() {
  const documents = new DocumentService({ text: 'hello', version: 1 })
  const commands = new CommandRegistry()
  const views = new ViewRegistry()
  const editor = new EditorAccess()
  const selectionListeners = new Set<(offset: number) => void>()
  editor.attach({
    onSelectionChange(fn) {
      selectionListeners.add(fn)
      return {
        dispose() {
          if (!selectionListeners.delete(fn)) return
          effects.selectionDisposals += 1
        },
      }
    },
  })
  const effects: Effects = {
    subscriptionCalls: 0,
    subscriptionDisposals: 0,
    viewMounts: 0,
    viewDisposals: 0,
    selectionCalls: 0,
    selectionDisposals: 0,
    viewWatchDisposals: 0,
  }
  const viewFile = fakeViewFile(effects)
  const registry = new PluginRegistry(documents, views, commands, editor, viewFile)
  const emitSelection = (offset: number) => {
    for (const fn of selectionListeners) fn(offset)
  }

  const plugin: AppPlugin = {
    id: 'demo',
    title: 'Demo',
    activate(ctx: PluginContext) {
      ctx.commands.register('demo.echo', (value) => `echo:${String(value)}`)
      ctx.views.register({
        id: 'demo.view',
        title: 'Demo view',
        location: 'left',
        mount(el) {
          effects.viewMounts += 1
          el.textContent = 'demo view'
          return {
            dispose() {
              effects.viewDisposals += 1
            },
          }
        },
      })
      ctx.subscriptions.push(ctx.documents.onDidChange(() => (effects.subscriptionCalls += 1)))
      ctx.editor.onSelectionChange(() => (effects.selectionCalls += 1))
      ctx.viewFile.watch(() => {})
      ctx.subscriptions.push({
        dispose() {
          effects.subscriptionDisposals += 1
        },
      })
    },
  }

  registry.register(plugin)
  return { documents, commands, views, registry, effects, emitSelection, viewFile }
}

describe('PluginRegistry', () => {
  it('disposes only its own optional plugin registration', async () => {
    const { registry } = setup()
    const plugin = { id: 'optional', title: 'Optional', activate() {} }
    const first = registry.register(plugin)
    await registry.enable(plugin.id)
    first.dispose()
    expect(registry.isEnabled(plugin.id)).toBe(false)
    registry.register(plugin)
    await registry.enable(plugin.id)
    first.dispose()
    expect(registry.isEnabled(plugin.id)).toBe(true)
  })

  it('starts disabled and activates on enable', async () => {
    const { registry, commands, views, documents, effects } = setup()
    expect(registry.isEnabled('demo')).toBe(false)
    expect(views.getAll()).toHaveLength(0)

    await registry.enable('demo')
    expect(registry.isEnabled('demo')).toBe(true)
    expect(views.getAll().map((view) => view.id)).toEqual(['demo.view'])
    expect(await commands.execute('demo.echo', 'x')).toBe('echo:x')

    const view = views.getAll()[0]
    const el = document.createElement('div')
    const mounted = view.mount(el)
    expect(el.textContent).toBe('demo view')
    expect(effects.viewMounts).toBe(1)

    documents.setText('changed')
    expect(effects.subscriptionCalls).toBe(1)

    registry.disable('demo')
    await expect(commands.execute('demo.echo', 'x')).rejects.toThrow(/Unknown command/)
    expect(registry.isEnabled('demo')).toBe(false)
    expect(views.getAll()).toHaveLength(0)
    expect(effects.subscriptionDisposals).toBe(1)

    documents.setText('after disable')
    expect(effects.subscriptionCalls).toBe(1)

    mounted.dispose()
    expect(effects.viewDisposals).toBe(1)
  })

  it('re-enables cleanly', async () => {
    const { registry, commands, effects } = setup()
    await registry.enable('demo')
    registry.disable('demo')
    await registry.enable('demo')

    expect(await commands.execute('demo.echo', 'again')).toBe('echo:again')
    expect(registry.isEnabled('demo')).toBe(true)
    expect(effects.subscriptionDisposals).toBe(1)
  })

  it('disposes editor selection listeners when disabled', async () => {
    const { registry, effects, emitSelection } = setup()
    await registry.enable('demo')
    emitSelection(4)
    expect(effects.selectionCalls).toBe(1)

    registry.disable('demo')
    emitSelection(9)
    expect(effects.selectionCalls).toBe(1)
    expect(effects.selectionDisposals).toBe(1)

    await registry.enable('demo')
    emitSelection(12)
    expect(effects.selectionCalls).toBe(2)
  })

  it('stops following the view file when disabled', async () => {
    const { registry, effects, viewFile } = setup()
    await registry.enable('demo')
    expect(viewFile.followers.size).toBe(1)

    registry.disable('demo')
    expect(viewFile.followers.size).toBe(0)
    expect(effects.viewWatchDisposals).toBe(1)
  })

  it('drops registrations made after a disable during a pending activation', async () => {
    const documents = new DocumentService()
    const commands = new CommandRegistry()
    const views = new ViewRegistry()
    const registry = new PluginRegistry(documents, views, commands, new EditorAccess(), {
      ...noGeneration,
      watchView: () => ({ dispose() {} }),
    })
    let release!: () => void
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })
    let activated = false

    registry.register({
      id: 'slow',
      title: 'Slow',
      async activate(ctx) {
        await gate
        ctx.commands.register('slow.ping', () => 'pong')
        ctx.views.register({
          id: 'slow.view',
          title: 'Slow view',
          location: 'left',
          mount() {
            return { dispose() {} }
          },
        })
        activated = true
      },
    })

    const enabling = registry.enable('slow')
    registry.disable('slow')
    release()
    await enabling

    expect(activated).toBe(true)
    expect(registry.isEnabled('slow')).toBe(false)
    expect(views.getAll()).toHaveLength(0)
    await expect(commands.execute('slow.ping')).rejects.toThrow(/Unknown command/)
  })

  it('disposes everything when activate throws', async () => {
    const documents = new DocumentService()
    const commands = new CommandRegistry()
    const views = new ViewRegistry()
    const registry = new PluginRegistry(documents, views, commands, new EditorAccess(), {
      ...noGeneration,
      watchView: () => ({ dispose() {} }),
    })
    let disposed = false

    registry.register({
      id: 'broken',
      title: 'Broken',
      activate(ctx) {
        ctx.commands.register('broken.run', () => 'ran')
        ctx.subscriptions.push({
          dispose() {
            disposed = true
          },
        })
        throw new Error('boom')
      },
    })

    await expect(registry.enable('broken')).rejects.toThrow('boom')
    expect(registry.isEnabled('broken')).toBe(false)
    expect(disposed).toBe(true)
    await expect(commands.execute('broken.run')).rejects.toThrow(/Unknown command/)
  })
})
