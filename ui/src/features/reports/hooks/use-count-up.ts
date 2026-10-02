import { useEffect, useRef, useState } from 'react'

/**
 * Animates from 0 up to `target` over `durationMs` using
 * requestAnimationFrame -- a reusable "count-up" entrance effect for the
 * Reports section's stat cards/hero numbers, adapted from
 * features/v2ray-sub/hooks/use-count-up.ts (same easeOutCubic easing).
 * Re-triggers whenever `target` changes (e.g. after a range change/refetch).
 */
export function useCountUp(target: number, durationMs = 700): number {
  const [value, setValue] = useState(0)
  const frameRef = useRef<number | null>(null)

  useEffect(() => {
    const start = performance.now()
    const from = 0

    const tick = (now: number) => {
      const elapsed = now - start
      const progress = Math.min(1, elapsed / durationMs)
      // easeOutCubic
      const eased = 1 - Math.pow(1 - progress, 3)
      setValue(from + (target - from) * eased)

      if (progress < 1) {
        frameRef.current = requestAnimationFrame(tick)
      }
    }

    frameRef.current = requestAnimationFrame(tick)

    return () => {
      if (frameRef.current !== null) cancelAnimationFrame(frameRef.current)
    }
  }, [target, durationMs])

  return value
}
