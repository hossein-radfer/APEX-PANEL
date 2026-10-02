import { useEffect, useRef, useState } from 'react'

/**
 * Animates from 0 up to `target` over `durationMs` using
 * requestAnimationFrame -- mirrors v2ray-sub's own identical hook so the
 * DNS status card gets the same entrance polish, kept as a separate copy
 * rather than a shared import so dns-share stays independently movable
 * from v2ray-sub (matching how both features already duplicate their own
 * scoped CSS rather than sharing one).
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
