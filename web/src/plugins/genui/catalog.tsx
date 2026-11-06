import type { ReactElement } from 'react'
import type {
  ChecklistBlock,
  GenUIBlock,
  KeyValueTableBlock,
  SourceRef,
  SummaryBlock,
  UnknownBlock,
} from './schema'

export interface BlockProps<B extends GenUIBlock> {
  block: B
  onReveal(source: SourceRef): void
}

function Summary({ block, onReveal }: BlockProps<SummaryBlock>): ReactElement {
  return (
    <article className="genui-block">
      <h3>{block.title}</h3>
      <ul className="genui-summary">
        {block.items.map((item, index) => (
          <li key={index}>
            <span>{item.text}</span>
            <SourceButton source={item.source} onReveal={onReveal} />
          </li>
        ))}
      </ul>
    </article>
  )
}

function KeyValueTable({ block, onReveal }: BlockProps<KeyValueTableBlock>): ReactElement {
  return (
    <article className="genui-block">
      <h3>{block.title}</h3>
      <table className="genui-table">
        <tbody>
          {block.rows.map((row, index) => (
            <tr key={index}>
              <th scope="row">{row.key}</th>
              <td>{row.value}</td>
              <td className="genui-table-source">
                <SourceButton source={row.source} onReveal={onReveal} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </article>
  )
}

function Checklist({ block, onReveal }: BlockProps<ChecklistBlock>): ReactElement {
  return (
    <article className="genui-block">
      <h3>{block.title}</h3>
      <ul className="genui-checklist">
        {block.items.map((item, index) => (
          <li key={index}>
            <label>
              <input type="checkbox" checked={item.checked} readOnly />
              <span>{item.label}</span>
            </label>
            <SourceButton source={item.source} onReveal={onReveal} />
          </li>
        ))}
      </ul>
    </article>
  )
}

function SourceButton({ source, onReveal }: { source?: SourceRef; onReveal(source: SourceRef): void }): ReactElement | null {
  if (!source) return null
  return (
    <button
      type="button"
      className="genui-source"
      title={`Reveal “${source.heading}”`}
      onClick={() => onReveal(source)}
    >
      {source.heading}
    </button>
  )
}

function Unknown({ block }: BlockProps<UnknownBlock>): ReactElement {
  return (
    <article className="genui-block genui-error" role="alert">
      <h3>Unsupported component</h3>
      <p>
        <code>{block.component}</code> is not in the whitelist, so it was not rendered.
      </p>
    </article>
  )
}

type CatalogEntry = (props: BlockProps<GenUIBlock>) => ReactElement

const catalog: Record<string, CatalogEntry> = {
  Summary: (props) => Summary(props as BlockProps<SummaryBlock>),
  KeyValueTable: (props) => KeyValueTable(props as BlockProps<KeyValueTableBlock>),
  Checklist: (props) => Checklist(props as BlockProps<ChecklistBlock>),
}

export function renderBlock(block: GenUIBlock, onReveal: (source: SourceRef) => void): ReactElement {
  const entry = Object.hasOwn(catalog, block.component) ? catalog[block.component] : undefined
  if (entry) {
    return entry({ block, onReveal })
  }
  return <Unknown block={block as UnknownBlock} onReveal={onReveal} />
}
