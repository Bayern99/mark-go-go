import { describe, expect, it } from 'vitest'
import { normalizeHeadingText } from '../src/markdown/headings'
import { extractSections, sectionHash } from '../src/markdown/sections'
import vectors from './fixtures/section-vectors.json'

describe('section text and hash (shared vectors)', () => {
  it('hashes text like FNV-1a 32 over UTF-8', () => {
    for (const { text, hash } of vectors.hashes) expect(sectionHash(text)).toBe(hash)
  })

  it('normalizes heading text for matching', () => {
    for (const { text, key } of vectors.normalize) expect(normalizeHeadingText(text)).toBe(key)
  })

  it('splits a document into heading sections with occurrences, offsets and hashes', () => {
    const sections = extractSections(vectors.document).map((section) => ({
      heading: section.heading.text,
      occurrence: section.occurrence,
      start: section.start,
      end: section.end,
      text: vectors.document.slice(section.start, section.end),
      hash: section.hash,
    }))
    expect(sections).toEqual(vectors.sections)
  })
})
