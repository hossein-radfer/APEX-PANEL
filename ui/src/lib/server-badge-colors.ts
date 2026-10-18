// Stable hash of a server ID to a Tailwind color class -- the SAME server
// always gets the SAME color everywhere it's shown (interface/peer/user
// manager pickers and tables), without needing a shared color-assignment
// table. Used by ServerBadge (shared-components/server-badge.tsx) and
// directly by pickers that render their own dot (e.g. InterfaceSelect).
const SERVER_BADGE_COLORS = [
  'bg-blue-500',
  'bg-emerald-500',
  'bg-amber-500',
  'bg-violet-500',
  'bg-rose-500',
  'bg-cyan-500',
  'bg-lime-500',
  'bg-fuchsia-500',
]

export function serverBadgeColorClass(serverId: number): string {
  const index = serverId % SERVER_BADGE_COLORS.length
  return SERVER_BADGE_COLORS[index]
}
