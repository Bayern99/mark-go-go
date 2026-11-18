import { useEffect, useRef, useState } from 'react'
import { z } from 'zod'
import {
  A2uiSurface,
  MarkdownComponentContext,
  createComponentImplementation,
  type ReactCatalogComponent,
} from '@a2ui/react'
import { Catalog, MessageProcessor, type SurfaceModel } from '@a2ui/web_core/v1_0'
import { basicCatalog } from '@a2ui/web_core/catalogs/basic/v1'
import { STRICT_VALIDATION } from '@a2ui/web_core/validation'
import { isRecord } from '../../lib/guards'
import { CONTENT_COMPONENTS } from './components'
import { SafeMarkdown } from './SafeMarkdown'
import { SourceContext, type SourceActions } from './SourceFrame'
import { ReadingContext, type ReadingStore } from './reading'

export const CATALOG_ID = 'markgogo.dev:genui/v1'

const UNKNOWN = 'MgoUnknown'
const Unknown = createComponentImplementation(
  {
    name: UNKNOWN,
    schema: z.object({ type: z.string() }).strict(),
    allowedChildren: [],
  },
  ({ props }) => (
    <p className="genui-unknown" role="alert">
      “{props.type}” is not a component this app can show.
    </p>
  ),
)

const CONTENT = CONTENT_COMPONENTS.map((component) => component.name)

function layout(unknown: string[]): ReactCatalogComponent[] {
  const basic = (name: string) => required(basicCatalog.components.get(name), name)
  return [
    basic('Column'),
    { ...basic('Row'), allowedParents: ['Surface', 'Column', 'Tabs', ...unknown] },
    { ...basic('Tabs'), allowedParents: ['Surface', 'Column', ...unknown] },
    { ...basic('Card'), allowedChildren: ['Column', 'Text', ...CONTENT, ...unknown] },
    basic('Text'),
  ]
}

const catalog = new Catalog<ReactCatalogComponent>(
  CATALOG_ID,
  '1.0',
  [...layout([UNKNOWN]), ...CONTENT_COMPONENTS, Unknown],
  Array.from(basicCatalog.functions.values()),
)

export const CATALOG_COMPONENTS: readonly string[] = [...catalog.components.keys()].filter(
  (name) => name !== UNKNOWN,
)
const KNOWN = new Set(CATALOG_COMPONENTS)

export function catalogDefinition(): Record<string, unknown> {
  return new Catalog<ReactCatalogComponent>(CATALOG_ID, '1.0', [...layout([]), ...CONTENT_COMPONENTS], []).toJson()
}

function required<T>(value: T | undefined, name: string): T {
  if (value === undefined) throw new Error(`A2UI basic catalog has no ${name}`)
  return value
}

const GENERATED_MESSAGES = ['createSurface', 'updateComponents']

export type GeneratedViewVerdict = { ok: true } | { ok: false; error: string }

function rejected(error: string): GeneratedViewVerdict {
  return { ok: false, error }
}

export function validateGeneratedView(raw: string): GeneratedViewVerdict {
  let file: unknown
  try {
    file = JSON.parse(raw)
  } catch {
    return rejected('The view is not valid JSON.')
  }
  if (!isRecord(file)) return rejected('The view is not a JSON object.')
  if (file.version !== 2) return rejected('The view is not a version 2 view file.')
  if (!isRecord(file.sources)) return rejected("The view's sources are not an object.")
  const messages = file.a2ui
  if (!Array.isArray(messages) || messages.length === 0) {
    return rejected('The view has no A2UI messages.')
  }
  const unknown = unknownComponent(messages)
  if (unknown) return rejected(unknown)

  const merged = messages.some((message) => isRecord(message) && isRecord(message.updateComponents))
  const processor = new MessageProcessor<ReactCatalogComponent>([catalog], undefined, {
    validationConfig: {
      ...STRICT_VALIDATION,
      allowOrphanComponents: merged,
      allowedMessages: GENERATED_MESSAGES,
      targetVersion: '1.0',
    },
  })
  try {
    processor.processMessages(messages as Parameters<typeof processor.processMessages>[0])
    const surfaces = [...processor.model.surfacesMap.values()]
    if (surfaces.length !== 1) {
      return rejected(`The view must create exactly one surface; it creates ${surfaces.length}.`)
    }
    const missing = missingSource(messages, file.sources)
    if (missing) return rejected(missing)
    return { ok: true }
  } catch (error) {
    return rejected(describe(error))
  } finally {
    processor.dispose()
  }
}

function unknownComponent(messages: unknown[]): string | undefined {
  for (const component of componentsOf(messages)) {
    if (typeof component.component !== 'string') continue
    if (KNOWN.has(component.component)) continue
    const id = typeof component.id === 'string' ? component.id : '(no id)'
    return `Component '${id}' uses '${component.component}', which is not a component this app can show.`
  }
  return undefined
}

