import type { DiffHunk, DiffLine, DiffLineKind, HunkDecision } from '../host/generated/mygo'

const DEFAULT_CONTEXT_LINES = 3

interface TextLine {
  raw: string
  content: string
}

function splitTextLines(text: string): TextLine[] {
  if (text === '') return []
  const out: TextLine[] = []
  let start = 0
  for (let i = 0; i < text.length; i += 1) {
    if (text[i] === '\n') {
      const raw = text.slice(start, i + 1)
      out.push({ raw, content: trimLineEnding(raw) })
      start = i + 1
    }
  }
  if (start < text.length) {
    const raw = text.slice(start)
    out.push({ raw, content: trimLineEnding(raw) })
  }
  return out
}

function trimLineEnding(s: string): string {
  if (s.endsWith('\r\n')) return s.slice(0, -2)
  if (s.endsWith('\n') || s.endsWith('\r')) return s.slice(0, -1)
  return s
}

function defaultLineEnding(text: string): string {
  return text.includes('\r\n') ? '\r\n' : '\n'
}

type EditOpKind = 'equal' | 'delete' | 'insert'

interface EditOp {
  kind: EditOpKind
  oldIdx: number
  newIdx: number
  text: string
  raw: string
}

interface ChangeCluster {
  oldStart: number
  oldEnd: number
  newStart: number
  newEnd: number
  deletes: EditOp[]
  inserts: EditOp[]
}

function myersEditScript(a: TextLine[], b: TextLine[], strictRaw: boolean): EditOp[] {
  const n = a.length
  const m = b.length
  const equal = (x: number, y: number) =>
    strictRaw ? a[x].raw === b[y].raw : a[x].content === b[y].content

  let prefix = 0
  while (prefix < n && prefix < m && equal(prefix, prefix)) prefix += 1
  let suffix = 0
  while (suffix < n - prefix && suffix < m - prefix && equal(n - 1 - suffix, m - 1 - suffix)) {
    suffix += 1
  }

  const ops: EditOp[] = []
  for (let i = 0; i < prefix; i += 1) {
    ops.push({ kind: 'equal', oldIdx: i, newIdx: i, text: a[i].content, raw: a[i].raw })
  }

  const midA = a.slice(prefix, n - suffix)
  const midB = b.slice(prefix, m - suffix)
  if (midA.length > 0 || midB.length > 0) {
    ops.push(...myersCore(midA, midB, prefix, prefix, strictRaw))
  }

  for (let i = 0; i < suffix; i += 1) {
    const oldIdx = n - suffix + i
    const newIdx = m - suffix + i
    ops.push({ kind: 'equal', oldIdx, newIdx, text: a[oldIdx].content, raw: a[oldIdx].raw })
  }
  return ops
}

function myersCore(
  a: TextLine[],
  b: TextLine[],
  offsetA: number,
  offsetB: number,
  strictRaw: boolean,
): EditOp[] {
  const n = a.length
  const m = b.length
  const maxD = n + m
  const size = 2 * maxD + 1
  const v = new Int32Array(size)
  const trace: Int32Array[] = []
  const equal = (x: number, y: number) =>
    strictRaw ? a[x].raw === b[y].raw : a[x].content === b[y].content

  let foundD = maxD
  for (let d = 0; d <= maxD; d += 1) {
    trace.push(v.slice())
    let done = false
    for (let k = -d; k <= d; k += 2) {
      const idx = k + maxD
      let x = k === -d || (k !== d && v[idx - 1] < v[idx + 1]) ? v[idx + 1] : v[idx - 1] + 1
      let y = x - k
      while (x < n && y < m && equal(x, y)) {
        x += 1
        y += 1
      }
      v[idx] = x
      if (x >= n && y >= m) {
        foundD = d
        done = true
        break
      }
    }
    if (done) break
  }

  const rev: EditOp[] = []
  let x = n
  let y = m
  for (let d = foundD; d > 0; d -= 1) {
    const prevV = trace[d]
    const k = x - y
    const prevK = k === -d || (k !== d && prevV[k + maxD - 1] < prevV[k + maxD + 1]) ? k + 1 : k - 1
    const prevX = prevV[prevK + maxD]
    const prevY = prevX - prevK

    while (x > prevX && y > prevY) {
      x -= 1
      y -= 1
      rev.push({ kind: 'equal', oldIdx: offsetA + x, newIdx: offsetB + y, text: a[x].content, raw: a[x].raw })
    }
    if (x === prevX) {
      y -= 1
      rev.push({ kind: 'insert', oldIdx: offsetA + x, newIdx: offsetB + y, text: b[y].content, raw: b[y].raw })
    } else {
      x -= 1
      rev.push({ kind: 'delete', oldIdx: offsetA + x, newIdx: offsetB + y, text: a[x].content, raw: a[x].raw })
    }
  }
  while (x > 0 && y > 0) {
    x -= 1
    y -= 1
    rev.push({ kind: 'equal', oldIdx: offsetA + x, newIdx: offsetB + y, text: a[x].content, raw: a[x].raw })
  }
  return rev.reverse()
}

