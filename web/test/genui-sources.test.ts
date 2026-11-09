import { describe, expect, it } from 'vitest'
import { extractSections } from '../src/markdown/sections'
import { SourceStore } from '../src/plugins/genui/sources'
import type { SourceRef } from '../src/plugins/genui/viewFile'

const DOC = `# Overview

First body. The quote lives here.

# Summary

Early summary.

# Metrics

Second body.

# Summary

Late summary.
`

function sectionRange(
  text: string,
  heading: string,
  occurrence = 1,
): { start: number; end: number } {
  const section = extractSections(text).find(
    (candidate) => candidate.heading.text === heading && candidate.occurrence === occurrence,
  )
  if (!section) throw new Error(`no section ${heading} #${occurrence}`)
  return { start: section.start, end: section.end }
}

function quoteRange(text: string, quote: string): { start: number; end: number } {
  const start = text.indexOf(quote)
  if (start < 0) throw new Error(`no quote ${quote}`)
  return { start, end: start + quote.length }
}

function stamped(markdown: string): Record<string, SourceRef> {
  const at = (heading: string, occurrence = 1): SourceRef => {
    const section = extractSections(markdown).find(
      (candidate) => candidate.heading.text === heading && candidate.occurrence === occurrence,
    )
    if (!section) throw new Error(`no section ${heading} #${occurrence}`)
    return { heading, occurrence, hash: section.hash }
  }
  return {
    overview: at('Overview'),
    metrics: at('Metrics'),
    late: at('Summary', 2),
  }
}

describe('SourceStore availability', () => {
  const table = stamped(DOC)
  const mixed = ['overview', 'metrics'] as const

  it('keeps aggregate missing when the first source is gone, and still targets the rest', () => {
    const text = DOC.replace('# Overview', '# Intro')
    const store = new SourceStore(table, text)
    expect(store.status(mixed)).toBe('missing')
    expect(store.hasSource(mixed)).toBe(true)
    expect(store.target(mixed)).toEqual(sectionRange(text, 'Metrics'))
    expect(store.status(['overview'])).toBe('missing')
    expect(store.hasSource(['overview'])).toBe(false)
    expect(store.target(['overview'])).toBeUndefined()
  })

  it('keeps aggregate missing when a later source is gone, and still targets the first that remains', () => {
    const text = DOC.replace('# Metrics', '# Figures')
    const store = new SourceStore(table, text)
    expect(store.status(mixed)).toBe('missing')
    expect(store.hasSource(mixed)).toBe(true)
    expect(store.target(mixed)).toEqual(sectionRange(text, 'Overview'))
  })

  it('has no target only when every listed source is gone', () => {
    const text = DOC.replace('# Overview', '# Intro').replace('# Metrics', '# Figures')
    const store = new SourceStore(table, text)
    expect(store.status(mixed)).toBe('missing')
    expect(store.hasSource(mixed)).toBe(false)
    expect(store.target(mixed)).toBeUndefined()
    expect(store.target(mixed, 'The quote lives here.')).toBeUndefined()
  })

  it('selects an exact quote in a remaining source, else the first remaining section', () => {
    const text = DOC.replace('# Overview', '# Intro')
    const store = new SourceStore(table, text)
    expect(store.target(mixed, 'Second body.')).toEqual(quoteRange(text, 'Second body.'))
    expect(store.target(mixed, 'The quote lives here.')).toEqual(sectionRange(text, 'Metrics'))
    expect(store.target(['metrics'], 'The quote lives here.')).toEqual(sectionRange(text, 'Metrics'))
  })

  it('selects the quote in the first remaining source that still holds it', () => {
    const store = new SourceStore(table, DOC)
    expect(store.target(mixed, 'The quote lives here.')).toEqual(
      quoteRange(DOC, 'The quote lives here.'),
    )
    expect(store.target(['late', 'overview'], 'Late summary.')).toEqual(
      quoteRange(DOC, 'Late summary.'),
    )
  })

  it('does not treat another heading of the same text as a missing occurrence', () => {
    const text = DOC.replace('# Summary\n\nLate summary.\n', '')
    const store = new SourceStore(table, text)
    expect(store.status(['late'])).toBe('missing')
    expect(store.hasSource(['late'])).toBe(false)
    expect(store.target(['late'])).toBeUndefined()
    expect(store.target(['late'], 'Early summary.')).toBeUndefined()
    expect(store.status(['late', 'overview'])).toBe('missing')
    expect(store.hasSource(['late', 'overview'])).toBe(true)
    expect(store.target(['late', 'overview'])).toEqual(sectionRange(text, 'Overview'))
    // The quote lives under the other Summary occurrence, not in a remaining
    // source, so the target falls back to the first remaining section.
    expect(store.target(['late', 'overview'], 'Early summary.')).toEqual(
      sectionRange(text, 'Overview'),
    )
  })

  it('targets a recovered source after it returns', () => {
    const store = new SourceStore(table, DOC)
    store.schedule(DOC.replace('# Overview', '# Intro').replace('# Metrics', '# Figures'))
    expect(store.target(mixed)).toBeUndefined()
    expect(store.status(mixed)).toBe('missing')
    expect(store.hasSource(mixed)).toBe(false)

    store.schedule(DOC)
    expect(store.target(mixed)).toEqual(sectionRange(DOC, 'Overview'))
    expect(store.status(mixed)).toBe('fresh')
    expect(store.hasSource(mixed)).toBe(true)
    expect(store.target(['late'], 'Late summary.')).toEqual(quoteRange(DOC, 'Late summary.'))
  })
})
