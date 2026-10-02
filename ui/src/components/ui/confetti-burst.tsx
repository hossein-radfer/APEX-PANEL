import { useEffect, useRef } from 'react'

interface Particle {
  x: number
  y: number
  vx: number
  vy: number
  rotation: number
  rotationSpeed: number
  size: number
  color: string
  shape: 'rect' | 'circle'
}

const COLORS = [
  '#f43f5e',
  '#fb923c',
  '#facc15',
  '#4ade80',
  '#38bdf8',
  '#a78bfa',
]

/**
 * A one-shot confetti burst rendered on a fixed, pointer-events-none
 * canvas covering the viewport. Mount it conditionally (e.g. only on a
 * new reseller's first login) -- it runs its animation once and never
 * reappears on its own.
 */
export function ConfettiBurst({ onDone }: { onDone?: () => void }) {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const dpr = window.devicePixelRatio || 1
    const resize = () => {
      canvas.width = window.innerWidth * dpr
      canvas.height = window.innerHeight * dpr
      ctx.scale(dpr, dpr)
    }
    resize()

    const particleCount = 140
    const particles: Particle[] = Array.from({ length: particleCount }, () => {
      const angle = Math.random() * Math.PI - Math.PI
      const speed = 4 + Math.random() * 6
      return {
        x: window.innerWidth / 2,
        y: window.innerHeight * 0.35,
        vx: Math.cos(angle) * speed,
        vy: Math.sin(angle) * speed - 6,
        rotation: Math.random() * 360,
        rotationSpeed: (Math.random() - 0.5) * 20,
        size: 6 + Math.random() * 6,
        color: COLORS[Math.floor(Math.random() * COLORS.length)],
        shape: Math.random() > 0.5 ? 'rect' : 'circle',
      }
    })

    const gravity = 0.25
    const drag = 0.995
    let frame = 0
    const maxFrames = 150
    let animationId: number

    const render = () => {
      ctx.clearRect(0, 0, window.innerWidth, window.innerHeight)

      for (const p of particles) {
        p.vx *= drag
        p.vy = p.vy * drag + gravity
        p.x += p.vx
        p.y += p.vy
        p.rotation += p.rotationSpeed

        ctx.save()
        ctx.translate(p.x, p.y)
        ctx.rotate((p.rotation * Math.PI) / 180)
        ctx.fillStyle = p.color
        const fade = Math.max(0, 1 - frame / maxFrames)
        ctx.globalAlpha = fade

        if (p.shape === 'rect') {
          ctx.fillRect(-p.size / 2, -p.size / 4, p.size, p.size / 2)
        } else {
          ctx.beginPath()
          ctx.arc(0, 0, p.size / 2, 0, Math.PI * 2)
          ctx.fill()
        }
        ctx.restore()
      }

      frame += 1
      if (frame < maxFrames) {
        animationId = requestAnimationFrame(render)
      } else {
        onDone?.()
      }
    }

    animationId = requestAnimationFrame(render)
    window.addEventListener('resize', resize)

    return () => {
      cancelAnimationFrame(animationId)
      window.removeEventListener('resize', resize)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <canvas
      ref={canvasRef}
      className='pointer-events-none fixed inset-0 z-[100]'
      style={{ width: '100vw', height: '100vh' }}
      aria-hidden='true'
    />
  )
}
