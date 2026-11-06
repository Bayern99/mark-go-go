import { describe, expect, it } from 'vitest'
import { extractHeadings, normalizeHeadingText } from '../src/markdown/headings'

describe('extractHeadings', () => {
  it('includes ATX and setext headings with their levels and source offsets', () => {
    const markdown = '# Alpha\n\nBeta\n====\n\nGamma\n-----\n'
    const headings = extractHeadings(markdown)

    expect(headings.map((heading) => [heading.level, heading.text])).toEqual([
      [1, 'Alpha'],
      [1, 'Beta'],
      [2, 'Gamma'],
    ])
    expect(headings.map((heading) => heading.offset)).toEqual([
      0,
      markdown.indexOf('Beta'),
      markdown.indexOf('Gamma'),
    ])
  })

  it('ignores headings inside frontmatter, fenced code and raw HTML', () => {
    const markdown = [
      '---',
      'title: "# Not a heading"',
      '---',
      '',
      '```md',
      '# Not a heading either',
      '```',
      '',
      '<div>',
      '# Still not a heading',
      '</div>',
      '',
      '# Real',
      '',
    ].join('\n')

    expect(extractHeadings(markdown)).toEqual([
      { level: 1, text: 'Real', offset: markdown.indexOf('# Real') },
    ])
  })

  it('strips inline formatting from heading text', () => {
    const markdown = '## **Bold** and *italic* `code` [link](https://example.com)\n'

    expect(extractHeadings(markdown)).toEqual([
      { level: 2, text: 'Bold and italic code link', offset: 0 },
    ])
  })

  it('drops raw inline HTML from heading text but keeps adjacent text', () => {
    expect(extractHeadings('## Title <span>x</span>\n')).toEqual([
      { level: 2, text: 'Title x', offset: 0 },
    ])
  })

  it('normalizes heading text for comparisons', () => {
    expect(normalizeHeadingText('  Next   steps ')).toBe('next steps')
  })
})
