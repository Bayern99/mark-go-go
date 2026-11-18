import '../src/styles.css'
import { undo as undoRich } from '@milkdown/kit/prose/history'
import { userEvent } from 'vitest/browser'
import { describe, expect, it, vi } from 'vitest'
import { mountEditor, type MountedEditor } from './helpers/editor'

const VALID = ['# Diagram', '', '```mermaid', 'graph TD', '  A[Start] --> B{Choice}', '  B -->|yes| C[Done]', '```', '', 'After.', ''].join('\n')

const INVALID = ['Intro.', '', '```mermaid', 'graph TD', '  A[Start --> B', '```', '', 'Outro.', ''].join('\n')

function undo(mounted: MountedEditor): void {
  const view = mounted.editor.getView()
  undoRich(view.state, view.dispatch)
}

async function waitForDiagram(mounted: MountedEditor, text: string): Promise<SVGSVGElement> {
  return vi.waitFor(
    () => {
      const svg = mounted.root.querySelector<SVGSVGElement>('.mgo-mermaid[data-mermaid="rendered"] svg')
      if (!svg) throw new Error('diagram not rendered yet')
      expect(svg.textContent).toContain(text)
      return svg
    },
    { timeout: 15_000 },
  )
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

describe('Mermaid fences in Preview', () => {
  it('renders a valid fence offline without changing its bytes', { timeout: 30_000 }, async () => {
    const mounted = await mountEditor(VALID)
    try {
      const svg = await waitForDiagram(mounted, 'Start')
      expect(svg.textContent).toContain('Choice')
      expect(svg.textContent).toContain('Done')
      expect(svg.querySelector('script, foreignObject, image, iframe')).toBeNull()
      expect(mounted.root.querySelector('.mgo-mermaid-source')?.checkVisibility() ?? false).toBe(false)
      expect(mounted.editor.getMarkdown()).toBe(VALID)
    } finally {
      await mounted.cleanup()
    }
  })

  it('keeps the original source visible with an explicit error for an invalid diagram', { timeout: 30_000 }, async () => {
    const mounted = await mountEditor(INVALID)
    try {
      const block = await vi.waitFor(
        () => {
          const failed = mounted.root.querySelector<HTMLElement>('.mgo-mermaid[data-mermaid="error"]')
          if (!failed) throw new Error('invalid diagram not reported yet')
          return failed
        },
        { timeout: 15_000 },
      )
      const alert = block.querySelector<HTMLElement>('[role="alert"]')
      expect(alert?.textContent).toMatch(/could not render this diagram/i)
      const source = block.querySelector<HTMLElement>('.mgo-mermaid-source')
      expect(source?.checkVisibility() ?? false).toBe(true)
      expect(source?.textContent).toBe('```mermaid\ngraph TD\n  A[Start --> B\n```')
      expect(mounted.editor.getMarkdown()).toBe(INVALID)
    } finally {
      await mounted.cleanup()
    }
  })

  it('edits the fence in place with CRLF preservation, undo, and no neighbouring rewrites', { timeout: 30_000 }, async () => {
    const source = VALID.replace(/\n/g, '\r\n')
    const mounted = await mountEditor(source)
    try {
      const block = mounted.root.querySelector<HTMLElement>('.mgo-mermaid')
      if (!block) throw new Error('no Mermaid block')
      await waitForDiagram(mounted, 'Start')
      const composer = await openComposer(mounted, block)
      const input = composer.querySelector<HTMLTextAreaElement>('textarea[aria-label="Markdown"]')
      if (!input) throw new Error('no Markdown textarea')
      expect(input.value).toBe('```mermaid\ngraph TD\n  A[Start] --> B{Choice}\n  B -->|yes| C[Done]\n```')

      await userEvent.fill(
        input,
        '```mermaid\ngraph TD\n  A[Start] --> B{Choice}\n  B -->|yes| C[Renamed]\n```',
      )
      const save = Array.from(composer.querySelectorAll<HTMLButtonElement>('.composer-foot button')).find(
        (button) => button.textContent === 'Save',
      )
      if (!save) throw new Error('no Save button')
      await userEvent.click(save)
      await vi.waitFor(
        () => expect(mounted.root.querySelector('.mgo-block-composer')).toBeNull(),
        { timeout: 5_000 },
      )
      await waitForDiagram(mounted, 'Renamed')

      const expected = source.replace('C[Done]', 'C[Renamed]')
      expect(mounted.editor.getMarkdown()).toBe(expected)

      undo(mounted)
      expect(mounted.editor.getMarkdown()).toBe(source)
      await waitForDiagram(mounted, 'Done')
    } finally {
      await mounted.cleanup()
    }
  })

  it('refuses diagrams that reference external resources before rendering them', { timeout: 30_000 }, async () => {
    const source = [
      '```mermaid',
      'flowchart TD',
      '  A@{ img: "https://example.com/a.png", label: "A" }',
      '  A --> B',
      '```',
      '',
    ].join('\n')
    const mounted = await mountEditor(source)
    try {
      const block = await vi.waitFor(
        () => {
          const failed = mounted.root.querySelector<HTMLElement>('.mgo-mermaid[data-mermaid="error"]')
          if (!failed) throw new Error('external diagram not refused yet')
          return failed
        },
        { timeout: 5_000 },
      )
      expect(block.querySelector('[role="alert"]')?.textContent).toMatch(/external resource/i)
      expect(block.querySelector('svg')).toBeNull()
      const raw = block.querySelector<HTMLElement>('.mgo-mermaid-source')
      expect(raw?.checkVisibility() ?? false).toBe(true)
      expect(mounted.editor.getMarkdown()).toBe(source)
    } finally {
      await mounted.cleanup()
    }
  })

  it('hides the edit entry point in a read-only document', { timeout: 30_000 }, async () => {
    const mounted = await mountEditor(VALID)
    try {
      await waitForDiagram(mounted, 'Start')
      mounted.editor.setReadonly(true)
      const edit = mounted.root.querySelector<HTMLElement>('.mgo-mermaid [data-block-action="edit"]')
      expect(edit?.checkVisibility() ?? false).toBe(false)
      edit?.click()
      expect(mounted.root.querySelector('.mgo-block-composer')).toBeNull()
      expect(mounted.editor.getMarkdown()).toBe(VALID)
    } finally {
      await mounted.cleanup()
    }
  })
})
