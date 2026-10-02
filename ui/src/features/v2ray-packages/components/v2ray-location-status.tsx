import { V2RayPackageLocationStatus } from '@/schema/v2ray.ts'
import { cn } from '@/lib/utils'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip.tsx'

interface Props {
  locations: V2RayPackageLocationStatus[]
}

// A small colored dot per registered panel this package is provisioned on:
// green if enabled and synced without error, red if the last sync failed,
// gray if the location is disabled for this package. A second, smaller
// dot layered on the corner shows is_online (the periodic sync job's most
// recent x-ui onlines poll) -- a separate signal from panel health: a
// location can be perfectly healthy (enabled, no sync error) while nobody
// is currently connected, or vice versa briefly around a sync error.
export function V2RayLocationStatus({ locations }: Props) {
  if (locations.length === 0) {
    return <span className='text-muted-foreground text-xs'>بدون لوکیشن</span>
  }

  return (
    <div className='flex flex-wrap items-center gap-2'>
      {locations.map((location) => {
        const color = !location.enabled
          ? 'bg-slate-300 dark:bg-slate-500'
          : location.last_sync_error
            ? 'bg-red-500 dark:bg-red-400'
            : 'bg-green-500 dark:bg-green-400'

        const statusLabel = !location.enabled
          ? 'غیرفعال'
          : location.last_sync_error
            ? `خطای همگام‌سازی: ${location.last_sync_error}`
            : 'همگام‌سازی‌شده'

        const onlineLabel = location.is_online
          ? 'کلاینت هم‌اکنون متصل است'
          : 'در حال حاضر کلاینتی متصل نیست'

        return (
          <Tooltip key={location.panel_id}>
            <TooltipTrigger asChild>
              <span className='relative inline-flex size-2.5'>
                <span
                  className={cn('inline-block size-2.5 rounded-full', color)}
                />
                <span
                  className={cn(
                    'absolute -right-1 -bottom-1 size-1.5 rounded-full ring-1 ring-background',
                    location.is_online
                      ? 'bg-sky-500 dark:bg-sky-400'
                      : 'bg-transparent'
                  )}
                />
              </span>
            </TooltipTrigger>
            <TooltipContent>
              <p className='max-w-64'>
                {location.panel_name}: {statusLabel}
                <br />
                {onlineLabel}
              </p>
            </TooltipContent>
          </Tooltip>
        )
      })}
    </div>
  )
}