function extractClusters(before: string, after: string): { ops: EditOp[]; clusters: ChangeCluster[] } {
  if (before === after) return { ops: [], clusters: [] }
  const a = splitTextLines(before)
  const b = splitTextLines(after)
  let ops = myersEditScript(a, b, false)
  if (!ops.some((op) => op.kind !== 'equal')) {
    ops = myersEditScript(a, b, true)
  }

  const clusters: ChangeCluster[] = []
  let i = 0
  while (i < ops.length) {
    if (ops[i].kind === 'equal') {
      i += 1
      continue
    }
    const cluster: ChangeCluster = {
      oldStart: ops[i].oldIdx,
      oldEnd: ops[i].oldIdx,
      newStart: ops[i].newIdx,
      newEnd: ops[i].newIdx,
      deletes: [],
      inserts: [],
    }
    while (i < ops.length && ops[i].kind !== 'equal') {
      if (ops[i].kind === 'delete') cluster.deletes.push(ops[i])
      else cluster.inserts.push(ops[i])
      i += 1
    }
    if (cluster.deletes.length > 0) {
      cluster.oldStart = cluster.deletes[0].oldIdx
      cluster.oldEnd = cluster.deletes[cluster.deletes.length - 1].oldIdx + 1
    }
    if (cluster.inserts.length > 0) {
      cluster.newStart = cluster.inserts[0].newIdx
      cluster.newEnd = cluster.inserts[cluster.inserts.length - 1].newIdx + 1
    }
    clusters.push(cluster)
  }
  return { ops, clusters }
}

function buildHunkHeader(
  lines: DiffLine[],
  fallbackOldStart: number,
  fallbackNewStart: number,
): { header: string; oldStart: number; oldLines: number; newStart: number; newLines: number } {
  let oldStart = fallbackOldStart
  let oldLines = 0
  let newStart = fallbackNewStart
  let newLines = 0
  let foundOld = false
  let foundNew = false
  for (const line of lines) {
    if (line.kind !== 'insert') {
      if (!foundOld && line.oldLine) {
        oldStart = line.oldLine
        foundOld = true
      }
      oldLines += 1
    }
    if (line.kind !== 'delete') {
      if (!foundNew && line.newLine) {
        newStart = line.newLine
        foundNew = true
      }
      newLines += 1
    }
  }
  return {
    header: `@@ -${oldStart},${oldLines} +${newStart},${newLines} @@`,
    oldStart,
    oldLines,
    newStart,
    newLines,
  }
}

