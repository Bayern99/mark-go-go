export function DocumentDiff({ before, after }: { before: string; after: string }) {
  const original = before.split('\n')
  const proposed = after.split('\n')
  let start = 0
  let oldEnd = original.length
  let newEnd = proposed.length
  while (start < oldEnd && start < newEnd && original[start] === proposed[start]) start++
  while (oldEnd > start && newEnd > start && original[oldEnd - 1] === proposed[newEnd - 1]) {
    oldEnd--
    newEnd--
  }
  if (before === after) return <p>No changes.</p>
  return (
    <div className="document-diff" aria-label="Markdown changes">
      <p>Changes from line {start + 1}</p>
      {oldEnd > start && <pre aria-label="Removed lines">{original.slice(start, oldEnd).map(line => '− ' + line).join('\n')}</pre>}
      {newEnd > start && <pre aria-label="Proposed lines">{proposed.slice(start, newEnd).map(line => '+ ' + line).join('\n')}</pre>}
    </div>
  )
}
