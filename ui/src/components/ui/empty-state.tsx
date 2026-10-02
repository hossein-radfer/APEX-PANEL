import { ReactNode } from 'react'
import { IconMoodEmpty } from '@tabler/icons-react'

type EmptyStateProps = {
  message?: string
  icon?: ReactNode
}

// Shared empty-data placeholder -- originally built only for the Reports
// section (as ReportEmptyState) but genuinely feature-agnostic, so it moved
// here to be the one consistent "no data" treatment across the whole
// panel, replacing the bare centered <p> text several other features
// (Accounting, Security, Faoxima Management, User Manager, Reseller User
// Manager, Application Locations) had each hand-rolled independently.
export function EmptyState({
  message = 'موردی برای نمایش وجود ندارد.',
  icon,
}: EmptyStateProps) {
  return (
    <div className='text-muted-foreground flex h-full min-h-[180px] w-full flex-col items-center justify-center gap-2 py-10 text-center'>
      {icon ?? <IconMoodEmpty className='size-10 opacity-60' />}
      <p className='text-sm'>{message}</p>
    </div>
  )
}