export function computeMyersHunks(
  before: string,
  after: string,
  contextLines = DEFAULT_CONTEXT_LINES,
): DiffHunk[] {
  const { ops, clusters } = extractClusters(before, after)
  if (clusters.length === 0) return []

  const equalRuns: EditOp[][] = []
  let currentEquals: EditOp[] = []
  let inChange = false
  for (const op of ops) {
    if (op.kind === 'equal') {
      currentEquals.push(op)
      inChange = false
    } else if (!inChange) {
      equalRuns.push(currentEquals)
      currentEquals = []
      inChange = true
    }
  }
  equalRuns.push(currentEquals)

  return clusters.map((cluster, idx) => {
    const preRun = equalRuns[idx] ?? []
    const postRun = equalRuns[idx + 1] ?? []
    const preEquals = preRun.length > contextLines ? preRun.slice(preRun.length - contextLines) : preRun
    const postEquals = postRun.length > contextLines ? postRun.slice(0, contextLines) : postRun

    const lines: DiffLine[] = [
      ...preEquals.map((eq) => ({
        kind: 'context' as DiffLineKind,
        text: eq.text,
        oldLine: eq.oldIdx + 1,
        newLine: eq.newIdx + 1,
      })),
      ...cluster.deletes.map((del) => ({
        kind: 'delete' as DiffLineKind,
        text: del.text,
        oldLine: del.oldIdx + 1,
      })),
      ...cluster.inserts.map((ins) => ({
        kind: 'insert' as DiffLineKind,
        text: ins.text,
        newLine: ins.newIdx + 1,
      })),
      ...postEquals.map((eq) => ({
        kind: 'context' as DiffLineKind,
        text: eq.text,
        oldLine: eq.oldIdx + 1,
        newLine: eq.newIdx + 1,
      })),
    ]

    const span = buildHunkHeader(lines, cluster.oldStart + 1, cluster.newStart + 1)
    return {
      id: `hunk-${idx + 1}`,
      ...span,
      changeOldLine: cluster.oldStart + 1,
      changeOldCount: cluster.deletes.length,
      changeNewLine: cluster.newStart + 1,
      changeNewCount: cluster.inserts.length,
      lines,
      decision: 'pending',
    }
  })
}

function clustersConflict(prop: ChangeCluster, user: ChangeCluster): { collides: boolean; identical: boolean } {
  if (
    prop.oldStart === user.oldStart &&
    prop.oldEnd === user.oldEnd &&
    prop.inserts.length === user.inserts.length &&
    prop.inserts.every((ins, idx) => ins.text === user.inserts[idx]?.text)
  ) {
    return { collides: false, identical: true }
  }
  const pEmpty = prop.oldStart === prop.oldEnd
  const uEmpty = user.oldStart === user.oldEnd
  if (pEmpty && uEmpty) return { collides: prop.oldStart === user.oldStart, identical: false }
  if (pEmpty) return { collides: user.oldStart < prop.oldStart && prop.oldStart < user.oldEnd, identical: false }
  if (uEmpty) return { collides: prop.oldStart < user.oldStart && user.oldStart < prop.oldEnd, identical: false }
  return {
    collides: Math.max(prop.oldStart, user.oldStart) < Math.min(prop.oldEnd, user.oldEnd),
    identical: false,
  }
}

