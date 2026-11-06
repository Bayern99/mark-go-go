import { EditorState } from '@codemirror/state'
import { undo } from '@codemirror/commands'
import { userEvent } from 'vitest/browser'
import { describe, expect, it, vi } from 'vitest'
import { createBrowserHost } from '../src/host/browserHost'
import type { HostAdapter, HostDocSnapshot } from '../src/host/types'
import { mountApp } from './helpers/app'
import { getSourceView, replaceSourceText, sourceText } from './helpers/source'

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

function modeToggle(container: HTMLElement): HTMLButtonElement {
  const button = container.querySelector<HTMLButtonElement>('[data-testid="mode-toggle"]')
  if (!button) throw new Error('mode toggle missing')
  return button
}

async function enterSourceMode(container: HTMLElement): Promise<void> {
  modeToggle(container).click()
  await vi.waitFor(() => {
    expect(modeToggle(container).getAttribute('aria-pressed')).toBe('true')
    expect(sourceText(container)).not.toBe('')
  })
}

function pressToggleShortcut(init: KeyboardEventInit): void {
  window.dispatchEvent(new KeyboardEvent('keydown', { key: '/', bubbles: true, ...init }))
}

describe('App source mode', () => {
  it('toggles with the toolbar control and shows the current mode', async () => {
    const app = await mountApp(DOCUMENT)
    try {
      expect(modeToggle(app.container).textContent).toBe('Rich text')
      expect(modeToggle(app.container).getAttribute('aria-pressed')).toBe('false')

      modeToggle(app.container).click()
      await vi.waitFor(() => {
        expect(modeToggle(app.container).textContent).toBe('Source')
        expect(modeToggle(app.container).getAttribute('aria-pressed')).toBe('true')
        expect(sourceText(app.container)).toBe(DOCUMENT)
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
      pressToggleShortcut({ metaKey: true })
      await vi.waitFor(() => {
        expect(modeToggle(app.container).textContent).toBe('Source')
        expect(sourceText(app.container)).toBe(DOCUMENT)
      })

      pressToggleShortcut({ ctrlKey: true })
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
      expect(sourceText(app.container)).toBe(DOCUMENT)

      modeToggle(app.container).click()
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
      const view = getSourceView(app.container)
      const afterTitle = replaceSourceText(view, 'title: Draft', 'title: Final')
      const expected = replaceSourceText(
        view,
        '[!NOTE]',
        'Note',
      )
      expect(expected).toBe(afterTitle.replace('[!NOTE]', 'Note'))

      await vi.waitFor(() => expect(update).toHaveBeenLastCalledWith(expected), {
        timeout: 3000,
      })

      modeToggle(app.container).click()
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

  it('undoes source edits and saves the reverted text', async () => {
    const app = await mountApp(DOCUMENT)
    const update = vi.spyOn(app.host, 'update')
    try {
      await enterSourceMode(app.container)
      const view = getSourceView(app.container)
      replaceSourceText(view, 'Paragraph one.', 'Paragraph one, changed.')

      await vi.waitFor(
        () => expect(update.mock.lastCall?.[0]).toContain('Paragraph one, changed.'),
        { timeout: 3000 },
      )

      undo(view)
      expect(sourceText(app.container)).toBe(DOCUMENT)
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
        expect(sourceText(app.container)).toBe(reloaded)
        expect(modeToggle(app.container).textContent).toBe('Source')
        expect(modeToggle(app.container).getAttribute('aria-pressed')).toBe('true')
      })
    } finally {
      await app.unmount()
    }
  })

  it('makes the source editor read-only when the document is read-only', async () => {
    const base = createBrowserHost(DOCUMENT)
    const host: HostAdapter = {
      ...base,
      async load(): Promise<HostDocSnapshot> {
        const snapshot = await base.load()
        return { ...snapshot, state: { ...snapshot.state, readOnly: true } }
      },
    }
    const app = await mountApp(DOCUMENT, host)
    try {
      await vi.waitFor(() => expect(app.container.querySelector('.banner-readonly')).not.toBeNull())
      await enterSourceMode(app.container)
      const view = getSourceView(app.container)
      expect(view.state.facet(EditorState.readOnly)).toBe(true)
      expect(view.contentDOM.getAttribute('contenteditable')).toBe('false')
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
      chip?.click()

      await vi.waitFor(() => {
        const view = getSourceView(app.container)
        expect(view.state.selection.main.head).toBe(SOURCE_ONLY_DOCUMENT.indexOf('## Metrics'))
      })
    } finally {
      await app.unmount()
    }
  })
})
