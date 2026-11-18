import type { Mermaid, MermaidConfig } from 'mermaid'
import { parseCodeFence } from './present'
import './mermaid.css'

const SVG_NS = 'http://www.w3.org/2000/svg'

const FORBIDDEN_ELEMENTS: Record<string, true> = {
  script: true,
  foreignobject: true,
  iframe: true,
  object: true,
  embed: true,
  image: true,
  audio: true,
  video: true,
  link: true,
  meta: true,
  base: true,
  form: true,
  input: true,
  textarea: true,
  button: true,
}

const UNSAFE_CSS = /@import|expression\s*\(|javascript:|url\(\s*(?!['"]?#)/i

const PAINT_ATTRIBUTES: Record<string, true> = {
  fill: true,
  stroke: true,
  filter: true,
  'clip-path': true,
  mask: true,
  'marker-start': true,
  'marker-mid': true,
  'marker-end': true,
}

interface MermaidRun {
  run: number
}

const runs = new WeakMap<HTMLElement, MermaidRun>()

let mermaidPromise: Promise<Mermaid> | undefined
let renderChain: Promise<void> = Promise.resolve()
let renderSerial = 0

function renderSerially<T>(task: () => Promise<T>): Promise<T> {
  const result = renderChain.then(task, task)
  renderChain = result.then(
    () => undefined,
    () => undefined,
  )
  return result
}

function cssToken(name: string, fallback: string): string {
  const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return value || fallback
}

function loadMermaid(): Promise<Mermaid> {
  if (!mermaidPromise) {
    mermaidPromise = import('mermaid')
      .then(({ default: mermaid }) => {
        const config: MermaidConfig = {
          startOnLoad: false,
          securityLevel: 'strict',
          suppressErrorRendering: true,
          htmlLabels: false,
          theme: 'neutral',
          fontFamily: cssToken('--font-sans', 'sans-serif'),
          maxTextSize: 50_000,
          flowchart: { htmlLabels: false, useMaxWidth: true, curve: 'basis' },
          sequence: { useMaxWidth: true },
          gantt: { useMaxWidth: true },
        }
        mermaid.initialize(config)
        return mermaid
      })
      .catch((error: unknown) => {
        mermaidPromise = undefined
        throw error
      })
  }
  return mermaidPromise
}

function errorMessage(error: unknown): string {
  const raw = error instanceof Error ? error.message : String(error)
  return raw.replace(/\s+/g, ' ').trim().slice(0, 320) || 'Unknown Mermaid error.'
}

function sanitizeSvg(markup: string): SVGSVGElement {
  const parsed = new DOMParser().parseFromString(markup, 'image/svg+xml')
  if (parsed.querySelector('parsererror')) throw new Error('Mermaid returned malformed SVG.')
  const root = parsed.documentElement
  if (root.namespaceURI !== SVG_NS || root.localName !== 'svg') {
    throw new Error('Mermaid did not return an SVG diagram.')
  }

  const elements = [root, ...Array.from(root.querySelectorAll('*'))]
  for (const element of elements) {
    if (!element.parentNode && element !== root) continue
    if (FORBIDDEN_ELEMENTS[element.localName.toLowerCase()]) {
      element.remove()
      continue
    }
    if (element.localName.toLowerCase() === 'style' && UNSAFE_CSS.test(element.textContent ?? '')) {
      element.remove()
      continue
    }
    for (const attribute of Array.from(element.attributes)) {
      const name = attribute.name.toLowerCase()
      const value = attribute.value
      const unsafe =
        name.startsWith('on') ||
        name === 'src' ||
        name === 'action' ||
        name === 'formaction' ||
        name === 'srcdoc' ||
        ((name === 'href' || name.endsWith(':href')) && !value.trim().startsWith('#')) ||
        (name === 'style' && UNSAFE_CSS.test(value)) ||
        (PAINT_ATTRIBUTES[name] === true && /url\(\s*(?!['"]?#)/i.test(value))
      if (unsafe) element.removeAttributeNode(attribute)
    }
  }

  const svg = document.importNode(root, true)
  if (!(svg instanceof SVGSVGElement)) throw new Error('Mermaid did not return an SVG diagram.')
  return svg
}

function actionButton(): HTMLButtonElement {
  const button = document.createElement('button')
  button.type = 'button'
  button.className = 'pill xs'
  button.dataset.blockAction = 'edit'
  button.textContent = 'Edit'
  return button
}

export function renderMermaidBlock(dom: HTMLElement, source: string, readonly: boolean): void {
  const state = runs.get(dom) ?? { run: 0 }
  state.run += 1
  const run = state.run
  runs.set(dom, state)

  dom.className = 'mgo-mermaid'
  dom.dataset.mermaid = 'pending'
  dom.setAttribute('aria-busy', 'true')
  dom.removeAttribute('aria-label')
  dom.replaceChildren()

  const chrome = document.createElement('div')
  chrome.className = 'mgo-mermaid-chrome'
  const label = document.createElement('span')
  label.className = 'mgo-code-lang'
  label.textContent = 'Mermaid'
  const status = document.createElement('span')
  status.className = 'mgo-mermaid-status'
  status.textContent = 'Rendering…'
  chrome.append(label, status)

  const view = document.createElement('div')
  view.className = 'mgo-mermaid-view'
  view.setAttribute('role', 'img')
  view.setAttribute('aria-label', 'Mermaid diagram')

  const error = document.createElement('div')
  error.className = 'mgo-mermaid-error'
  error.setAttribute('role', 'alert')
  error.hidden = true

  const raw = document.createElement('pre')
  raw.className = 'mgo-mermaid-source'
  raw.hidden = true

  const actions = document.createElement('span')
  actions.className = 'mgo-block-actions'
  if (!readonly) actions.append(actionButton())

  dom.append(chrome, view, error, raw, actions)

  const fail = (detail: string) => {
    if (state.run !== run) return
    dom.dataset.mermaid = 'error'
    dom.removeAttribute('aria-busy')
    dom.setAttribute('aria-label', 'Mermaid diagram could not be rendered')
    status.hidden = true
    view.hidden = true
    error.hidden = false
    error.textContent = `Mermaid could not render this diagram. ${detail}`
    raw.hidden = false
    raw.textContent = source
  }

  const code = parseCodeFence(source)?.code ?? ''
  if (code.trim() === '') {
    fail('The diagram source is empty.')
    return
  }
  const diagram = code
    .replace(/%%\{[\s\S]*?\}%%/g, '')
    .replace(/^\s*---\r?\n[\s\S]*?\r?\n---\r?\n/, '')
  if (diagram.trim() === '') {
    fail('The diagram has no content besides Mermaid configuration.')
    return
  }
  if (
    /<img\b|!\[[^\]]*\]\(|@\{[^}]*\b(?:img|icon)\s*:|\b(?:href|src)\s*=|url\(\s*(?!['"]?#)/i.test(
      diagram,
    )
  ) {
    fail('This diagram references an external resource. Markdown Preview renders diagrams offline.')
    return
  }

  void renderSerially(async () => {
    await document.fonts?.ready
    const mermaid = await loadMermaid()
    await mermaid.parse(diagram)
    const id = `mgo-mermaid-${++renderSerial}`
    const host = document.createElement('div')
    host.setAttribute('aria-hidden', 'true')
    host.style.cssText =
      'position:fixed;left:-10000px;top:0;width:1200px;visibility:hidden;pointer-events:none;'
    document.body.append(host)
    try {
      return await mermaid.render(id, diagram, host)
    } finally {
      host.remove()
      document.getElementById(`d${id}`)?.remove()
    }
  })
    .then((result) => {
      if (state.run !== run) return
      const svg = sanitizeSvg(result.svg)
      if (state.run !== run) return
      view.replaceChildren(svg)
      view.hidden = false
      dom.dataset.mermaid = 'rendered'
      dom.removeAttribute('aria-busy')
      dom.setAttribute('aria-label', 'Mermaid diagram')
      status.hidden = true
    })
    .catch((error: unknown) => fail(errorMessage(error)))
}

export function disposeMermaidBlock(dom: HTMLElement): void {
  const state = runs.get(dom)
  if (!state) return
  state.run += 1
  runs.delete(dom)
}
