import type { PluginContext } from '../../core/types'
import { extractHeadings, normalizeHeadingText } from '../../markdown/headings'
import { renderBlock } from './catalog'
import type { GenUIDocument, SourceRef } from './schema'

export function GenUIPanel({
  ctx,
  document,
}: {
  ctx: PluginContext
  document: GenUIDocument
}): React.ReactElement {
  const revealSource = (source: SourceRef) => {
    const wanted = normalizeHeadingText(source.heading)
    if (wanted === '') return
    // Resolve the current offset only when a chip is used; the panel itself
    // does not re-render on document changes.
    const match = extractHeadings(ctx.documents.snapshot().text).find(
      (heading) => normalizeHeadingText(heading.text) === wanted,
    )
    if (match) void ctx.commands.execute('editor.reveal', match.offset)
  }

  return (
    <div className="genui-panel">
      <header className="genui-header">
        <h2>{document.title}</h2>
        <p>Generated from {document.blocks.length} components</p>
      </header>

      {document.blocks.map((block, index) => (
        <div key={index}>{renderBlock(block, revealSource)}</div>
      ))}
    </div>
  )
}
