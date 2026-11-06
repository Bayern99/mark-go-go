import { describe, expect, it } from 'vitest'
import { extractSections, sectionHash } from '../src/markdown/sections'
import vectors from './fixtures/section-vectors.json'

describe('section text and hash (shared vectors)', () => {
  it('hashes text like FNV-1a 32 over UTF-8', () => {
    for (const { text, hash } of vectors.hashes) expect(sectionHash(text)).toBe(hash)
  })

  it('splits a document into heading sections with occurrences and hashes', () => {
    const sections = extractSections(vectors.document).map((section) => ({
      heading: section.heading.text,
      occurrence: section.occurrence,
      text: vectors.document.slice(section.start, section.end),
      hash: section.hash,
    }))
    expect(sections).toEqual(vectors.sections)
  })
})
