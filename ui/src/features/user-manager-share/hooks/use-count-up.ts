import { useEffect, useRef, useState } from 'react'

/**
 * Animates from 0 up to `target` over `durationMs` using
 * requestAnimationFrame -- mirrors v2ray-sub's/dns-share's/share's own
 * identical hook, kept as a separate copy so each share feature stays
 * independently movable.
 */
export function useCountUp(target: number, durationMs = 900): number {
  const [value, setValue] = useState(0)
  const frameRef = useRef<number | null>(null)

  useEffect(() => {
    const start = performance.now()
    const from = 0

    const tick = (now: number) => {
      const elapsed = now - start
      const progress = Math.min(1, elapsed / durationMs)
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