function missingSource(messages: unknown[], sources: Record<string, unknown>): string | undefined {
  for (const component of componentsOf(messages)) {
    if (!Array.isArray(component.sources)) continue
    for (const source of component.sources) {
      if (typeof source === 'string' && !Object.hasOwn(sources, source)) {
        const id = typeof component.id === 'string' ? component.id : '(no id)'
        return `Component '${id}' uses the source '${source}', which the view does not have.`
      }
    }
  }
  return undefined
}

function* componentsOf(messages: unknown[]): Generator<Record<string, unknown>> {
  for (const message of messages) {
    if (!isRecord(message)) continue
    for (const key of GENERATED_MESSAGES as Array<'createSurface' | 'updateComponents'>) {
      const body = message[key]
      if (!isRecord(body) || !Array.isArray(body.components)) continue
      for (const component of body.components) {
        if (isRecord(component)) yield component
      }
    }
  }
}

export type LoadedSurface =
  | { ok: true; surface: SurfaceModel<ReactCatalogComponent>; dispose(): void }
  | { ok: false; error: string }

export function loadSurface(messages: unknown[]): LoadedSurface {
  const processor = new MessageProcessor<ReactCatalogComponent>([catalog], undefined, {
    validationConfig: {
      ...STRICT_VALIDATION,
      allowOrphanComponents: true,
      allowedMessages: ['createSurface', 'updateComponents'],
    },
  })
  try {
    const input = messages.map(replaceUnknown) as Parameters<typeof processor.processMessages>[0]
    processor.processMessages(input)
  } catch (error) {
    processor.dispose()
    return { ok: false, error: describe(error) }
  }
  const surfaces = [...processor.model.surfacesMap.values()]
  const surface = surfaces[0]
  if (surfaces.length !== 1 || !surface) {
    processor.dispose()
    return { ok: false, error: `The view must create exactly one surface; it creates ${surfaces.length}.` }
  }
  return { ok: true, surface, dispose: () => processor.dispose() }
}

function replaceUnknown(message: unknown): unknown {
  if (!isRecord(message)) return message
  for (const key of ['createSurface', 'updateComponents'] as const) {
    const body = message[key]
    if (!isRecord(body) || !Array.isArray(body.components)) continue
    return { ...message, [key]: { ...body, components: body.components.map(replaceComponent) } }
  }
  return message
}

function replaceComponent(component: unknown): unknown {
  if (!isRecord(component) || typeof component.component !== 'string') return component
  if (KNOWN.has(component.component)) return component
  return { id: component.id, component: UNKNOWN, type: component.component }
}

function describe(error: unknown): string {
  const message = (error instanceof Error ? error.message : String(error))
    .replace(/^\w*Error: /, '')
    .replaceAll(`,"${UNKNOWN}"`, '')
  const code = isRecord(error) ? error.code : undefined
  return code === 'UNALLOWED_PARENT' || code === 'UNALLOWED_CHILD' ? `${code}: ${message}` : message
}

export function ViewSurface({
  surface,
  sources,
  reading,
}: {
  surface: SurfaceModel<ReactCatalogComponent>
  sources: SourceActions
  reading: ReadingStore
}): React.ReactElement {
  const problems = useSurfaceProblems(surface)
  const root = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const el = root.current
    if (!el) return
    const tabsOf = () => [...el.querySelectorAll<HTMLElement & { activeIndex: number }>('a2ui-tabs')]
    let interacted = false
    const restore = () => {
      const tabs = tabsOf()
      if (tabs.length === 0) return
      observer.disconnect()
      requestAnimationFrame(() => {
        if (interacted || !el.isConnected) return
        for (const [index, tabsElement] of tabsOf().entries()) {
          const saved = reading.tab(`tabs:${index}`)
          if (saved !== undefined) tabsElement.activeIndex = saved
        }
      })
    }
    const observer = new MutationObserver(restore)
    observer.observe(el, { childList: true, subtree: true })
    restore()
    const remember = () => {
      interacted = true
      for (const [index, tabs] of tabsOf().entries()) reading.setTab(`tabs:${index}`, tabs.activeIndex)
    }
    el.addEventListener('click', remember)
    return () => {
      observer.disconnect()
      el.removeEventListener('click', remember)
    }
  }, [surface, reading])
  return (
    <ReadingContext.Provider value={reading}>
      <SourceContext.Provider value={sources}>
        <MarkdownComponentContext.Provider value={SafeMarkdown}>
          {problems.length > 0 ? (
            <ul className="genui-problems" role="alert">
              {problems.map((problem, index) => (
                <li key={index}>{problem}</li>
              ))}
            </ul>
          ) : null}
          <div className="genui-surface" ref={root}>
            <A2uiSurface surface={surface} />
          </div>
        </MarkdownComponentContext.Provider>
      </SourceContext.Provider>
    </ReadingContext.Provider>
  )
}

function useSurfaceProblems(surface: SurfaceModel<ReactCatalogComponent>): string[] {
  const [problems, setProblems] = useState<string[]>([])
  useEffect(() => {
    setProblems([])
    const subscription = surface.onError.subscribe((error) => {
      if (error.code === 'UNKNOWN_COMPONENT_TYPE') return
      setProblems((current) => [...current, error.message])
    })
    return () => subscription.unsubscribe()
  }, [surface])
  return problems
}
