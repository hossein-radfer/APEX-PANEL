interface StatusPulseDotProps {
  active: boolean
}

// A breathing status dot for "is this server actually up and reachable
// right now" -- green + pulse animation when active, static gray when not
// (e.g. stats failed to load). Purely presentational; active is derived
// from whether the dashboard's stats query actually returned data.
export function StatusPulseDot({ active }: StatusPulseDotProps) {
  if (!active) {
    return (
      <span className='relative flex size-2.5'>
        <span className='bg-muted-foreground/50 relative inline-flex size-2.5 rounded-full' />
      </span>
    )
  }

  return (
    <span className='relative flex size-2.5'>
      <span className='absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-500 opacity-75' />
      <span className='relative inline-flex size-2.5 rounded-full bg-emerald-500' />
    </span>
  )
}
