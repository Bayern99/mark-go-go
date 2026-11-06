import '../src/styles.css'
import { userEvent } from 'vitest/browser'
import { describe, expect, it, vi } from 'vitest'
import { createBrowserHost } from '../src/host/browserHost'
import type { HostAdapter, HostDocSnapshot } from '../src/host/types'
import { mountApp } from './helpers/app'
import { caretLineInSource, sourceLines, visibleSource } from './helpers/source'

const DOCUMENT = `---
title: Draft
tags: [one]
---

# Heading

> [!NOTE]
> Keep this note.

Paragraph one.

Paragraph two.
`

const SOURCE_ONLY_DOCUMENT = `# Overview

## Metrics

Body text.
`

function longDocument(): string {
  const paragraphs = Array.from({ length: 300 }, (_, index) => `Paragraph ${index}.`)
  return `# Long\n\n${paragraphs.join('\n\n')}\n`
}

function modeToggle(container: HTMLElement): HTMLButtonElement {
  const button = container.querySelector<HTMLButtonElement>('[data-testid="mode-toggle"]')
  if (!button) throw new Error('mode toggle missing')
  return button
}

function sourceLine(container: HTMLElement, text: string): HTMLElement {
  const line = sourceLines(container).find((candidate) => candidate.textContent === text)
  if (!line) throw new Error(`source line not found: ${text}`)
  return line
}

function caretBlock(container: HTMLElement): HTMLElement | undefined {
  const anchor = window.getSelection()?.anchorNode
  const element = anchor instanceof Element ? anchor : anchor?.parentElement
  const block = element?.closest<HTMLElement>('p, h1, h2, h3, h4, h5, h6, li, blockquote')
  return block && container.querySelector('.ProseMirror')?.contains(block) ? block : undefined
}

function isWithin(element: HTMLElement, scroller: HTMLElement): boolean {
  const rect = element.getBoundingClientRect()
  const viewport = scroller.getBoundingClientRect()
  const visible = Math.min(rect.bottom, viewport.bottom) - Math.max(rect.top, viewport.top)
  return visible >= Math.min(rect.height, 10)
}

async function enterSourceMode(container: HTMLElement): Promise<void> {
  await userEvent.click(modeToggle(container))
  await vi.waitFor(() => {
    expect(modeToggle(container).getAttribute('aria-pressed')).toBe('true')
    expect(visibleSource(container)).not.toBe('')
  })
}

async function toggleShortcut(modifier: 'Meta' | 'Control'): Promise<void> {
  await userEvent.keyboard(`{${modifier}>}/{/${modifier}}`)
}

/** Replace one visible source line through real keyboard input. */
async function replaceSourceLine(
  container: HTMLElement,
  oldLine: string,
  newLine: string,
): Promise<void> {
  await userEvent.click(sourceLine(container, oldLine))
  await userEvent.keyboard('{Home}{Shift>}{End}{/Shift}')
  await userEvent.keyboard(newLine)
  await vi.waitFor(() => {
    expect(sourceLines(container).some((line) => line.textContent === newLine)).toBe(true)
  })
}

