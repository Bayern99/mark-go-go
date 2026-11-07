import { undo as undoRich } from '@milkdown/kit/prose/history'
import { userEvent } from 'vitest/browser'
import { describe, expect, it, vi } from 'vitest'
import { mountEditor, type MountedEditor } from './helpers/editor'
import corpus from './fixtures/special-blocks.md?raw'

function undo(mounted: MountedEditor): void {
  const view = mounted.editor.getView()
  undoRich(view.state, view.dispatch)
}

async function openBlockComposer(mounted: MountedEditor, locked: HTMLElement): Promise<HTMLElement> {
  const edit = locked.querySelector<HTMLButtonElement>('[data-block-action="edit"]')
  if (!edit) throw new Error('no Edit action')
  await userEvent.click(edit)
  return vi.waitFor(() => {
    expect(document.querySelector('dialog')).toBeNull()
    const composer = mounted.root.querySelector<HTMLElement>(
      '.ProseMirror .composer.mgo-block-composer[role="region"]',
    )
    if (!composer) throw new Error('no Markdown composer')
    return composer
  })
}

async function saveComposer(composer: HTMLElement, commit: 'click' | 'shortcut' = 'click'): Promise<void> {
  const input = composer.querySelector<HTMLTextAreaElement>('textarea[aria-label="Markdown"]')
  const save = Array.from(composer.querySelectorAll<HTMLButtonElement>('.composer-foot button')).find(
    (button) => button.textContent === 'Save',
  )
  if (!input || !save) throw new Error('incomplete Markdown composer')
  if (commit === 'shortcut') {
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', metaKey: true, bubbles: true, cancelable: true }))
  } else {
    await userEvent.click(save)
  }
}

