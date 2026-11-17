// Dev-only measurement harness for editor typesetting research.
// Not part of the production build (vite only builds index.html).
// Usage: /bench/typeset.html?doc=paper-bilingual|tech-report-tables|llm-research-report
//
// The figure referenced by paper-bilingual.md is embedded as a base64 data
// URL below so the editor does not depend on static-file serving for it.
import { createRoot } from 'react-dom/client'
import { EditorView } from '@milkdown/kit/prose/view'
import { App } from '../src/App'
import { createBrowserHost, createBrowserWorkspace } from '../src/host/browserHost'
import '../src/assets/fonts.css'
import '../src/assets/icons/phosphor.css'
import '../src/styles.css'
// @ts-ignore — ?raw imports of files outside web/src
import paperBilingual from '../../docs/editor-typeset/fixtures/paper-bilingual.md?raw'
// @ts-ignore
import paperEnglish from '../../docs/editor-typeset/fixtures/paper-english.md?raw'
// @ts-ignore
import paperChinese from '../../docs/editor-typeset/fixtures/paper-chinese.md?raw'
// @ts-ignore
import techReportTables from '../../docs/editor-typeset/fixtures/tech-report-tables.md?raw'
// @ts-ignore
import llmResearchReport from '../../docs/editor-typeset/fixtures/llm-research-report.md?raw'
// @ts-ignore
import figureSvg from '../../docs/editor-typeset/fixtures/figure-1.svg?raw'

// Table variants: ?variant=<name> injects docs/.../variants/<name>.css last.
const VARIANTS = import.meta.glob('../../docs/editor-typeset/variants/*.css', {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>

const DOCS: Record<string, string> = {
  'paper-bilingual': paperBilingual,
  'paper-english': paperEnglish,
  'paper-chinese': paperChinese,
  'tech-report-tables': techReportTables,
  'llm-research-report': llmResearchReport,
}

const params = new URLSearchParams(window.location.search)
const docName = params.get('doc') ?? 'paper-bilingual'
const variantName = params.get('variant') ?? ''
const variantCss = variantName
  ? VARIANTS[`../../docs/editor-typeset/variants/${variantName}.css`]
  : undefined
if (variantCss) {
  const style = document.createElement('style')
  style.dataset.variant = variantName
  style.textContent = variantCss
  document.head.append(style)
}
const raw = DOCS[docName] ?? paperBilingual
const figureDataUrl = `data:image/svg+xml;base64,${window.btoa(figureSvg)}`
const text = raw.replaceAll('figure-1.svg', figureDataUrl)

const host = createBrowserHost(text, `${docName}.md`)
const workspace = createBrowserWorkspace([host])

// Measurement hook for docs/research/editor-typeset/measure.mjs. `fixture` is
// the exact byte string the editor was seeded with (image src rewritten);
// `source` is the untouched fixture file content.
;(window as unknown as { __bench: unknown }).__bench = {
  doc: docName,
  variant: variantCss ? variantName : null,
  host,
  workspace,
  fixture: text,
  source: raw,
}

// Expose the live ProseMirror EditorView for the measurement script
// (.ProseMirror.pmViewDesc carries no .view back-reference in this PM version).
const origUpdateStateInner = EditorView.prototype.updateStateInner
EditorView.prototype.updateStateInner = function (this: EditorView, ...args) {
  ;(window as unknown as { __pmView?: EditorView }).__pmView = this
  return origUpdateStateInner.apply(this, args as Parameters<typeof origUpdateStateInner>)
}

const container = document.getElementById('root')
if (!container) throw new Error('#root is missing from typeset.html')
createRoot(container).render(<App workspace={workspace} />)