describe('App source mode', () => {
  it('toggles with the toolbar control and shows the current mode', async () => {
    const app = await mountApp(DOCUMENT)
    try {
      expect(modeToggle(app.container).textContent).toBe('Rich text')
      expect(modeToggle(app.container).getAttribute('aria-pressed')).toBe('false')

      await userEvent.click(modeToggle(app.container))
      await vi.waitFor(() => {
        expect(modeToggle(app.container).textContent).toBe('Source')
        expect(modeToggle(app.container).getAttribute('aria-pressed')).toBe('true')
        expect(visibleSource(app.container)).toBe(DOCUMENT)
      })
      expect(app.container.querySelector<HTMLElement>('.mgo-source-root')?.hidden).toBe(false)
      expect(app.container.querySelector<HTMLElement>('.mgo-wysiwyg-root')?.hidden).toBe(true)
    } finally {
      await app.unmount()
    }
  })

  it('toggles with ⌘/ and Ctrl+/', async () => {
    const app = await mountApp(DOCUMENT)
    try {
      await toggleShortcut('Meta')
      await vi.waitFor(() => {
        expect(modeToggle(app.container).textContent).toBe('Source')
        expect(visibleSource(app.container)).toBe(DOCUMENT)
      })

      await toggleShortcut('Control')
      await vi.waitFor(() => {
        expect(modeToggle(app.container).textContent).toBe('Rich text')
        expect(app.container.querySelector<HTMLElement>('.mgo-wysiwyg-root')?.hidden).toBe(false)
        expect(app.container.querySelector<HTMLElement>('.mgo-source-root')?.hidden).toBe(true)
      })
    } finally {
      await app.unmount()
    }
  })

  it('entering and leaving without edits never updates the host', async () => {
    const app = await mountApp(DOCUMENT)
    const update = vi.spyOn(app.host, 'update')
    try {
      await enterSourceMode(app.container)
      expect(visibleSource(app.container)).toBe(DOCUMENT)

      await userEvent.click(modeToggle(app.container))
      await vi.waitFor(() => {
        expect(modeToggle(app.container).getAttribute('aria-pressed')).toBe('false')
      })
      await new Promise((resolve) => setTimeout(resolve, 350))
      expect(update).not.toHaveBeenCalled()
    } finally {
      await app.unmount()
    }
  })

  it('saves source edits verbatim and keeps them through later WYSIWYG edits', async () => {
    const app = await mountApp(DOCUMENT)
    const update = vi.spyOn(app.host, 'update')
    try {
      await enterSourceMode(app.container)
      await replaceSourceLine(app.container, 'title: Draft', 'title: Final')
      await replaceSourceLine(app.container, '> [!NOTE]', '> Note')

      const expected = DOCUMENT.replace('title: Draft', 'title: Final').replace(
        '> [!NOTE]',
        '> Note',
      )
      expect(visibleSource(app.container)).toBe(expected)

      await vi.waitFor(() => expect(update).toHaveBeenLastCalledWith(expected), {
        timeout: 3000,
      })

      await userEvent.click(modeToggle(app.container))
      await vi.waitFor(() => {
        expect(modeToggle(app.container).getAttribute('aria-pressed')).toBe('false')
        expect(app.container.querySelector('[data-testid="editor-root"] .ProseMirror')).not.toBeNull()
        expect(app.container.querySelector('[data-testid="editor-root"] blockquote')).not.toBeNull()
      })
      const locked = Array.from(
        app.container.querySelectorAll<HTMLElement>('[data-testid="editor-root"] .mgo-locked-block'),
      )
      expect(locked.some((element) => element.textContent?.includes('[!NOTE]'))).toBe(false)

      const paragraph = Array.from(
        app.container.querySelectorAll<HTMLElement>('[data-testid="editor-root"] .ProseMirror p'),
      ).find((element) => element.textContent === 'Paragraph two.')
      expect(paragraph).toBeDefined()
      await userEvent.click(paragraph as HTMLElement)
      await userEvent.keyboard('{End} Edited.')
      expect(paragraph?.textContent).toContain('Paragraph two. Edited.')

      await vi.waitFor(
        () => {
          const last = update.mock.lastCall?.[0]
          expect(last).not.toBe(expected)
          expect(last).toContain('title: Final')
          expect(last).toContain('> Note\n> Keep this note.')
        },
        { timeout: 3000 },
      )
    } finally {
      await app.unmount()
    }
  })

  it('undoes source edits with ⌘Z and saves the reverted text', async () => {
    const app = await mountApp(DOCUMENT)
    const update = vi.spyOn(app.host, 'update')
    try {
      await enterSourceMode(app.container)
      await userEvent.click(sourceLine(app.container, 'Paragraph one.'))
      await userEvent.keyboard('{End}!')
      await vi.waitFor(() => expect(visibleSource(app.container)).toContain('Paragraph one.!'))
      await vi.waitFor(
        () => expect(update.mock.lastCall?.[0]).toContain('Paragraph one.!'),
        { timeout: 3000 },
      )

      await userEvent.keyboard('{Meta>}z{/Meta}')
      await vi.waitFor(() => expect(visibleSource(app.container)).toBe(DOCUMENT))
      await vi.waitFor(() => expect(update).toHaveBeenLastCalledWith(DOCUMENT), {
        timeout: 3000,
      })
    } finally {
      await app.unmount()
    }
  })

  it('reloads external changes in source mode without changing mode', async () => {
    const host = createBrowserHost(DOCUMENT)
    const app = await mountApp(DOCUMENT, host)
    try {
      await enterSourceMode(app.container)
      const reloaded = DOCUMENT.replace('Paragraph two.', 'Paragraph two, externally changed.')
      host.reload(reloaded)

      await vi.waitFor(() => {
        expect(visibleSource(app.container)).toBe(reloaded)
        expect(modeToggle(app.container).textContent).toBe('Source')
        expect(modeToggle(app.container).getAttribute('aria-pressed')).toBe('true')
      })
    } finally {
      await app.unmount()
    }
  })

  it('ignores typing when the document is read-only', async () => {
    const base = createBrowserHost(DOCUMENT)
    const host: HostAdapter = {
      ...base,
      async load(): Promise<HostDocSnapshot> {
        const snapshot = await base.load()
        return { ...snapshot, state: { ...snapshot.state, readOnly: true } }
      },
    }
    const app = await mountApp(DOCUMENT, host)
    const update = vi.spyOn(app.host, 'update')
    try {
      await vi.waitFor(() => expect(app.container.querySelector('.banner-readonly')).not.toBeNull())
      await enterSourceMode(app.container)

      await userEvent.click(sourceLine(app.container, 'Paragraph one.'))
      await userEvent.keyboard('X')
      expect(visibleSource(app.container)).toBe(DOCUMENT)
      await new Promise((resolve) => setTimeout(resolve, 350))
      expect(update).not.toHaveBeenCalled()
    } finally {
      await app.unmount()
    }
  })

  it('jumps to a GenUI heading while in source mode', async () => {
    const app = await mountApp(SOURCE_ONLY_DOCUMENT)
    try {
      await enterSourceMode(app.container)
      const chip = Array.from(app.container.querySelectorAll<HTMLButtonElement>('.genui-source')).find(
        (button) => button.textContent === 'Metrics',
      )
      expect(chip).toBeDefined()
      await userEvent.click(chip as HTMLButtonElement)

      await vi.waitFor(() => {
        expect(caretLineInSource(app.container)?.textContent).toBe('## Metrics')
        expect(document.activeElement?.closest('.cm-editor')).not.toBeNull()
      })
    } finally {
      await app.unmount()
    }
  })

  it('keeps the caret block visible when switching modes near the end', async () => {
    const source = longDocument()
    const app = await mountApp(source)
    try {
      const scroller = app.container.querySelector<HTMLElement>('.editor-scroll')
      expect(scroller).not.toBeNull()
      const last = Array.from(
        app.container.querySelectorAll<HTMLElement>('[data-testid="editor-root"] .ProseMirror p'),
      ).find((paragraph) => paragraph.textContent === 'Paragraph 299.')
      expect(last).toBeDefined()

      await userEvent.click(last as HTMLElement)
      expect(caretBlock(app.container)).toBe(last)
      expect(isWithin(last as HTMLElement, scroller as HTMLElement)).toBe(true)

      ;(scroller as HTMLElement).scrollTop = 0
      expect(isWithin(last as HTMLElement, scroller as HTMLElement)).toBe(false)

      await userEvent.click(modeToggle(app.container))
      await vi.waitFor(() => {
        expect(modeToggle(app.container).getAttribute('aria-pressed')).toBe('true')
        const line = caretLineInSource(app.container)
        expect(line?.textContent).toBe('Paragraph 299.')
        expect(isWithin(line as HTMLElement, scroller as HTMLElement)).toBe(true)
      })

      ;(scroller as HTMLElement).scrollTop = 0
      await userEvent.click(modeToggle(app.container))
      await vi.waitFor(() => {
        expect(modeToggle(app.container).getAttribute('aria-pressed')).toBe('false')
        const block = caretBlock(app.container)
        expect(block?.textContent).toBe('Paragraph 299.')
        expect(isWithin(block as HTMLElement, scroller as HTMLElement)).toBe(true)
      })
    } finally {
      await app.unmount()
    }
  })
})