export function reconcileProposal(
  baseText: string,
  proposedText: string,
  liveText: string,
  decisions: Record<string, HunkDecision> = {},
): { hunks: DiffHunk[]; mergedText: string; hasConflict: boolean } {
  const { clusters: baseClusters } = extractClusters(baseText, proposedText)
  if (baseClusters.length === 0) {
    return { hunks: [], mergedText: liveText, hasConflict: false }
  }

  const liveLines = splitTextLines(liveText)
  const { clusters: userClusters } = extractClusters(baseText, liveText)

  const baseToLive = (baseIdx: number) => {
    let delta = 0
    for (const u of userClusters) {
      if (u.oldEnd <= baseIdx) {
        delta += u.inserts.length - (u.oldEnd - u.oldStart)
      }
    }
    return Math.max(0, Math.min(liveLines.length, baseIdx + delta))
  }

  let hasConflict = false
  const resolved: Array<{
    hunk: DiffHunk
    liveStart: number
    liveEnd: number
    inserts: EditOp[]
    blocked: boolean
  }> = []

  for (let idx = 0; idx < baseClusters.length; idx += 1) {
    const c = baseClusters[idx]
    const hunkId = `hunk-${idx + 1}`
    let conflict = false
    let conflictReason = ''
    let alreadyApplied = false

    for (const u of userClusters) {
      const check = clustersConflict(c, u)
      if (check.identical) {
        alreadyApplied = true
        break
      }
      if (check.collides) {
        conflict = true
        conflictReason = `Concurrent edit overlaps base line ${c.oldStart + 1}`
        break
      }
    }

    let decision: HunkDecision = decisions[hunkId] ?? 'pending'
    if (conflict) {
      hasConflict = true
      if (decision === 'accepted') decision = 'pending'
    }

    const liveStart = Math.min(liveLines.length, baseToLive(c.oldStart))
    let liveEnd = conflict || alreadyApplied ? baseToLive(c.oldEnd) : liveStart + (c.oldEnd - c.oldStart)
    if (liveEnd < liveStart) liveEnd = liveStart
    liveEnd = Math.min(liveLines.length, liveEnd)

    let preStart = Math.max(0, liveStart - DEFAULT_CONTEXT_LINES)
    if (idx > 0 && preStart < resolved[idx - 1].liveEnd) {
      preStart = resolved[idx - 1].liveEnd
    }
    let postEnd = Math.min(liveLines.length, liveEnd + DEFAULT_CONTEXT_LINES)
    if (idx + 1 < baseClusters.length) {
      const nextLiveStart = baseToLive(baseClusters[idx + 1].oldStart)
      if (postEnd > nextLiveStart) postEnd = nextLiveStart
    }

    const lines: DiffLine[] = []
    let newLineCursor = Math.max(1, c.newStart + 1 - (liveStart - preStart))
    for (let lIdx = preStart; lIdx < liveStart; lIdx += 1) {
      lines.push({
        kind: 'context',
        text: liveLines[lIdx].content,
        oldLine: lIdx + 1,
        newLine: newLineCursor,
      })
      newLineCursor += 1
    }
    c.deletes.forEach((del, dIdx) => {
      lines.push({
        kind: 'delete',
        text: del.text,
        oldLine: liveStart + dIdx + 1,
      })
    })
    for (const ins of c.inserts) {
      lines.push({
        kind: 'insert',
        text: ins.text,
        newLine: newLineCursor,
      })
      newLineCursor += 1
    }
    for (let lIdx = liveEnd; lIdx < postEnd; lIdx += 1) {
      lines.push({
        kind: 'context',
        text: liveLines[lIdx].content,
        oldLine: lIdx + 1,
        newLine: newLineCursor,
      })
      newLineCursor += 1
    }

    const span = buildHunkHeader(lines, liveStart + 1, c.newStart + 1)
    const hunk: DiffHunk = {
      id: hunkId,
      ...span,
      changeOldLine: liveStart + 1,
      changeOldCount: c.deletes.length,
      changeNewLine: c.newStart + 1,
      changeNewCount: c.inserts.length,
      lines,
      decision,
      conflict,
      conflictReason: conflictReason || undefined,
    }
    resolved.push({
      hunk,
      liveStart,
      liveEnd,
      inserts: c.inserts,
      blocked: conflict || alreadyApplied,
    })
  }

  const anyAccepted = resolved.some((r) => r.hunk.decision === 'accepted' && !r.blocked)
  const anyRejected = resolved.some((r) => r.hunk.decision === 'rejected')
  const shouldApply = (r: (typeof resolved)[number]) => {
    if (r.blocked) return false
    if (anyAccepted) return r.hunk.decision === 'accepted'
    if (anyRejected) return r.hunk.decision !== 'rejected'
    return true
  }

  const nl =
    defaultLineEnding(liveText) === '\r\n'
      ? '\r\n'
      : !liveText.includes('\n') && proposedText.includes('\r\n')
        ? '\r\n'
        : '\n'

  const outRaw: string[] = []
  let cursor = 0
  for (const r of resolved) {
    if (!shouldApply(r)) continue
    while (cursor < r.liveStart && cursor < liveLines.length) {
      outRaw.push(liveLines[cursor].raw)
      cursor += 1
    }
    for (const ins of r.inserts) outRaw.push(ins.raw)
    cursor = r.liveEnd
  }
  while (cursor < liveLines.length) {
    outRaw.push(liveLines[cursor].raw)
    cursor += 1
  }

  let mergedText = ''
  for (let i = 0; i < outRaw.length; i += 1) {
    const raw = outRaw[i]
    mergedText += i + 1 < outRaw.length && !raw.endsWith('\n') ? raw + nl : raw
  }

  return {
    hunks: resolved.map((r) => r.hunk),
    mergedText,
    hasConflict,
  }
}

