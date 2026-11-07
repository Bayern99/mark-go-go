import '../src/styles.css'
import { page, userEvent } from 'vitest/browser'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createBrowserHost } from '../src/host/browserHost'
import type { HostAdapter } from '../src/host/types'
import { extractSections } from '../src/markdown/sections'
import { SOURCE_DEBOUNCE_MS } from '../src/plugins/genui/sources'
import { mountApp, type MountedApp } from './helpers/app'
import { visibleSource } from './helpers/source'

/** Uneven gaps and line ends: adding a note must leave all of them as they are. */
const DOCUMENT = `# Overview

First body.


# Metrics\r
\r
Second body.

## Detail

Detail body.

# Next steps

Third body.`

/** A view of the document with the hashes the host would record. */
function viewOf(markdown: string): string {
  const sections = extractSections(markdown)
  const ids = ['overview', 'metrics', 'steps']
  const headings = ['Overview', 'Metrics', 'Next steps']
  return JSON.stringify({
    version: 2,
    sources: Object.fromEntries(
      ids.map((id, index) => [
        id,
        {
          heading: headings[index],
          occurrence: 1,
          hash: sections.find((section) => section.heading.text === headings[index])?.hash,
        },
      ]),
    ),
    a2ui: [
      {
        version: '1.0',
        createSurface: {
          surfaceId: 'view',
          catalogId: 'markgogo.dev:genui/v1',
          components: [
            { id: 'root', component: 'Column', children: ids },
            ...ids.map((id) => ({
              id,
              component: 'KeyPoints',
              title: id,
              items: [`Point of ${id}.`],
              sources: [id],
            })),
          ],
        },
      },
    ],
  })
}

function blockOf(container: HTMLElement, title: string): HTMLElement {
  const heading = Array.from(container.querySelectorAll<HTMLElement>('.genui-keypoints h4')).find(
    (candidate) => candidate.textContent === title,
  )
  const block = heading?.closest<HTMLElement>('.genui-keypoints')
  if (!block) throw new Error(`no component ${title}`)
  return block
}

function notesOf(block: HTMLElement): string[] {
  return Array.from(block.querySelectorAll('.genui-note')).map((note) => note.textContent ?? '')
}

function statuses(container: HTMLElement): string[] {
  return Array.from(container.querySelectorAll('.genui-source-status')).map((status) => status.textContent ?? '')
}

async function addNote(
  container: HTMLElement,
  title: string,
  text: string,
  commit: 'click' | 'shortcut' = 'click',
): Promise<void> {
  const card = blockOf(container, title)
  const button = await vi.waitFor(() => {
    const found = card.querySelector<HTMLButtonElement>('.genui-source-note')
    if (!found) throw new Error('no Note button')
    return found
  })
  await userEvent.click(button)
  const composer = await vi.waitFor(() => {
    expect(document.querySelector('dialog')).toBeNull()
    const found = card.querySelector<HTMLElement>(
      '.composer.mgo-note-composer[role="region"][aria-label="Note composer"]',
    )
    if (!found) throw new Error('no inline note composer on card')
    return found
  })
  const input = composer.querySelector<HTMLTextAreaElement>('textarea[aria-label="Note"]')
  const save = Array.from(composer.querySelectorAll<HTMLButtonElement>('.composer-foot button')).find(
    (b) => b.textContent === 'Save',
  )
  const hint = composer.querySelector('.composer-foot .composer-hint')
  if (!input || !save || !hint) throw new Error('incomplete inline note composer')
  expect(hint.textContent).toContain('⌘↩ to save · Esc to cancel')
  await userEvent.fill(input, text)
  if (commit === 'shortcut') {
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', metaKey: true, bubbles: true, cancelable: true }))
  } else {
    await userEvent.click(save)
  }
  await vi.waitFor(() => expect(card.querySelector('.mgo-note-composer')).toBeNull())
}

const waitLong = { timeout: SOURCE_DEBOUNCE_MS + 3000 }

let app: MountedApp | undefined

afterEach(async () => {
  await app?.unmount()
  app = undefined
})

async function open(host: HostAdapter, text = DOCUMENT): Promise<MountedApp> {
  app = await mountApp(text, host)
  await page.getByTestId('view-mode-genui').click()
  await vi.waitFor(() => {
    if (!app?.container.querySelector('.genui-keypoints')) throw new Error('no view yet')
  })
  return app
}

