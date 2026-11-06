export interface HeadingEntry {
  level: number
  text: string
}

/** ATX headings that are not inside fenced code blocks. */
export function extractHeadings(markdown: string): HeadingEntry[] {
  const headings: HeadingEntry[] = []
  let fence: string | null = null

  for (const line of markdown.split(/\r?\n/)) {
    const fenceMatch = /^\s*(`{3,}|~{3,})/.exec(line)
    if (fenceMatch) {
      const marker = fenceMatch[1][0]
      if (fence === null) fence = marker
      else if (fence === marker) fence = null
      continue
    }
    if (fence !== null) continue

    const atx = /^(#{1,6})[ \t]+(.*?)[ \t]*#*[ \t]*$/.exec(line)
    if (!atx) continue
    const text = atx[2].trim()
    if (text === '') continue
    headings.push({ level: atx[1].length, text })
  }

  return headings
}
