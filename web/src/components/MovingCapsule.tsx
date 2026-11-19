import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useSyncExternalStore,
  type RefObject,
} from 'react'

const REDUCED_MOTION_QUERY = '(prefers-reduced-motion: reduce)'
const FALLBACK_DURATION = 120
const FALLBACK_EASING = 'cubic-bezier(0.42, 0, 0.58, 1)'

function motionDuration(value: string): number {
  const parsed = value.endsWith('ms') ? Number.parseFloat(value) : Number.parseFloat(value) * 1000
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : FALLBACK_DURATION
}

function subscribeReducedMotion(onChange: () => void): () => void {
  const query = window.matchMedia(REDUCED_MOTION_QUERY)
  if (typeof query.addEventListener === 'function') {
    query.addEventListener('change', onChange)
    return () => query.removeEventListener('change', onChange)
  }
  query.addListener(onChange)
  return () => query.removeListener(onChange)
}

function prefersReducedMotion(): boolean {
  return window.matchMedia(REDUCED_MOTION_QUERY).matches
}

function serverReducedMotion(): boolean {
  return false
}

export function MovingCapsule({
  trackRef,
  selector = '[aria-checked="true"], [aria-selected="true"]',
  axis = 'x',
  watch,
}: {
  trackRef: RefObject<HTMLElement | null>
  selector?: string
  axis?: 'x' | 'y'
  watch: readonly unknown[]
}): React.ReactElement {
  const capsuleRef = useRef<HTMLSpanElement>(null)
  const positioned = useRef(false)
  const animation = useRef<Animation | null>(null)
  const observerRef = useRef<ResizeObserver | null>(null)
  const observedSelected = useRef<HTMLElement | null>(null)
  const pendingObservation = useRef(new WeakSet<Element>())
  const reduced = useSyncExternalStore(subscribeReducedMotion, prefersReducedMotion, serverReducedMotion)

  const place = useCallback((animate: boolean) => {
    const track = trackRef.current
    const capsule = capsuleRef.current
    const selected = track?.querySelector<HTMLElement>(selector)
    if (!track || !capsule) return
    if (!selected || selected.getClientRects().length === 0) {
      capsule.hidden = true
      positioned.current = false
      animation.current?.cancel()
      animation.current = null
      return
    }
    if (observerRef.current && observedSelected.current !== selected) {
      if (observedSelected.current) observerRef.current.unobserve(observedSelected.current)
      observerRef.current.observe(selected)
      pendingObservation.current.add(selected)
      observedSelected.current = selected
    }

    const trackBounds = track.getBoundingClientRect()
    const selectedBounds = selected.getBoundingClientRect()
    const targetX = selectedBounds.left - trackBounds.left
    const targetY = selectedBounds.top - trackBounds.top
    const targetWidth = selectedBounds.width
    const targetHeight = selectedBounds.height
    const current = positioned.current ? capsule.getBoundingClientRect() : null
    const fromX = current ? current.left - trackBounds.left : targetX
    const fromY = current ? current.top - trackBounds.top : targetY
    const fromWidth = current?.width ?? targetWidth
    const fromHeight = current?.height ?? targetHeight

    capsule.hidden = false
    capsule.style.width = `${targetWidth}px`
    capsule.style.height = `${targetHeight}px`
    capsule.style.transform = `translate(${targetX}px, ${targetY}px) scale(1, 1)`
    positioned.current = true

    animation.current?.cancel()
    animation.current = null
    const changed = fromX !== targetX || fromY !== targetY || fromWidth !== targetWidth || fromHeight !== targetHeight
    if (!animate || !current || prefersReducedMotion() || !changed) return
    if (document.documentElement.dataset.input === 'keyboard') return
    const fromScaleX = axis === 'x' ? fromWidth / targetWidth : 1
    const fromScaleY = axis === 'y' ? fromHeight / targetHeight : 1
    const styles = getComputedStyle(track)
    const duration = motionDuration(styles.getPropertyValue('--duration-switch').trim())
    const easing = styles.getPropertyValue('--ease-in-out').trim() || FALLBACK_EASING
    const deltaX = Math.abs(targetX - fromX)
    const deltaY = Math.abs(targetY - fromY)
    const keyframes: Keyframe[] =
      axis === 'x' && deltaX > 18
        ? [
            { transform: `translate(${fromX}px, ${fromY}px) scale(${fromScaleX}, ${fromScaleY})` },
            {
              offset: 0.36,
              transform: `translate(${fromX + (targetX - fromX) * 0.56}px, ${targetY}px) scale(${(fromScaleX + 1) * 0.52}, 0.96)`,
            },
            { transform: `translate(${targetX}px, ${targetY}px) scale(1, 1)` },
          ]
        : axis === 'y' && deltaY > 18
          ? [
              { transform: `translate(${fromX}px, ${fromY}px) scale(${fromScaleX}, ${fromScaleY})` },
              {
                offset: 0.36,
                transform: `translate(${targetX}px, ${fromY + (targetY - fromY) * 0.56}px) scale(0.985, ${(fromScaleY + 1) * 0.51})`,
              },
              { transform: `translate(${targetX}px, ${targetY}px) scale(1, 1)` },
            ]
          : [
              { transform: `translate(${fromX}px, ${fromY}px) scale(${fromScaleX}, ${fromScaleY})` },
              { transform: `translate(${targetX}px, ${targetY}px) scale(1, 1)` },
            ]
    animation.current = capsule.animate(keyframes, { duration, easing })

  }, [axis, selector, trackRef])

  useLayoutEffect(() => {
    place(true)
  }, [place, ...watch])

  useEffect(() => {
    if (!reduced) return
    animation.current?.cancel()
    animation.current = null
    place(false)
  }, [place, reduced])

  useEffect(() => {
    const track = trackRef.current
    if (!track) return
    const observer = new ResizeObserver((entries) => {
      let resized = false
      for (const entry of entries) {
        if (pendingObservation.current.has(entry.target)) {
          pendingObservation.current.delete(entry.target)
        } else {
          resized = true
        }
      }
      if (resized) place(false)
    })
    observerRef.current = observer
    observer.observe(track)
    const selected = track.querySelector<HTMLElement>(selector)
    if (selected) {
      observer.observe(selected)
      pendingObservation.current.add(selected)
      observedSelected.current = selected
    }
    return () => {
      observer.disconnect()
      observerRef.current = null
      observedSelected.current = null
      animation.current?.cancel()
      animation.current = null
    }
  }, [place, trackRef])

  return <span className="moving-capsule" aria-hidden="true" ref={capsuleRef} hidden />
}