describe('notes from a GenUI component', () => {
  const WITH_NOTE = `# Overview

First body.


# Metrics\r
\r
Second body.

## Detail

Detail body.

<!-- note: Check this number -->

# Next steps

Third body.`

  it('writes the note at the end of the source section, shows it, and marks nothing changed', async () => {
    const host = createBrowserHost(DOCUMENT, 'report.md', { view: viewOf(DOCUMENT) })
    const update = vi.spyOn(host, 'update')
    const { container } = await open(host)

    await addNote(container, 'metrics', 'Check this number')

    // The end of the section is after its sub-section; everything else keeps its bytes.
    await vi.waitFor(() => expect(update).toHaveBeenLastCalledWith(WITH_NOTE), waitLong)
    await vi.waitFor(() => expect(notesOf(blockOf(container, 'metrics'))).toEqual(['Check this number']), waitLong)
    expect(notesOf(blockOf(container, 'overview'))).toEqual([])
    expect(statuses(container)).toEqual([])
    // Still in GenUI: adding a note does not navigate.
    await expect.element(page.getByTestId('view-mode-genui')).toHaveAttribute('aria-checked', 'true')
  })

  it('writes the same bytes when the hidden editor is in Markdown mode, and shows them raw there', async () => {
    const host = createBrowserHost(DOCUMENT, 'report.md', { view: viewOf(DOCUMENT) })
    const update = vi.spyOn(host, 'update')
    app = await mountApp(DOCUMENT, host)
    await page.getByTestId('view-mode-markdown').click()
    await page.getByTestId('view-mode-genui').click()
    await vi.waitFor(() => {
      if (!app?.container.querySelector('.genui-keypoints')) throw new Error('no view yet')
    })

    await addNote(app.container, 'metrics', 'Check this number', 'shortcut')
    await vi.waitFor(() => expect(update).toHaveBeenLastCalledWith(WITH_NOTE), waitLong)
    expect(statuses(app.container)).toEqual([])

    await page.getByTestId('view-mode-markdown').click()
    await vi.waitFor(() => expect(visibleSource(app?.container as HTMLElement)).toContain('<!-- note: Check this number -->'))
  })

  it('closes the in-place card composer without writing when Escape or Cancel is used', async () => {
    const host = createBrowserHost(DOCUMENT, 'report.md', { view: viewOf(DOCUMENT) })
    const update = vi.spyOn(host, 'update')
    const { container } = await open(host)
    const card = blockOf(container, 'metrics')
    const button = card.querySelector<HTMLButtonElement>('.genui-source-note')
    if (!button) throw new Error('no Note button')

    await userEvent.click(button)
    const composer = await vi.waitFor(() => {
      const found = card.querySelector<HTMLElement>('.composer.mgo-note-composer')
      if (!found) throw new Error('no inline note composer')
      return found
    })
    const input = composer.querySelector<HTMLTextAreaElement>('textarea[aria-label="Note"]')
    if (!input) throw new Error('no Note textarea')
    await userEvent.fill(input, 'Draft to discard\nSecond line\nThird line')
    expect(Number.parseFloat(input.style.height)).toBeGreaterThanOrEqual(40)
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }))
    await vi.waitFor(() => expect(card.querySelector('.mgo-note-composer')).toBeNull())

    await userEvent.click(button)
    const secondComposer = await vi.waitFor(() => {
      const found = card.querySelector<HTMLElement>('.composer.mgo-note-composer')
      if (!found) throw new Error('no inline note composer')
      return found
    })
    const cancelBtn = Array.from(secondComposer.querySelectorAll('button')).find((b) => b.textContent === 'Cancel')
    if (!cancelBtn) throw new Error('no Cancel button')
    await userEvent.click(cancelBtn)
    await vi.waitFor(() => expect(card.querySelector('.mgo-note-composer')).toBeNull())
    expect(update).not.toHaveBeenCalled()
    expect(notesOf(card)).toEqual([])
  })

  it('shows notes already in the source sections, and changing them changes no status', async () => {
    const noted = DOCUMENT.replace('First body.', 'First body.\n\n<!-- note author="a": One -->\n\n<!-- note: Two -->')
    const host = createBrowserHost(noted, 'report.md', { view: viewOf(DOCUMENT) })
    const { container } = await open(host, noted)

    await vi.waitFor(() => expect(notesOf(blockOf(container, 'overview'))).toEqual(['One', 'Two']))
    expect(statuses(container)).toEqual([])

    host.reload(DOCUMENT)
    await vi.waitFor(() => expect(notesOf(blockOf(container, 'overview'))).toEqual([]), waitLong)
    expect(statuses(container)).toEqual([])

    host.reload(DOCUMENT.replace('First body.', 'First body, edited.'))
    await vi.waitFor(() => expect(statuses(container)).toEqual(['Source changed']), waitLong)
  })

  it('says so when the document is read only', async () => {
    const base = createBrowserHost(DOCUMENT, 'report.md', { view: viewOf(DOCUMENT) })
    const host: HostAdapter = {
      ...base,
      async load() {
        const snapshot = await base.load()
        return { ...snapshot, state: { ...snapshot.state, readOnly: true } }
      },
    }
    const { container } = await open(host)

    await addNote(container, 'metrics', 'Nope')

    await vi.waitFor(() =>
      expect(blockOf(container, 'metrics').querySelector('[role="alert"]')?.textContent).toContain('read only'),
    )
    expect(notesOf(blockOf(container, 'metrics'))).toEqual([])
  })

  it('tells the host which ranges are notes when generating', async () => {
    const host = createBrowserHost(WITH_NOTE, 'report.md', {
      view: viewOf(WITH_NOTE),
      generator: () => new Promise<string>(() => {}),
    })
    await open(host, WITH_NOTE)
    await page.getByTestId('right-tab-genui.panel').click()

    await page.getByRole('button', { name: 'Regenerate all' }).click()
    await vi.waitFor(() => expect(host.generations.length).toBe(1))

    const noteAt = WITH_NOTE.indexOf('<!-- note')
    const noteEnd = noteAt + '<!-- note: Check this number -->'.length
    const gapAt = noteAt - '\n\n'.length
    const metrics = host.generations[0]?.sections.find((section) => section.heading === 'Metrics')
    expect(metrics?.exclude).toEqual([{ start: gapAt, end: noteEnd }])
    expect(host.generations[0]?.sections.find((section) => section.heading === 'Overview')?.exclude).toBeUndefined()
  })
})
