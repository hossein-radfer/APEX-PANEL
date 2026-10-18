import { serverBadgeColorClass } from '@/lib/server-badge-colors.ts'

// ServerBadge shows a small colored dot + the server's name next to any
// resource list entry (interfaces, peers, user manager accounts) whose
// name could duplicate across servers now that the panel supports more
// than one -- per the admin's own explicit decision, duplicate names
// across servers are allowed and only visually distinguished by ID/color,
// never merged or rejected.
interface Props {
  serverId: number
  serverName: string
  className?: string
}

export function ServerBadge({ serverId, serverName, className }: Props) {
  return (
    <span className={`inline-flex items-center gap-1.5 ${className ?? ''}`}>
      <span
        className={`inline-block size-2 shrink-0 rounded-full ${serverBadgeColorClass(serverId)}`}
        aria-hidden='true'
      />
      <span className='text-muted-foreground text-xs'>{serverName}</span>
    </span>
  )
}