describe('special blocks in rich text', () => {
  it('shows frontmatter keys, alert type, footnotes, code meta, and kept HTML', async () => {
    const mounted = await mountEditor(corpus)
    try {
      const fm = mounted.root.querySelector('.mgo-frontmatter')
      expect(fm?.textContent).toContain('title')
      expect(fm?.textContent).toContain('测试文档')
      expect(fm?.textContent).toContain('unknown')

      const alert = mounted.root.querySelector('.mgo-alert.warning')
      expect(alert?.querySelector('.mgo-alert-k')?.textContent).toMatch(/Warning/i)
      expect(alert?.textContent).toContain('Stay locked')
      expect(alert?.textContent).not.toContain('[!WARNING]')

      const refs = mounted.root.querySelectorAll<HTMLAnchorElement>('.mgo-fnref')
      expect(Array.from(refs).map((el) => el.dataset.footnoteRef)).toEqual(['1', 'note'])
      const defs = mounted.root.querySelectorAll('.mgo-footnote')
      expect(defs[0]?.textContent).toContain('First footnote')
      expect(defs[1]?.textContent).toContain('Second footnote')

      const code = mounted.root.querySelector('.mgo-code-block')
      expect(code?.querySelector('.mgo-code-lang')?.textContent).toMatch(/python/i)
      expect(code?.querySelector('.mgo-code-meta')?.textContent).toContain('title="demo.py"')
      expect(code?.textContent).toContain('return "~~~ inside"')

      const html = mounted.root.querySelector('[data-kind="raw-html"]')
      expect(html?.classList.contains('mgo-locked-block')).toBe(true)
      expect(html?.textContent).toContain('<div class="callout">')
      expect(html?.querySelector('.callout')).toBeNull()

      expect(mounted.editor.getMarkdown()).toBe(corpus)
    } finally {
      await mounted.cleanup()
    }
  })

  it('edits an alert in place through Composer and keeps neighbouring bytes', async () => {
    const source = 'Intro.\n\n> [!WARNING]\n> stay locked\n\nOutro.\n'
    const mounted = await mountEditor(source)
    const changes: string[] = []
    mounted.editor.onChange((text) => changes.push(text))
    try {
      const alert = mounted.root.querySelector<HTMLElement>('.mgo-alert')
      if (!alert) throw new Error('no alert')
      const composer = await openBlockComposer(mounted, alert)
      expect(alert.nextElementSibling).toBe(composer)
      const input = composer.querySelector<HTMLTextAreaElement>('textarea[aria-label="Markdown"]')
      if (!input) throw new Error('no Markdown textarea')
      expect(input.value).toBe('> [!WARNING]\n> stay locked')
      expect(mounted.editor.getMarkdown()).toBe(source)
      expect(changes).toEqual([])

      await userEvent.fill(input, '> [!NOTE]\n> now a note 中文 💡')
      await saveComposer(composer, 'shortcut')
      await vi.waitFor(() => expect(mounted.root.querySelector('.mgo-block-composer')).toBeNull())

      const expected = 'Intro.\n\n> [!NOTE]\n> now a note 中文 💡\n\nOutro.\n'
      expect(mounted.editor.getMarkdown()).toBe(expected)
      expect(mounted.root.querySelector('.mgo-alert.note')?.textContent).toContain('now a note')
      expect(mounted.root.querySelector('.mgo-alert.warning')).toBeNull()

      undo(mounted)
      expect(mounted.editor.getMarkdown()).toBe(source)
      expect(mounted.root.querySelector('.mgo-alert.warning')).not.toBeNull()
    } finally {
      await mounted.cleanup()
    }
  })

  it('cancel and identical save leave the document clean', async () => {
    const source = 'A.\r\n\r\n> [!TIP]\n> keep\r\n\r\nB.\n'
    const mounted = await mountEditor(source)
    const changes: string[] = []
    mounted.editor.onChange((text) => changes.push(text))
    try {
      const alert = mounted.root.querySelector<HTMLElement>('.mgo-alert')
      if (!alert) throw new Error('no alert')
      const composer = await openBlockComposer(mounted, alert)
      const input = composer.querySelector<HTMLTextAreaElement>('textarea[aria-label="Markdown"]')
      if (!input) throw new Error('no Markdown textarea')
      await userEvent.fill(input, '> [!TIP]\n> rewritten')
      input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }))
      await vi.waitFor(() => expect(mounted.root.querySelector('.mgo-block-composer')).toBeNull())
      expect(mounted.editor.getMarkdown()).toBe(source)

      const again = await openBlockComposer(mounted, mounted.root.querySelector<HTMLElement>('.mgo-alert')!)
      await saveComposer(again, 'click')
      await vi.waitFor(() => expect(mounted.root.querySelector('.mgo-block-composer')).toBeNull())
      expect(mounted.editor.getMarkdown()).toBe(source)
      expect(changes).toEqual([])
    } finally {
      await mounted.cleanup()
    }
  })

  it('does not open Composer on a read-only document', async () => {
    const mounted = await mountEditor('> [!CAUTION]\n> no edit\n')
    try {
      mounted.editor.setReadonly(true)
      const alert = mounted.root.querySelector<HTMLElement>('.mgo-alert')
      expect(alert?.querySelector('[data-block-action="edit"]')).toBeNull()
      expect(mounted.editor.getMarkdown()).toBe('> [!CAUTION]\n> no edit\n')
    } finally {
      await mounted.cleanup()
    }
  })

  it('hops between a footnote mark and its definition', async () => {
    const source = 'See this[^1].\n\n[^1]: Definition body.\n'
    const mounted = await mountEditor(source)
    try {
      const ref = mounted.root.querySelector<HTMLAnchorElement>('.mgo-fnref')
      const def = mounted.root.querySelector<HTMLElement>('.mgo-footnote')
      if (!ref || !def) throw new Error('footnote missing')
      await userEvent.click(ref)
      expect(def.id).toBe(ref.getAttribute('href')?.slice(1))
      const back = def.querySelector<HTMLAnchorElement>('[data-footnote-back]')
      if (!back) throw new Error('no back link')
      await userEvent.click(back)
      expect(ref.id).toBe(back.getAttribute('href')?.slice(1))
    } finally {
      await mounted.cleanup()
    }
  })

  it('keeps mixed newlines and CJK bytes around an in-place fence edit', async () => {
    const source = '# 中文\r\n\r\n~~~js title="a.js"\r\nconst n = 1\r\n~~~\n\nEnd 🎉'
    const mounted = await mountEditor(source)
    try {
      const block = mounted.root.querySelector<HTMLElement>('.mgo-code-block')
      if (!block) throw new Error('no code block')
      const composer = await openBlockComposer(mounted, block)
      const input = composer.querySelector<HTMLTextAreaElement>('textarea[aria-label="Markdown"]')
      if (!input) throw new Error('no Markdown textarea')
      await userEvent.fill(input, '~~~js title="a.js"\r\nconst n = 2\r\n~~~')
      await saveComposer(composer)
      await vi.waitFor(() => expect(mounted.root.querySelector('.mgo-block-composer')).toBeNull())
      expect(mounted.editor.getMarkdown()).toBe('# 中文\r\n\r\n~~~js title="a.js"\r\nconst n = 2\r\n~~~\n\nEnd 🎉')
    } finally {
      await mounted.cleanup()
    }
  })

  it('keeps an in-place edit across Markdown mode and undo in Rich text', async () => {
    const source = 'Intro.\n\n> [!NOTE]\n> keep\n\nOutro.\n'
    const mounted = await mountEditor(source)
    try {
      const alert = mounted.root.querySelector<HTMLElement>('.mgo-alert')
      if (!alert) throw new Error('no alert')
      const composer = await openBlockComposer(mounted, alert)
      const input = composer.querySelector<HTMLTextAreaElement>('textarea[aria-label="Markdown"]')
      if (!input) throw new Error('no Markdown textarea')
      await userEvent.fill(input, '> [!CAUTION]\n> rewritten')
      await saveComposer(composer)
      await vi.waitFor(() => expect(mounted.root.querySelector('.mgo-block-composer')).toBeNull())
      const edited = mounted.editor.getMarkdown()

      mounted.editor.setMode('source')
      expect(mounted.editor.getMarkdown()).toBe(edited)
      mounted.editor.setMode('wysiwyg')
      expect(mounted.root.querySelector('.mgo-alert.caution')?.textContent).toContain('rewritten')
      expect(mounted.editor.getMarkdown()).toBe(edited)

      undo(mounted)
      expect(mounted.editor.getMarkdown()).toBe(source)
    } finally {
      await mounted.cleanup()
    }
  })
})
