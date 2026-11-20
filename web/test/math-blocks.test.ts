import '../src/styles.css'
import { undo as undoRich } from '@milkdown/kit/prose/history'
import { userEvent } from 'vitest/browser'
import { describe, expect, it, vi } from 'vitest'
import { mountEditor, type MountedEditor } from './helpers/editor'

const VALID = [
  '# Equations',
  '',
  '$$',
  '\\int_0^\\infty e^{-x^2}\\,dx = \\frac{\\sqrt{\\pi}}{2}',
  '$$',
  '',
  'After.',
  '',
].join('\n')

function undo(mounted: MountedEditor): void {
  const view = mounted.editor.getView()
  undoRich(view.state, view.dispatch)
}

async function openComposer(mounted: MountedEditor, block: Element): Promise<HTMLElement> {
  const edit = block.querySelector<HTMLButtonElement>('[data-block-action="edit"]')
  if (!edit) throw new Error('no Edit action')
  await userEvent.click(edit)
  return vi.waitFor(
    () => {
      const composer = mounted.root.querySelector<HTMLElement>('.composer.mgo-block-composer[role="region"]')
      if (!composer) throw new Error('no Markdown composer')
      return composer
    },
    { timeout: 5_000 },
  )
}

describe('KaTeX block math in Preview', () => {
  it('renders valid block math with KaTeX display mode without altering Markdown bytes', async () => {
    const mounted = await mountEditor(VALID)
    try {
      const block = mounted.root.querySelector<HTMLElement>('.mgo-math[data-math="rendered"]')
      if (!block) throw new Error('rendered math block missing')
      const render = block.querySelector<HTMLElement>('.mgo-math-render')
      expect(render?.checkVisibility() ?? false).toBe(true)
      expect(render?.querySelector('.katex-display .katex')).not.toBeNull()
      expect(render?.querySelector('.katex-mathml math')?.getAttribute('display')).toBe('block')
      expect(render?.querySelector('.katex-error')).toBeNull()
      const rect = render?.querySelector('.katex')?.getBoundingClientRect()
      expect(rect?.width ?? 0).toBeGreaterThan(20)
      expect(rect?.height ?? 0).toBeGreaterThan(10)
      expect(block.querySelector<HTMLElement>('.mgo-math-source')?.checkVisibility() ?? false).toBe(false)
      expect(mounted.editor.getMarkdown()).toBe(VALID)
    } finally {
      await mounted.cleanup()
    }
  })

  it('falls back gracefully on syntax errors while keeping raw source accessible', async () => {
    const invalid = ['Intro.', '', '$$', '\\frac{1}{', '$$', '', 'Outro.', ''].join('\n')
    const mounted = await mountEditor(invalid)
    try {
      const block = mounted.root.querySelector<HTMLElement>('.mgo-math[data-math="error"]')
      if (!block) throw new Error('error math block missing')
      const alert = block.querySelector<HTMLElement>('.mgo-math-error[role="alert"]')
      expect(alert?.checkVisibility() ?? false).toBe(true)
      expect(alert?.textContent).toMatch(/could not render/i)
      const source = block.querySelector<HTMLElement>('.mgo-math-source')
      expect(source?.checkVisibility() ?? false).toBe(true)
      expect(source?.textContent).toContain('\\frac{1}{')
      expect(block.querySelector('.mgo-math-render')?.checkVisibility() ?? false).toBe(false)
      expect(mounted.editor.getMarkdown()).toBe(invalid)
    } finally {
      await mounted.cleanup()
    }
  })

  it('supports in-place Markdown editing with CRLF/CJK byte preservation, undo, and mode switch', async () => {
    const source = '# 公式\r\n\r\n$$\r\nE = mc^2\r\n$$\n\nEnd 🎉\n'
    const mounted = await mountEditor(source)
    try {
      const block = mounted.root.querySelector<HTMLElement>('.mgo-math[data-math="rendered"]')
      if (!block) throw new Error('math block missing')
      const composer = await openComposer(mounted, block)
      const input = composer.querySelector<HTMLTextAreaElement>('textarea[aria-label="Markdown"]')
      if (!input) throw new Error('no Markdown textarea')
      expect(input.value.replace(/\r\n?/g, '\n')).toBe('$$\nE = mc^2\n$$')

      await userEvent.fill(input, '$$\r\na^2 + b^2 = c^2\r\n$$')
      const save = Array.from(composer.querySelectorAll<HTMLButtonElement>('.composer-foot button')).find(
        (button) => button.textContent === 'Save',
      )
      if (!save) throw new Error('no Save button')
      await userEvent.click(save)
      await vi.waitFor(() => expect(mounted.root.querySelector('.mgo-block-composer')).toBeNull())

      const expected = '# 公式\r\n\r\n$$\r\na^2 + b^2 = c^2\r\n$$\n\nEnd 🎉\n'
      expect(mounted.editor.getMarkdown()).toBe(expected)
      expect(mounted.root.querySelector('.mgo-math[data-math="rendered"] .katex')).not.toBeNull()

      mounted.editor.setMode('source')
      expect(mounted.editor.getMarkdown()).toBe(expected)
      mounted.editor.setMode('preview')
      expect(mounted.editor.getMarkdown()).toBe(expected)

      undo(mounted)
      expect(mounted.editor.getMarkdown()).toBe(source)
      expect(mounted.root.querySelector('.mgo-math[data-math="rendered"] .katex-mathml')?.textContent).toContain('E')
    } finally {
      await mounted.cleanup()
    }
  })

  it('hides the edit button when the document is read-only', async () => {
    const mounted = await mountEditor(VALID)
    try {
      mounted.editor.setReadonly(true)
      const edit = mounted.root.querySelector<HTMLElement>('.mgo-math [data-block-action="edit"]')
      expect(edit?.checkVisibility() ?? false).toBe(false)
      edit?.click()
      expect(mounted.root.querySelector('.mgo-block-composer')).toBeNull()
      expect(mounted.editor.getMarkdown()).toBe(VALID)
    } finally {
      await mounted.cleanup()
    }
  })
})
