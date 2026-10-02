import { ReactNode } from 'react'
import { useCountUp } from '@/features/reports/hooks/use-count-up.ts'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

type ReportStatCardProps = {
  title: string
  icon: ReactNode
  value: number | undefined
  isLoading: boolean
  suffix?: string
  subtitle?: ReactNode
  formatValue?: (animatedValue: number) => string
  live?: boolean
  decimals?: number
}

// Shared stat-card for the Reports section's single-number summaries
// (online users, renewal rate, peak resource usage) -- mirrors
// dashboard/components/highlight-stats-card.tsx's visual language but adds
// the count-up animation and an optional "live" pulsing dot indicator.
export function ReportStatCard({
  title,
  icon,
  value,
  isLoading,
  suffix,
  subtitle,
  formatValue,
  live,
  decimals = 0,
}: ReportStatCardProps) {
  const animated = useCountUp(value ?? 0)

  const displayValue =
    value === undefined
      ? '—'
      : formatValue
        ? formatValue(animated)
        : animated.toLocaleString('fa-IR', {
            maximumFractionDigits: decimals,
          })

  return (
    <Card className='border-border/60 animate-in fade-in slide-in-from-bottom-2 py-0 transition-[transform,box-shadow] duration-500 hover:scale-[1.02] hover:shadow-lg hover:duration-150'>
      <CardContent className='flex flex-wrap items-center justify-between gap-5 px-6 py-5'>
        <div className='flex min-w-0 items-center gap-5'>
          <div className='bg-primary/10 text-primary relative flex size-11 shrink-0 items-center justify-center rounded-lg [&>svg]:size-5'>
            {icon}
            {live && (
              <span className='absolute -end-0.5 -top-0.5 flex size-3'>
                <span className='bg-chart-2 absolute inline-flex h-full w-full animate-ping rounded-full opacity-75' />
                <span className='bg-chart-2 relative inline-flex size-3 rounded-full' />
              </span>
            )}
          </div>
          <div className='min-w-0'>
            <p className='text-muted-foreground truncate text-sm font-medium leading-snug'>
              {title}
            </p>
            {isLoading ? (
              <Skeleton className='mt-1.5 h-7 w-24 rounded-sm' />
            ) : (
              <>
                <p className='text-foreground truncate text-2xl font-bold leading-snug tracking-tight tabular-nums'>
                  {displayValue}
                  {suffix ? (
                    <span className='ms-1 text-base font-medium'>
                      {suffix}
                    </span>
                  ) : null}
                </p>
                {subtitle && (
                  <p className='text-muted-foreground mt-0.5 text-xs break-words'>
                    {subtitle}
                  </p>
                )}
              </>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
