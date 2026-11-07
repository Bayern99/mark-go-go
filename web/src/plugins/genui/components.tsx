/**
 * Mark Go Go's own content components. Each owns its layout, takes
 * `sources` and `weight`, has no children, and draws in the shared
 * source frame. `a2ui.tsx` puts them in the catalog.
 */
import { useState } from 'react'
import { z } from 'zod'
import { createComponentImplementation } from '@a2ui/react'
import { SourceFrame, useComponentSource } from './SourceFrame'

/** Props every content component accepts besides its own. */
const common = {
  /** Source references: ids in the view file's `sources` table. */
  sources: z.array(z.string()).optional().describe('Ids of the source sections this component draws on.'),
  /** Share of a Row's width, as for the basic layout components. */
  weight: z.number().positive().optional().describe("Share of a Row's width."),
}

const title = z.string().optional().describe('A short heading shown above the content.')

function flex(weight: number | undefined): React.CSSProperties | undefined {
  return weight === undefined ? undefined : { flex: weight }
}

export const KeyPoints = createComponentImplementation(
  {
    name: 'KeyPoints',
    schema: z
      .object({
        title,
        items: z.array(z.string()).min(1).describe('The points, as plain text.'),
        ...common,
      })
      .strict()
      .describe('The few points a reader should take away.'),
  },
  ({ props, context }) => (
    <SourceFrame
      source={useComponentSource(props.sources)}
      componentId={context.componentModel.id}
      className="genui-keypoints"
      style={flex(props.weight)}
      title={props.title}
    >
      <ul>
        {props.items.map((item, index) => (
          <li key={index}>{item}</li>
        ))}
      </ul>
    </SourceFrame>
  ),
)

const Metric = z
  .object({
    label: z.string().describe('What is measured.'),
    value: z.string().describe('The figure as it should read, such as "12" or "3.4".'),
    unit: z.string().optional().describe('Unit shown after the value, such as "%" or "ms".'),
    change: z.string().optional().describe('Change against a baseline, as it should read, such as "+4%".'),
    trend: z
      .enum(['up', 'down', 'flat'])
      .optional()
      .describe('Direction of the change, which only sets its colour and arrow.'),
  })
  .strict()

const TREND_MARK = { up: '▲', down: '▼', flat: '■' } as const

export const MetricGrid = createComponentImplementation(
  {
    name: 'MetricGrid',
    schema: z
      .object({
        title,
        metrics: z.array(Metric).min(1).describe('The figures, in reading order.'),
        ...common,
      })
      .strict()
      .describe('A grid of headline figures.'),
  },
  ({ props, context }) => (
    <SourceFrame
      source={useComponentSource(props.sources)}
      componentId={context.componentModel.id}
      className="genui-metrics"
      style={flex(props.weight)}
      title={props.title}
    >
      <dl>
        {props.metrics.map((metric, index) => (
          <div key={index} className="genui-metric">
            <dt>{metric.label}</dt>
            <dd>
              <span className="genui-metric-value">{metric.value}</span>
              {metric.unit ? <span className="genui-metric-unit">{metric.unit}</span> : null}
            </dd>
            {metric.change ? (
              <dd className="genui-metric-change" data-trend={metric.trend}>
                {metric.trend ? <span aria-hidden="true">{TREND_MARK[metric.trend]} </span> : null}
                {metric.change}
              </dd>
            ) : null}
          </div>
        ))}
      </dl>
    </SourceFrame>
  ),
)

type SortState = { column: number; direction: 'ascending' | 'descending' } | null

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' })

/** Cells that read as numbers sort by value; anything else by text. */
function compareCells(a: string, b: string): number {
  const x = asNumber(a)
  const y = asNumber(b)
  if (x !== null && y !== null) return x - y
  return collator.compare(a, b)
}

function asNumber(cell: string): number | null {
  const cleaned = cell.replace(/[,\s%$€£¥]/g, '')
  if (!/^[+-]?(\d+(\.\d*)?|\.\d+)$/.test(cleaned)) return null
  return Number(cleaned)
}

