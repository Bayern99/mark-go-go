/**
 * The only module that knows A2UI. It owns the component catalog, turns a
 * view file's messages into one rendered surface, and reports what went
 * wrong. Everything else in GenUI talks to it through `loadSurface`,
 * `ViewSurface` and `catalogDefinition`, so the renderer can be replaced
 * without touching them.
 */
import { useEffect, useState } from 'react'
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

/** The catalog id a view file's surface must name. */
export const CATALOG_ID = 'markgogo.dev:genui/v1'

/**
 * Stands in for a component the catalog does not have, so that one
 * unknown component shows an error where it was and the rest still renders.
 * The view file never names it: `loadSurface` puts it there, and every
 * nesting rule admits it so that it is never refused for where it stands.
 */
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

/**
 * The A2UI basic layout components, with the nesting rules that keep a view
 * readable in one editor-wide pane: tabs only at the top, rows side by side
 * but never inside a row or a card, a card around one piece of content.
 * Content components hold no other component.
 */
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

/** Names a view file may use, in the order the format document lists them. */
export const CATALOG_COMPONENTS: readonly string[] = [...catalog.components.keys()].filter(
  (name) => name !== UNKNOWN,
)
const KNOWN = new Set(CATALOG_COMPONENTS)

/**
 * The catalog as an A2UI 1.0 catalog document, for an agent writing a view:
 * every component a view may use with its JSON Schema and nesting rules.
 * Functions are left out because views are static.
 */
export function catalogDefinition(): Record<string, unknown> {
  return new Catalog<ReactCatalogComponent>(CATALOG_ID, '1.0', [...layout([]), ...CONTENT_COMPONENTS], []).toJson()
}

function required<T>(value: T | undefined, name: string): T {
  if (value === undefined) throw new Error(`A2UI basic catalog has no ${name}`)
  return value
}

export type LoadedSurface =
  | { ok: true; surface: SurfaceModel<ReactCatalogComponent>; dispose(): void }
  | { ok: false; error: string }

/**
 * Applies a view file's A2UI messages, in order, to a fresh surface. The
 * whole view is refused when a message, a component's props, a reference
 * between components or a nesting rule is invalid; an unknown component only
 * marks its own place.
 */
export function loadSurface(messages: unknown[]): LoadedSurface {
  const processor = new MessageProcessor<ReactCatalogComponent>([catalog], undefined, {
    validationConfig: {
      ...STRICT_VALIDATION,
      // The children of an unknown component are unreachable once it is
      // replaced; they are not shown, which is not worth refusing the view.
      allowOrphanComponents: true,
      allowedMessages: ['createSurface', 'updateComponents'],
    },
  })
  try {
    // The processor validates every message against A2UI 1.0 and the catalog.
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

/**
 * Nesting violations keep their A2UI code, so the reason names the rule
 * broken; the placeholder for unknown components is ours, not the reader's.
 */
function describe(error: unknown): string {
  const message = (error instanceof Error ? error.message : String(error))
    .replace(/^\w*Error: /, '')
    .replaceAll(`,"${UNKNOWN}"`, '')
  const code = isRecord(error) ? error.code : undefined
  return code === 'UNALLOWED_PARENT' || code === 'UNALLOWED_CHILD' ? `${code}: ${message}` : message
}

/**
 * Renders a loaded surface with the app's Markdown rendering for Text and
 * the source actions its content components draw on.
 */
export function ViewSurface({
  surface,
  sources,
}: {
  surface: SurfaceModel<ReactCatalogComponent>
  sources: SourceActions
}): React.ReactElement {
  const problems = useSurfaceProblems(surface)
  return (
    <SourceContext.Provider value={sources}>
      <MarkdownComponentContext.Provider value={SafeMarkdown}>
        {problems.length > 0 ? (
          <ul className="genui-problems" role="alert">
            {problems.map((problem, index) => (
              <li key={index}>{problem}</li>
            ))}
          </ul>
        ) : null}
        <div className="genui-surface">
          <A2uiSurface surface={surface} />
        </div>
      </MarkdownComponentContext.Provider>
    </SourceContext.Provider>
  )
}

/**
 * Problems the renderer finds while it resolves the tree. An unknown
 * component is already shown in place, so it is not listed again.
 */
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
