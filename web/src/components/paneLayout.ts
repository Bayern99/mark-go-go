export const SIDEBAR_MIN = 190
export const SIDEBAR_MAX = 420
export const SIDEBAR_DEFAULT = 248
export const SIDEBAR_FLOOR = 140

export const RIGHT_MIN = 310
export const RIGHT_MAX = 800
export const RIGHT_DEFAULT = 376
export const RIGHT_MAX_RATIO = 0.56

export const CENTER_MIN = 412
export const SIDEBAR_CHROME = 16
export const RIGHT_CHROME = 16
export const SIDE_DOCKED_MIN_WIDTH = 641

const RESEARCH_WIDTH_KEY = 'mgo.researchWidth'

export function readSavedRightWidth(): number {
  if (typeof window === 'undefined') return RIGHT_DEFAULT
  try {
    const saved = Number(window.localStorage.getItem(RESEARCH_WIDTH_KEY))
    return saved >= RIGHT_MIN && saved <= RIGHT_MAX ? saved : RIGHT_DEFAULT
  } catch {
    return RIGHT_DEFAULT
  }
}

export function saveRightWidth(width: number): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(RESEARCH_WIDTH_KEY, String(Math.round(width)))
  } catch {
  }
}

export interface PaneWidthInput {
  winWidth: number
  targetLeft: number
  targetRight: number
  leftDocked: boolean
  rightDocked: boolean
}

export interface PaneWidthResult {
  left: number
  right: number
  maxLeft: number
  maxRight: number
}

/**
 * Resolves docked SidePane and RightPane widths against a shared window budget
 * so dragging or resizing never squeezes the center Document below CENTER_MIN.
 */
export function resolvePaneWidths({
  winWidth,
  targetLeft,
  targetRight,
  leftDocked,
  rightDocked,
}: PaneWidthInput): PaneWidthResult {
  const rightCap = Math.min(RIGHT_MAX, Math.floor(winWidth * RIGHT_MAX_RATIO))
  const chrome = (leftDocked ? SIDEBAR_CHROME : 0) + (rightDocked ? RIGHT_CHROME : 0)
  const budget = Math.max(0, winWidth - chrome - CENTER_MIN)

  if (!leftDocked && !rightDocked) {
    return {
      left: 0,
      right: 0,
      maxLeft: SIDEBAR_MAX,
      maxRight: Math.max(RIGHT_MIN, rightCap),
    }
  }

  if (leftDocked && !rightDocked) {
    const maxLeft = Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_FLOOR, budget))
    const left = Math.min(maxLeft, Math.max(SIDEBAR_FLOOR, Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, targetLeft))))
    return {
      left,
      right: 0,
      maxLeft,
      maxRight: Math.max(RIGHT_MIN, rightCap),
    }
  }

  if (!leftDocked && rightDocked) {
    const maxRight = Math.max(RIGHT_MIN, Math.min(rightCap, budget))
    const right = Math.min(maxRight, Math.max(RIGHT_MIN, targetRight))
    return {
      left: 0,
      right,
      maxLeft: SIDEBAR_MAX,
      maxRight,
    }
  }

  let left = Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, targetLeft))
  let right = Math.min(Math.max(RIGHT_MIN, rightCap), Math.max(RIGHT_MIN, targetRight))

  if (left + right > budget) {
    let excess = left + right - budget
    const leftSlack = Math.max(0, left - SIDEBAR_MIN)
    const rightSlack = Math.max(0, right - RIGHT_MIN)
    const totalSlack = leftSlack + rightSlack

    if (totalSlack > 0) {
      const take = Math.min(excess, totalSlack)
      const takeLeft = Math.min(leftSlack, Math.round((take * leftSlack) / totalSlack))
      const takeRight = Math.min(rightSlack, take - takeLeft)
      const remainder = take - takeLeft - takeRight
      const extraLeft = Math.min(leftSlack - takeLeft, remainder)
      const extraRight = Math.min(rightSlack - takeRight, remainder - extraLeft)
      left -= takeLeft + extraLeft
      right -= takeRight + extraRight
      excess -= take
    }

    if (excess > 0) {
      const floorDrop = Math.min(excess, Math.max(0, left - SIDEBAR_FLOOR))
      left -= floorDrop
    }
  }

  const maxLeft = Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_FLOOR, budget - right))
  const maxRight = Math.max(RIGHT_MIN, Math.min(rightCap, budget - left))

  return { left, right, maxLeft, maxRight }
}
