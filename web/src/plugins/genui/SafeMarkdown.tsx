import type { ReactNode } from 'react'
import { parseMdast, type MdastNode } from '../../markdown/mdast'

export function SafeMarkdown({ markdown }: { markdown: string }): React.ReactElement {
  return <span className="genui-markdown">{blocks(parseMdast(markdown).children)}</span>
}

function blocks(nodes: MdastNode[] | undefined): ReactNode[] {
  return (nodes ?? []).map((node, index) => block(node, index))
}

function block(node: MdastNode, key: number): ReactNode {
  switch (node.type) {
    case 'paragraph':
      return <p key={key}>{inline(node.children)}</p>
    case 'heading':
      return (
        <p key={key} className="genui-markdown-heading">
          {inline(node.children)}
        </p>
      )
    case 'list':
      return node.ordered ? (
        <ol key={key}>{blocks(node.children)}</ol>
      ) : (
        <ul key={key}>{blocks(node.children)}</ul>
      )
    case 'listItem':
      return <li key={key}>{listItem(node)}</li>
    case 'blockquote':
      return <blockquote key={key}>{blocks(node.children)}</blockquote>
    case 'code':
      return (
        <pre key={key}>
          <code>{node.value}</code>
        </pre>
      )
    case 'thematicBreak':
      return <hr key={key} />
    default:
      return null
  }
}

function listItem(node: MdastNode): ReactNode {
  const [only] = node.children ?? []
  if (node.children?.length === 1 && only?.type === 'paragraph') return inline(only.children)
  return blocks(node.children)
}

function inline(nodes: MdastNode[] | undefined): ReactNode[] {
  return (nodes ?? []).map((node, index) => span(node, index))
}

function span(node: MdastNode, key: number): ReactNode {
  switch (node.type) {
    case 'text':
      return node.value
    case 'strong':
      return <strong key={key}>{inline(node.children)}</strong>
    case 'emphasis':
      return <em key={key}>{inline(node.children)}</em>
    case 'delete':
      return <del key={key}>{inline(node.children)}</del>
    case 'inlineCode':
      return <code key={key}>{node.value}</code>
    case 'break':
      return <br key={key} />
    case 'link':
    case 'linkReference':
      return (
        <span key={key} className="genui-link">
          {inline(node.children)}
        </span>
      )
    default:
      return null
  }
}
