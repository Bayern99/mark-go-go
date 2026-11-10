import { describe, expect, it } from 'vitest'
import { loadSurface, validateGeneratedView } from '../src/plugins/genui/a2ui'

const CATALOG = 'markgogo.dev:genui/v1'

type Component = Record<string, unknown> & { id: string; component: string }

/** One createSurface message with the given components. */
function createSurface(components: Component[], surfaceId = 'view'): unknown {
  return {
    version: '1.0',
    createSurface: { surfaceId, catalogId: CATALOG, components },
  }
}

/** A complete view file, as the host writes it. */
function viewFile(messages: unknown[], sources: Record<string, unknown> = {}): string {
  return JSON.stringify({
    version: 2,
    generatedAt: '2026-11-10T02:47:20Z',
    sources,
    a2ui: messages,
  })
}

/** The reason a generated view is refused. */
function errorOf(raw: string): string {
  const verdict = validateGeneratedView(raw)
  expect(verdict.ok).toBe(false)
  return verdict.ok ? '' : verdict.error
}

const OVERVIEW = { overview: { heading: 'Overview', occurrence: 1, hash: 'fnv1a32:00000000' } }

function validComponents(): Component[] {
  return [
    { id: 'root', component: 'Column', children: ['kp'] },
    { id: 'kp', component: 'KeyPoints', title: 'Points', items: ['One.'], sources: ['overview'] },
  ]
}

/** A merged update: the base view's messages plus the agent's message. */
function mergedMessages(): unknown[] {
  return [
    createSurface([
      { id: 'root', component: 'Column', children: ['a', 'b'] },
      { id: 'a', component: 'KeyPoints', items: ['Old A'] },
      { id: 'b', component: 'KeyPoints', items: ['Old B'] },
    ]),
    {
      version: '1.0',
      updateComponents: {
        surfaceId: 'view',
        components: [
          { id: 'b', component: 'Card', child: 'c' },
          { id: 'c', component: 'KeyPoints', items: ['New C'] },
        ],
      },
    },
  ]
}

describe('generated view validation', () => {
  it('accepts a whole generated view and a merged update', () => {
    expect(validateGeneratedView(viewFile([createSurface(validComponents())], OVERVIEW))).toEqual({ ok: true })
    expect(validateGeneratedView(viewFile(mergedMessages()))).toEqual({ ok: true })
  })

  it('rejects a version 2 envelope that is not JSON or has no messages', () => {
    expect(errorOf('not json')).toContain('not valid JSON')
    expect(errorOf(JSON.stringify({ version: 1, sources: {}, a2ui: [] }))).toContain('version 2')
    expect(errorOf(viewFile([]))).toContain('no A2UI messages')
  })

  it('rejects a known component whose properties are wrong', () => {
    const error = errorOf(
      viewFile([createSurface([{ id: 'root', component: 'MetricGrid', metrics: [{ label: 'Size' }] }])]),
    )
    expect(error).toContain('metrics.0.value')
  })

  it('rejects the Card holding Tabs that the agent actually wrote, despite a valid envelope', () => {
    const corrupted: Component[] = [
      { id: 'root', component: 'Column', children: ['card'] },
      { id: 'card', component: 'Card', child: 'tabs' },
      { id: 'tabs', component: 'Tabs', tabs: [{ title: 'A tab', child: 'kp' }] },
      { id: 'kp', component: 'KeyPoints', items: ['A point.'] },
    ]
    const error = errorOf(viewFile([createSurface(corrupted)]))
    expect(error).toContain('UNALLOWED_')
    expect(error).toContain('Tabs')
  })

  it('rejects missing references, cycles, orphans and a missing root', () => {
    expect(errorOf(viewFile([createSurface([
      { id: 'root', component: 'Column', children: ['gone'] },
    ])]))).toContain("non-existent component 'gone'")

    expect(errorOf(viewFile([createSurface([
      { id: 'root', component: 'Column', children: ['a'] },
      { id: 'a', component: 'Column', children: ['root'] },
    ])]))).toContain('Circular reference')

    expect(errorOf(viewFile([createSurface([
      { id: 'root', component: 'Column', children: ['kp'] },
      { id: 'kp', component: 'KeyPoints', items: ['One.'] },
      { id: 'extra', component: 'KeyPoints', items: ['Two.'] },
    ])]))).toContain("not reachable from 'root'")

    expect(errorOf(viewFile([createSurface([
      { id: 'top', component: 'KeyPoints', items: ['One.'] },
    ])]))).toContain("Missing root component")
  })

  it('rejects an unknown generated component while a hand-written view still shows its placeholder', () => {
    const messages = [createSurface([{ id: 'root', component: 'FancyBox' }])]
    expect(errorOf(viewFile(messages))).toContain("'FancyBox'")

    const loaded = loadSurface(messages)
    expect(loaded.ok).toBe(true)
    if (loaded.ok) loaded.dispose()
  })

  it('accepts a merged update that retires an old child, while a full view still rejects an orphan', () => {
    const components: Component[] = [
      { id: 'root', component: 'Column', children: ['card', 'b'] },
      { id: 'card', component: 'Card', child: 'leaf' },
      { id: 'leaf', component: 'KeyPoints', items: ['Old point.'] },
      { id: 'b', component: 'KeyPoints', items: ['B.'] },
    ]
    const merged = [
      createSurface(components),
      {
        version: '1.0',
        updateComponents: {
          surfaceId: 'view',
          components: [{ id: 'card', component: 'KeyPoints', items: ['Flattened.'] }],
        },
      },
    ]
    // Replacing the Card with a leaf unlinks 'leaf'; the update cannot
    // delete it, so it stays retired in the history and is still valid.
    expect(validateGeneratedView(viewFile(merged))).toEqual({ ok: true })

    // A whole view has no history: the same unreachable component is refused.
    const orphaned: Component[] = [
      { id: 'root', component: 'Column', children: ['card', 'b'] },
      { id: 'card', component: 'KeyPoints', items: ['A.'] },
      { id: 'b', component: 'KeyPoints', items: ['B.'] },
      { id: 'leaf', component: 'KeyPoints', items: ['Old point.'] },
    ]
    expect(errorOf(viewFile([createSurface(orphaned)]))).toContain("not reachable from 'root'")
  })

  it('rejects a source a component uses that the view does not define', () => {
    const error = errorOf(viewFile([createSurface(validComponents())]))
    expect(error).toContain('source')
    expect(error).toContain('overview')
    expect(error).toContain('does not have')
  })

  it('rejects unsupported message kinds, a foreign version and more than one surface', () => {
    const one = createSurface(validComponents())
    expect(errorOf(viewFile([one, { version: '1.0', deleteSurface: { surfaceId: 'view' } }]))).toContain('not permitted')

    const older = { ...(one as Record<string, unknown>), version: '0.9' }
    expect(errorOf(viewFile([older]))).toContain('does not match expected target version')

    expect(errorOf(viewFile([createSurface(validComponents(), 'one'), createSurface(validComponents(), 'two')])))
      .toContain('exactly one surface')
  })
})
