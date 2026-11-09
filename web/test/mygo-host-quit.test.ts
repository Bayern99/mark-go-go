import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Disposable, WorkspaceAdapter } from '../src/host/types'

const { bridge } = vi.hoisted(() => {
  const event = () => {
    const listeners = new Set<(value: any) => void>()
    return {
      on(listener: (value: any) => void) {
        listeners.add(listener)
        return () => listeners.delete(listener)
      },
      emit(value: any) {
        for (const listener of listeners) listener(value)
      },
      clear() {
        listeners.clear()
      },
    }
  }
  const flushQuitCall = vi.fn(async (_id: string, _text: string, _attempt: number): Promise<void> => {})
  const prepareQuit = vi.fn(async (_attempt: number, _failure: string): Promise<void> => {})
  const events = { nativeCommand: event(), workspaceChanged: event() }
  return {
    bridge: {
      events,
      flushQuitCall,
      prepareQuit,
      workspaceHost: { flushQuit: flushQuitCall, prepareQuit },
      reset() {
        events.nativeCommand.clear()
        events.workspaceChanged.clear()
        flushQuitCall.mockReset().mockResolvedValue(undefined)
        prepareQuit.mockReset().mockResolvedValue(undefined)
      },
    },
  }
})

vi.mock('mygo-runtime', () => ({
  Channel: class {},
  isMyGo: () => true,
}))

vi.mock('../src/host/generated/mygo', () => ({
  WorkspaceHost: bridge.workspaceHost,
  events: bridge.events,
}))

vi.mock('../src/editor/nativeEdit', () => ({ applyNativeEdit: vi.fn() }))

import { createMyGoHost } from '../src/host/mygoHost'

const flushBindings: Disposable[] = []

beforeEach(() => bridge.reset())
afterEach(() => {
  for (const binding of flushBindings.splice(0)) binding.dispose()
})

function bindReader(workspace: WorkspaceAdapter, id: string, text: string): void {
  const adapter = workspace.document(id)
  if (!adapter.bindFlush) throw new Error('MyGo host must provide bindFlush')
  flushBindings.push(adapter.bindFlush(() => text))
}

function flushQuit(attempt?: number): void {
  bridge.events.nativeCommand.emit({ id: '', command: 'flushQuit', attempt })
}

describe('MyGo host flushQuit', () => {
  it('awaits every reader flush before confirming the attempt', async () => {
    const workspace = createMyGoHost()
    let finishFirst!: () => void
    const first = new Promise<void>((resolve) => {
      finishFirst = resolve
    })
    const flushed: string[] = []
    bridge.flushQuitCall.mockImplementation(async (id, text, attempt) => {
      flushed.push(`${id}:${text}:${attempt}`)
      if (id === 'a') await first
    })
    bindReader(workspace, 'a', '# A\n')
    bindReader(workspace, 'b', '# B\n')

    flushQuit(7)
    await vi.waitFor(() => expect(flushed).toEqual(['a:# A\n:7']))
    expect(bridge.prepareQuit).not.toHaveBeenCalled()

    finishFirst()
    await vi.waitFor(() => expect(bridge.prepareQuit).toHaveBeenCalledWith(7, ''))
    expect(flushed).toEqual(['a:# A\n:7', 'b:# B\n:7'])
    expect(bridge.prepareQuit).toHaveBeenCalledTimes(1)
  })

  it('reports a bounded flush failure with the same attempt after trying every reader', async () => {
    const workspace = createMyGoHost()
    const flushed: string[] = []
    bridge.flushQuitCall.mockImplementation(async (id, text, attempt) => {
      flushed.push(`${id}:${text}:${attempt}`)
      if (id === 'a') throw new Error(`disk busy: ${'x'.repeat(2048)}`)
    })
    bindReader(workspace, 'a', '# A\n')
    bindReader(workspace, 'b', '# B\n')

    flushQuit(12)
    await vi.waitFor(() => expect(bridge.prepareQuit).toHaveBeenCalledTimes(1))

    expect(flushed).toEqual(['a:# A\n:12', 'b:# B\n:12'])
    const [attempt, failure] = bridge.prepareQuit.mock.calls[0]
    expect(attempt).toBe(12)
    expect(failure).toContain('document a')
    expect(failure).toContain('disk busy')
    expect(failure.length).toBeLessThanOrEqual(512)
  })

  it('allows a retry to finish while an older flush is still pending', async () => {
    const workspace = createMyGoHost()
    let releaseFirst!: () => void
    const first = new Promise<void>((resolve) => {
      releaseFirst = resolve
    })
    const attempts: number[] = []
    bridge.flushQuitCall.mockImplementation(async (_id, _text, attempt) => {
      attempts.push(attempt)
      if (attempt === 10) await first
    })
    bindReader(workspace, 'a', '# A\n')

    try {
      flushQuit(10)
      await vi.waitFor(() => expect(attempts).toEqual([10]))
      flushQuit(9)
      flushQuit()
      flushQuit(11)
      await vi.waitFor(() => expect(bridge.prepareQuit).toHaveBeenCalledWith(11, ''))

      expect(attempts).toEqual([10, 11])
      expect(bridge.prepareQuit).toHaveBeenCalledTimes(1)
    } finally {
      releaseFirst()
    }
  })
})
