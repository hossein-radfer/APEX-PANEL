import { JSX } from 'react'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

type StatsCardProps = {
  title: string
  icon: JSX.Element
  value: number | string | undefined
  isLoading: boolean
}

export function StatsCard({ title, icon, value, isLoading }: StatsCardProps) {
  return (
    <Card className='border-border/60 py-0 transition-[transform,box-shadow] duration-150 hover:scale-[1.02] hover:shadow-lg'>
      <CardContent className='flex items-center gap-5 px-6 py-5'>
        <div className='bg-primary/10 text-primary flex size-11 shrink-0 items-center justify-center rounded-lg [&>svg]:size-5'>
          {icon}
        </div>
        <div className='min-w-0'>
          <p className='text-muted-foreground truncate text-sm font-medium leading-snug'>
            {title}
          </p>
          {isLoading ? (
            <Skeleton className='mt-1.5 h-7 w-12 rounded-sm' />
          ) : value === undefined || value === null ? (
            // Confirmed, reported bug: this card previously rendered
            // nothing at all (an empty <p>) whenever value was
            // undefined/null -- e.g. GetDeviceData (api/service/
            // device_data.go) now correctly returns a null section
            // instead of failing the whole dashboard request when one
            // Mikrotik router is unreachable, but that null must still
            // be visibly distinguishable from "loading" or "zero" here.
            <p className='text-muted-foreground text-2xl font-bold leading-snug tracking-tight'>
              —
            </p>
          ) : (
            <p className='text-foreground text-2xl font-bold leading-snug tracking-tight'>
              {value}
            </p>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
