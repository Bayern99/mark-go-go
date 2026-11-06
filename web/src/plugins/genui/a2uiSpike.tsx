// SPIKE (branch spike/a2ui only): proves @a2ui/react 0.15 renders an A2UI 1.0
// surface with an app-owned catalog inside the MyGo WKWebView. Not for main.
import { createContext, useContext, useEffect, useState } from 'react'
import { z } from 'zod'
import { A2uiSurface, createComponentImplementation, type ReactCatalogComponent } from '@a2ui/react'
import { Catalog, CommonSchemas, MessageProcessor } from '@a2ui/web_core/v1_0'
import { basicCatalog } from '@a2ui/web_core/catalogs/basic/v1'

export const SPIKE_CATALOG_ID = 'https://markgogo.dev/catalogs/genui/v1.json'

const Reveal = createContext<(sources: string[]) => void>(() => {})

const KeyPoints = createComponentImplementation(
  {
    name: 'KeyPoints',
    schema: z.object({
      title: CommonSchemas.DynamicString,
      items: z.array(z.string()),
      sources: z.array(z.string()),
    }),
  },
  ({ props }) => {
    const reveal = useContext(Reveal)
    return (
      <article className="genui-block" data-testid="spike-keypoints">
        <h4>{props.title}</h4>
        <ul className="genui-summary">
          {props.items.map((item, index) => (
            <li key={index}>{item}</li>
          ))}
        </ul>
        <button type="button" onClick={() => reveal(props.sources)}>
          去原文
        </button>
      </article>
    )
  },
)

const LAYOUT = ['Column', 'Row', 'Tabs', 'Card', 'Text']

const catalog = new Catalog<ReactCatalogComponent>(
  SPIKE_CATALOG_ID,
  '1.0',
  [...LAYOUT.map((name) => basicCatalog.components.get(name)!), KeyPoints],
  Array.from(basicCatalog.functions.values()),
)

export function A2uiSpike({
  messages,
  onReveal,
}: {
  messages: unknown[]
  onReveal: (sources: string[]) => void
}): React.ReactElement {
  const [result] = useState(() => {
    const processor = new MessageProcessor<ReactCatalogComponent>([catalog])
    // Problems found while resolving the tree (e.g. UNKNOWN_COMPONENT_TYPE) are
    // reported on the surface, not thrown.
    const problems: string[] = []
    processor.onSurfaceCreated((surface) => {
      surface.onError.subscribe((e) => {
        problems.push(`${e.code}: ${e.message}`)
      })
    })
    try {
      // The processor validates each message against the A2UI 1.0 schema and the catalog.
      const input = messages as Parameters<typeof processor.processMessages>[0]
      processor.processMessages(input)
      return { surfaces: Array.from(processor.model.surfacesMap.values()), problems, error: '' }
    } catch (err) {
      const code = err && typeof err === 'object' && 'code' in err ? String(err.code) : 'ERROR'
      return { surfaces: [], problems, error: `${code}: ${String(err)}` }
    }
  })
  if (result.error) {
    return (
      <p className="genui-file-error" role="alert" data-testid="spike-error">
        {result.error}
      </p>
    )
  }
  return (
    <Reveal.Provider value={onReveal}>
      {result.surfaces.map((surface) => (
        <A2uiSurface key={surface.id} surface={surface} />
      ))}
      <SurfaceProblems problems={result.problems} />
    </Reveal.Provider>
  )
}

/** Problems reported after the first render, read once they have settled. */
function SurfaceProblems({ problems }: { problems: string[] }): React.ReactElement | null {
  const [shown, setShown] = useState<string[]>([])
  useEffect(() => {
    const id = window.setTimeout(() => setShown([...problems]), 0)
    return () => window.clearTimeout(id)
  }, [problems])
  if (shown.length === 0) return null
  return (
    <ul className="genui-problems" role="alert" data-testid="spike-problems">
      {shown.map((problem, index) => (
        <li key={index}>{problem}</li>
      ))}
    </ul>
  )
}
