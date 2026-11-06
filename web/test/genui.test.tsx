import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'
import { renderBlock } from '../src/plugins/genui/catalog'
import { parseGenUIDocument } from '../src/plugins/genui/schema'

describe('GenUI schema', () => {
  it('turns unknown components into inert errors', () => {
    const parsed = parseGenUIDocument({
      title: 'Fixture',
      blocks: [{ component: 'DangerousWidget', run: 'alert(1)' }],
    })
    expect(parsed.blocks[0]?.component).toBe('DangerousWidget')

    const host = document.createElement('div')
    document.body.append(host)
    const root = createRoot(host)
    flushSync(() => {
      root.render(<div>{renderBlock(parsed.blocks[0], () => {})}</div>)
    })

    expect(host.textContent).toContain('DangerousWidget')
    expect(host.textContent).toContain('not in the whitelist')
    expect(host.querySelector('script')).toBeNull()

    root.unmount()
    host.remove()
  })

  it('parses known components and drops malformed props', () => {
    const parsed = parseGenUIDocument({
      blocks: [
        { component: 'Summary', title: 'S', items: [{ text: 'ok' }, { nope: true }] },
        { component: 'Checklist', items: [{ label: 'done', checked: true }, { checked: true }] },
      ],
    })
    const summary = parsed.blocks[0]
    const checklist = parsed.blocks[1]
    expect(summary?.component).toBe('Summary')
    expect('items' in summary && summary.items).toHaveLength(1)
    expect('items' in checklist && checklist.items).toHaveLength(1)
  })
})

