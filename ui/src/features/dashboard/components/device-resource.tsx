import { DeviceData } from '@/schema/dashboard.ts'
import { buildDeviceResourceGauges } from '@/utils/helper.ts'
import { Card, CardContent } from '@/components/ui/card'
import { ResourceGauge } from '@/features/dashboard/components/resource-gauge.tsx'
import { StatusPulseDot } from '@/features/dashboard/components/status-pulse-dot.tsx'

interface Props {
  stats: DeviceData['DeviceInfo'] | undefined
}

export default function DeviceResource({ stats }: Props) {
  const gauges = buildDeviceResourceGauges(stats)
  // Confirmed, reported bug: stats is now also `null` (not just
  // `undefined`) whenever GetDeviceData's own getDeviceInfo call failed
  // to reach the Mikrotik router (api/service/device_data.go) --
  // `stats !== undefined` alone missed that case, showing "Up" with a
  // blank uptime instead of "Unreachable".
  const isUp = stats !== undefined && stats !== null

  return (
    <Card className='col-span-1 lg:col-span-2'>
      <CardContent>
        <div className='mb-5 flex items-center justify-between'>
          <h2 className='text-foreground text-lg font-semibold'>
            آمار سخت‌افزاری
          </h2>
          <div className='flex items-center gap-2'>
            <StatusPulseDot active={isUp} />
            <span className='text-muted-foreground text-sm'>
              {isUp ? `فعال ${stats?.uptime || ''}` : 'در دسترس نیست'}
            </span>
          </div>
        </div>

        <div className='flex flex-wrap items-start justify-around gap-6 py-2'>
          <ResourceGauge
            label={gauges.cpu.label}
            percent={gauges.cpu.percent}
            detail={gauges.cpu.detail ?? undefined}
            colorClassName='fill-sky-500 text-sky-500'
          />
          <ResourceGauge
            label={gauges.memory.label}
            percent={gauges.memory.percent}
            detail={gauges.memory.detail ?? undefined}
            colorClassName='fill-violet-500 text-violet-500'
          />
          <ResourceGauge
            label={gauges.disk.label}
            percent={gauges.disk.percent}
            detail={gauges.disk.detail ?? undefined}
            colorClassName='fill-amber-500 text-amber-500'
          />
        </div>
      </CardContent>
    </Card>
  )
}