export const ComparisonTable = createComponentImplementation(
  {
    name: 'ComparisonTable',
    schema: z
      .object({
        title,
        columns: z
          .array(z.string())
          .min(2)
          .describe('Column headings; the first names what each row compares.'),
        rows: z
          .array(z.array(z.string()))
          .min(1)
          .describe('One list of cell texts per row, as many as there are columns.'),
        ...common,
      })
      .strict()
      .superRefine((value, ctx) => {
        value.rows.forEach((row, index) => {
          if (row.length !== value.columns.length) {
            ctx.addIssue({
              code: z.ZodIssueCode.custom,
              path: ['rows', index],
              message: `Row ${index + 1} has ${row.length} cells; there are ${value.columns.length} columns.`,
            })
          }
        })
      })
      .describe('Items compared side by side. Readers can sort by any column; sorting changes only the display.'),
  },
  ({ props, context }) => {
    const [sort, setSort] = useState<SortState>(null)
    const order = props.rows.map((_, index) => index)
    if (sort) {
      const sign = sort.direction === 'ascending' ? 1 : -1
      order.sort(
        (a, b) =>
          sign * compareCells(props.rows[a]?.[sort.column] ?? '', props.rows[b]?.[sort.column] ?? '') || a - b,
      )
    }
    const toggle = (column: number) =>
      setSort((current) =>
        current?.column !== column
          ? { column, direction: 'ascending' }
          : current.direction === 'ascending'
            ? { column, direction: 'descending' }
            : null,
      )
    return (
      <SourceFrame
        source={useComponentSource(props.sources)}
        componentId={context.componentModel.id}
        className="genui-comparison"
        style={flex(props.weight)}
        title={props.title}
      >
        <div className="genui-table-scroll">
          <table>
            <thead>
              <tr>
                {props.columns.map((column, index) => (
                  <th
                    key={index}
                    scope="col"
                    aria-sort={sort?.column === index ? sort.direction : 'none'}
                  >
                    <button type="button" onClick={() => toggle(index)}>
                      {column}
                      <span className="genui-sort-mark" aria-hidden="true">
                        {sort?.column === index ? (sort.direction === 'ascending' ? '↑' : '↓') : '↕'}
                      </span>
                    </button>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {order.map((rowIndex) => {
                const row = props.rows[rowIndex] ?? []
                return (
                  <tr key={rowIndex}>
                    {row.map((cell, cellIndex) =>
                      cellIndex === 0 ? (
                        <th key={cellIndex} scope="row">
                          {cell}
                        </th>
                      ) : (
                        <td key={cellIndex}>{cell}</td>
                      ),
                    )}
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      </SourceFrame>
    )
  },
)

const Moment = z
  .object({
    when: z.string().describe('The time or phase, as it should read, such as "2002" or "Phase 2".'),
    title: z.string().describe('What happened or happens then.'),
    detail: z.string().optional().describe('One or two sentences more.'),
  })
  .strict()

export const Timeline = createComponentImplementation(
  {
    name: 'Timeline',
    schema: z
      .object({
        title,
        events: z.array(Moment).min(1).describe('The events, in order.'),
        ...common,
      })
      .strict()
      .describe('Events or phases in order.'),
  },
  ({ props, context }) => (
    <SourceFrame
      source={useComponentSource(props.sources)}
      componentId={context.componentModel.id}
      className="genui-timeline"
      style={flex(props.weight)}
      title={props.title}
    >
      <ol>
        {props.events.map((event, index) => (
          <li key={index}>
            <span className="genui-timeline-when">{event.when}</span>
            <div className="genui-timeline-body">
              <p className="genui-timeline-title">{event.title}</p>
              {event.detail ? <p className="genui-timeline-detail">{event.detail}</p> : null}
            </div>
          </li>
        ))}
      </ol>
    </SourceFrame>
  ),
)

const Task = z
  .object({
    text: z.string().describe('The item, as plain text.'),
    done: z.boolean().optional().describe('Whether the document marks it done. Default false.'),
    quote: z
      .string()
      .optional()
      .describe('Text copied exactly from the source section, which clicking the item selects. Default: the item text.'),
  })
  .strict()

export const Checklist = createComponentImplementation(
  {
    name: 'Checklist',
    schema: z
      .object({
        title,
        items: z.array(Task).min(1).describe('The items, in order.'),
        ...common,
      })
      .strict()
      .describe(
        'Items with their state in the document. Read-only: clicking an item opens it in the document, it never changes the state.',
      ),
  },
  ({ props, context }) => {
    const source = useComponentSource(props.sources)
    return (
      <SourceFrame
        source={source}
        componentId={context.componentModel.id}
        className="genui-checklist"
        style={flex(props.weight)}
        title={props.title}
      >
        <ul>
          {props.items.map((item, index) => (
            <li key={index} data-done={item.done ? 'true' : 'false'}>
              <button
                type="button"
                role="checkbox"
                aria-checked={item.done ?? false}
                aria-readonly="true"
                disabled={!source.edit}
                title={source.edit ? 'Show in the document' : undefined}
                onClick={() => source.edit?.(item.quote ?? item.text)}
              >
                <span className="genui-check" aria-hidden="true">
                  {item.done ? '✓' : ''}
                </span>
                <span className="genui-check-text">{item.text}</span>
              </button>
            </li>
          ))}
        </ul>
      </SourceFrame>
    )
  },
)

/** Content components, in the order the format document describes them. */
export const CONTENT_COMPONENTS = [KeyPoints, MetricGrid, ComparisonTable, Timeline, Checklist]