export interface DocumentDiffProps {
  before: string
  after: string
  hunks?: DiffHunk[]
  onDecideHunk?: (hunkId: string, decision: HunkDecision) => void
  onAcceptAll?: () => void
}

export function DocumentDiff({ before, after, hunks, onDecideHunk, onAcceptAll }: DocumentDiffProps) {
  const computedHunks = hunks ?? computeMyersHunks(before, after)
  if (computedHunks.length === 0) return <p>No changes.</p>

  const showControls = typeof onDecideHunk === 'function'
  return (
    <div className="document-diff" aria-label="Markdown changes">
      {showControls && onAcceptAll && computedHunks.length > 1 && (
        <div className="diff-toolbar">
          <button type="button" className="pill quiet" onClick={onAcceptAll}>
            Accept All
          </button>
        </div>
      )}
      {computedHunks.map((hunk) => (
        <div
          key={hunk.id}
          className="diff-hunk"
          data-hunk-id={hunk.id}
          data-decision={hunk.decision}
          data-conflict={hunk.conflict ? 'true' : 'false'}
        >
          <div className="diff-hunk-header">
            <code className="diff-hunk-range">{hunk.header}</code>
            {hunk.conflict && (
              <span className="diff-hunk-conflict" role="status">
                Conflict{hunk.conflictReason ? `: ${hunk.conflictReason}` : ''}
              </span>
            )}
            {showControls && (
              <div className="diff-hunk-actions">
                <button
                  type="button"
                  className="pill quiet"
                  aria-label={`Accept ${hunk.id}`}
                  aria-pressed={hunk.decision === 'accepted'}
                  disabled={!!hunk.conflict}
                  onClick={() =>
                    onDecideHunk(hunk.id, hunk.decision === 'accepted' ? 'pending' : 'accepted')
                  }
                >
                  Accept
                </button>
                <button
                  type="button"
                  className="pill quiet"
                  aria-label={`Reject ${hunk.id}`}
                  aria-pressed={hunk.decision === 'rejected'}
                  onClick={() =>
                    onDecideHunk(hunk.id, hunk.decision === 'rejected' ? 'pending' : 'rejected')
                  }
                >
                  Reject
                </button>
              </div>
            )}
          </div>
          <div className="diff-lines" role="region" aria-label={`Hunk ${hunk.header}`}>
            {hunk.lines.map((line, index) => (
              <div key={`${hunk.id}-${index}`} className={`diff-line diff-line-${line.kind}`}>
                <span className="diff-ln" aria-hidden="true">
                  {line.oldLine ?? ''}
                </span>
                <span className="diff-ln" aria-hidden="true">
                  {line.newLine ?? ''}
                </span>
                <span className="diff-sigil" aria-hidden="true">
                  {line.kind === 'delete' ? '−' : line.kind === 'insert' ? '+' : ' '}
                </span>
                <span className="diff-text">{line.text}</span>
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
