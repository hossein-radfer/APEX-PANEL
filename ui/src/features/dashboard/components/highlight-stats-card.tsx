import { ReactNode } from 'react'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

type HighlightStatsCardProps = {
  title: string
  icon: ReactNode
  value: number | string | undefined
  suffix?: string
  isLoading: boolean
  action?: ReactNode
  subtitle?: ReactNode
}

export function HighlightStatsCard({
  title,
  icon,
  value,
  suffix,
  isLoading,
  action,
  subtitle,
}: HighlightStatsCardProps) {
  let displayValue: string | number = '-'
  if (value !== undefined && value !== null && value !== '') {
    displayValue = suffix ? `${value} ${suffix}` : value
  }

  return (
    <Card
      className='border-border/60 py-0 transition-[transform,box-shadow] duration-150 hover:scale-[1.02] hover:shadow-lg'
    >
      <CardContent className='flex flex-wrap items-center justify-between gap-5 px-6 py-5'>
        <div className='flex min-w-0 items-center gap-5'>
          <div className='bg-primary/10 text-primary flex size-11 shrink-0 items-center justify-center rounded-lg [&>svg]:size-5'>
            {icon}
          </div>
          <div className='min-w-0'>
            <p className='text-muted-foreground truncate text-sm font-medium leading-snug'>
              {title}
            </p>
            {isLoading ? (
              <Skeleton className='mt-1.5 h-7 w-24 rounded-sm' />
            ) : (
              <>
                <p className='text-foreground truncate text-2xl font-bold leading-snug tracking-tight'>
                  {displayValue}
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
        {action}
      </CardContent>
    </Card>
  )
}
