import { createRoot } from 'react-dom/client'
import { isMyGo } from 'mygo-runtime'
import { App } from './App'
import { createBrowserHost, createBrowserWorkspace } from './host/browserHost'
import { createMyGoHost } from './host/mygoHost'
import './assets/fonts.css'
import './assets/icons/phosphor.css'
import './styles.css'

const workspace = isMyGo() ? createMyGoHost() : createBrowserWorkspace([createBrowserHost()])
const container = document.getElementById('root')
if (!container) throw new Error('#root is missing from index.html')

const prototypeVariant = import.meta.env.DEV
  ? new URLSearchParams(window.location.search).get('variant')
  : null

if (prototypeVariant) {
  const { WorkspaceChromePrototype } = await import('./prototype/WorkspaceChromePrototype')
  const prototypeWorkspace = isMyGo()
    ? workspace
    : createBrowserWorkspace(
        Array.from({ length: 9 }, (_, index) =>
          createBrowserHost(
            `# Prototype document ${index + 1}\n\nA readable excerpt for the navigation prototype.\n`,
            index === 0 ? '03-tasks.md' : `source-${index + 1}.md`,
          ),
        ),
        {
          nav: {
            status: 'ready',
            folders: [
              { path: '/demo/docs', name: 'docs', parent: '/demo', depth: 0, markdownCount: 4 },
              { path: '/demo/docs/adr', name: 'adr', parent: '/demo/docs', depth: 1, markdownCount: 2 },
              { path: '/demo/research', name: 'research', parent: '/demo', depth: 0, markdownCount: 2 },
            ],
            files: [
              { path: '/demo/README.md', name: 'README.md', parent: '/demo', depth: 0, text: '# Readme', title: 'Readme', excerpt: 'Local-first Markdown workspace', notes: 0, modifiedAt: Date.now(), headings: [] },
              { path: '/demo/docs/adr/0001-architecture.md', name: '0001-architecture.md', parent: '/demo/docs/adr', depth: 2, text: '# Architecture', title: 'Architecture baseline', excerpt: '原文是唯一权威来源', notes: 2, modifiedAt: Date.now(), headings: [] },
              { path: '/demo/docs/adr/0002-genui.md', name: '0002-genui.md', parent: '/demo/docs/adr', depth: 2, text: '# GenUI', title: 'GenUI derived view', excerpt: 'GenUI is a read-only derived view', notes: 0, modifiedAt: Date.now(), headings: [] },
              { path: '/demo/research/source.md', name: 'source.md', parent: '/demo/research', depth: 1, text: '# Source', title: 'Source constraints', excerpt: 'Evidence before interface decisions', notes: 0, modifiedAt: Date.now(), headings: [] },
            ],
          },
        },
      )
  createRoot(container).render(
    <WorkspaceChromePrototype workspace={prototypeWorkspace} initialVariant={prototypeVariant} />,
  )
} else {
  createRoot(container).render(<App workspace={workspace} />)
}
