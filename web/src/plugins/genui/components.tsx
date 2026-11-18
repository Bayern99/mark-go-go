import { useCallback, useContext, useId, useRef, useState } from 'react'
import { z } from 'zod'
import { createComponentImplementation } from '@a2ui/react'
import { ScrollArea } from '../../components/ScrollArea'
import { SourceFrame, useComponentSource } from './SourceFrame'
import { ReadingContext } from './reading'

const KEYPOINTS_FOLD_AFTER = 4

const common = {
  sources: z.array(z.string()).optional().describe('Ids of the source sections this component draws on.'),
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
  ({ props, context }) => {
    const id = context.componentModel.id
    const reading = useContext(ReadingContext)
    const [expanded, setExpanded] = useState(() => reading?.keypointsExpanded(id) ?? false)
    const foldable = props.items.length > KEYPOINTS_FOLD_AFTER
    const visible = foldable && !expanded ? props.items.slice(0, KEYPOINTS_FOLD_AFTER) : props.items
    return (
      <SourceFrame
        source={useComponentSource(props.sources)}
        componentId={id}
        className="genui-keypoints"
        style={flex(props.weight)}
        title={props.title}
      >
        <ol>
          {visible.map((item, index) => (
            <li key={index} value={index + 1}>
              {item}
            </li>
          ))}
        </ol>
        {foldable ? (
          <button
            type="button"
            className="pill xs quiet genui-keypoints-more"
            aria-expanded={expanded}
            onClick={() => {
              const next = !expanded
              setExpanded(next)
              reading?.setKeypointsExpanded(id, next)
            }}
          >
            {expanded ? 'Show fewer' : `Show all ${props.items.length}`}
          </button>
        ) : null}
      </SourceFrame>
    )
  },
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
    const id = context.componentModel.id
    const reading = useContext(ReadingContext)
    const [sort, setSort] = useState<SortState>(() => reading?.tableSort(id) ?? null)
    const tableName = props.title ? `${props.title} table` : 'Comparison table'
    const scrollLabelId = useId()
    const order = props.rows.map((_, index) => index)
    if (sort) {
      const sign = sort.direction === 'ascending' ? 1 : -1
      order.sort(
        (a, b) =>
          sign * compareCells(props.rows[a]?.[sort.column] ?? '', props.rows[b]?.[sort.column] ?? '') || a - b,
      )
    }
    const toggle = (column: number) =>
      setSort((current) => {
        const next =
          current?.column !== column
            ? { column, direction: 'ascending' as const }
            : current.direction === 'ascending'
              ? { column, direction: 'descending' as const }
              : null
        reading?.setTableSort(id, next)
        return next
      })
    const viewport = useRef<HTMLElement | null>(null)
    const onViewport = useCallback(
      (element: HTMLElement | null) => {
        viewport.current = element
        if (!element) return
        element.tabIndex = 0
        element.setAttribute('role', 'group')
        element.setAttribute('aria-labelledby', scrollLabelId)
        element.scrollLeft = reading?.tableScrollLeft(id) ?? 0
      },
      [id, reading, scrollLabelId],
    )
    const onViewportScroll = useCallback(() => {
      if (viewport.current) reading?.setTableScrollLeft(id, viewport.current.scrollLeft)
    }, [id, reading])
    return (
      <SourceFrame
        source={useComponentSource(props.sources)}
        componentId={id}
        className="genui-comparison"
        style={flex(props.weight)}
        title={props.title}
      >
        <ScrollArea
          axis="x"
          className="genui-table-scroll"
          contentClassName="genui-table-scroll-content"
          onViewport={onViewport}
          onViewportScroll={onViewportScroll}
        >
          <span id={scrollLabelId} hidden>{tableName}, scrollable</span>
          <table>
            <caption className="genui-table-caption">{tableName}</caption>
            <thead>
              <tr>
                {props.columns.map((column, index) => (
                  <th
                    key={index}
                    scope="col"
                    aria-sort={sort?.column === index ? sort.direction : 'none'}
                  >
                    <button
                      type="button"
                      onClick={() => toggle(index)}
                      aria-label={column.trim() === '' ? `Column ${index + 1}` : undefined}
                    >
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
        </ScrollArea>
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

export const CONTENT_COMPONENTS = [KeyPoints, MetricGrid, ComparisonTable, Timeline, Checklist]
