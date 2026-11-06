import type { ReactElement } from 'react'
import type { ChecklistBlock, GenUIBlock, KeyValueTableBlock, SummaryBlock } from './schema'

// The whitelist: the only components a view file can show. Every value is
// rendered as React text, so Markdown, HTML and scripts stay inert.

function Summary({ block }: { block: SummaryBlock }): ReactElement {
  return (
    <article className="genui-block">
      <h4>{block.title}</h4>
      <ul className="genui-summary">
        {block.items.map((item, index) => (
          <li key={index}>{item.text}</li>
        ))}
      </ul>
    </article>
  )
}

function KeyValueTable({ block }: { block: KeyValueTableBlock }): ReactElement {
  return (
    <article className="genui-block">
      <h4>{block.title}</h4>
      <table className="genui-table">
        <tbody>
          {block.rows.map((row, index) => (
            <tr key={index}>
              <th scope="row">{row.key}</th>
              <td>{row.value}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </article>
  )
}

function Checklist({ block }: { block: ChecklistBlock }): ReactElement {
  return (
    <article className="genui-block">
      <h4>{block.title}</h4>
      <ul className="genui-checklist">
        {block.items.map((item, index) => (
          <li key={index}>
            <label>
              <input type="checkbox" checked={item.checked} readOnly />
              <span>{item.label}</span>
            </label>
          </li>
        ))}
      </ul>
    </article>
  )
}

export function renderBlock(block: GenUIBlock): ReactElement {
  switch (block.component) {
    case 'Summary':
      return <Summary block={block} />
    case 'KeyValueTable':
      return <KeyValueTable block={block} />
    case 'Checklist':
      return <Checklist block={block} />
  }
}
