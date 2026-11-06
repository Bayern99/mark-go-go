import { describe, expect, it } from 'vitest'
import { mountEditor } from './helpers/editor'

const files = import.meta.glob('./fixtures/*.md', {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>

describe('byte-identical round trip', () => {
  const names = Object.keys(files).sort()

  it('has the corpus', () => {
    expect(names.length).toBeGreaterThanOrEqual(21)
  })

  for (const name of names) {
    it(name.split('/').pop() ?? name, async () => {
      const source = files[name]
      const mounted = await mountEditor(source)
      try {
        expect(mounted.editor.getMarkdown()).toBe(source)
      } finally {
        await mounted.cleanup()
      }
    })
  }
})
