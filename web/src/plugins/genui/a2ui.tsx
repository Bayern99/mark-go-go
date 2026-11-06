/**
 * The only module that knows A2UI. It owns the component catalog, turns a
 * view file's messages into one rendered surface, and reports what went
 * wrong. Everything else in GenUI talks to it through `loadSurface` and
 * `ViewSurface`, so the renderer can be replaced without touching them.
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
import { SafeMarkdown } from './SafeMarkdown'
import { SourceContext, SourceFrame, type SourceActions } from './SourceFrame'

/** The catalog id a view file's surface must name. */
export const CATALOG_ID = 'markgogo.dev:genui/v1'

/** Layout components taken as they are from the A2UI basic catalog. */
const LAYOUT = ['Column', 'Row', 'Tabs', 'Card', 'Text'] as const

/** Props every one of our components accepts besides its own. */
const common = {
  /** Source references: ids in the view file's `sources` table. */
  sources: z.array(z.string()).optional(),
  /** Share of a Row's width, as for the basic layout components. */
  weight: z.number().optional(),
}

const KeyPoints = createComponentImplementation(
  {
    name: 'KeyPoints',
    schema: z
      .object({
        title: z.string().optional(),
        items: z.array(z.string()).min(1),
        ...common,
      })
      .strict(),
    allowedChildren: [],
  },
  ({ props }) => (
    <SourceFrame sources={props.sources} className="genui-keypoints" style={flex(props.weight)}>
      {props.title ? <h4>{props.title}</h4> : null}
      <ul>
        {props.items.map((item, index) => (
          <li key={index}>{item}</li>
        ))}
      </ul>
    </SourceFrame>
  ),
)

/**
 * Stands in for a component the catalog does not have, so that one
 * unknown component shows an error where it was and the rest still renders.
 * The view file never names it: `loadSurface` puts it there.
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

const catalog = new Catalog<ReactCatalogComponent>(
  CATALOG_ID,
  '1.0',
  [...LAYOUT.map((name) => required(basicCatalog.components.get(name), name)), KeyPoints, Unknown],
  Array.from(basicCatalog.functions.values()),
)

/** Names a view file may use. */
const KNOWN = new Set([...catalog.components.keys()].filter((name) => name !== UNKNOWN))

function required<T>(value: T | undefined, name: string): T {
  if (value === undefined) throw new Error(`A2UI basic catalog has no ${name}`)
  return value
}

function flex(weight: number | undefined): React.CSSProperties | undefined {
  return weight === undefined ? undefined : { flex: weight }
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

function describe(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error)
  return message.replace(/^\w*Error: /, '')
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
