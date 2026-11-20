import { afterEach, beforeEach, vi } from 'vitest'

beforeEach(() => {
  vi.stubGlobal('mygo', {
    preferences: { reduceTransparency: false },
    on: () => () => {},
    callWith: async (method: string) => {
      if (method !== 'WorkspaceHost.SetNativeEditState') throw new Error('Unexpected native call: ' + method)
    },
  })
  localStorage.setItem('mgo.plugin.feynman', 'on')
})

afterEach(() => {
  localStorage.removeItem('mgo.plugin.feynman')
  vi.unstubAllGlobals()
})
