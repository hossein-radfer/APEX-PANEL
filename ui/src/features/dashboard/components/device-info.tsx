import { DeviceData } from '@/schema/dashboard.ts'
import { Card, CardContent } from '@/components/ui/card'

interface Props {
  stats: DeviceData | undefined
}

export default function DeviceInfo({ stats }: Props) {
  const items = [
    { label: 'شناسه دستگاه', value: stats?.DeviceIdentity?.identity },
    {
      label: 'دستگاه',
      value: stats?.DeviceInfo?.board_name,
      badge: stats?.DeviceInfo?.cpu_arch,
    },
    {
      label: 'نسخه سیستم‌عامل',
      // Confirmed, reported bug: os_version (RouterOS's own raw /system/
      // resource `version` field) already includes the release channel,
      // e.g. "7.22.1 (stable)" -- the separate hardcoded 'stable' badge
      // below was pure redundant decoration (never derived from real
      // per-device data, always the same literal string), so it's been
      // removed rather than duplicating what the value already shows.
      value: stats?.DeviceInfo?.os_version,
    },
    {
      label: 'آی‌پی عمومی',
      value: stats?.DeviceIPv4Address?.ipv4,
      badge: stats?.DeviceIPv4Address?.isp,
      badgeStyle: 'bg-cyan-700',
    },
    { label: 'سرورهای DNS', value: stats?.DNSConfig?.dns_servers },
  ]

  return (
    <Card className='col-span-1 lg:col-span-3'>
      <CardContent>
        <h2 className='text-foreground mb-4 text-lg font-semibold'>
          آمار میکروتیک
        </h2>
        <div>
          {items.map(({ label, value, badge, badgeStyle }, idx) => (
            <div
              key={idx}
              className='border-border/60 flex items-start justify-between border-b py-4 last:border-b-0'
            >
              <span className='text-muted-foreground text-sm font-medium'>
                {label}
              </span>
              <div className='flex flex-col items-end text-sm'>
                <span
                  className={value ? 'text-foreground' : 'text-muted-foreground'}
                >
                  {value ?? 'نامشخص'}
                </span>
                {badge && (
                  <span
                    className={`mt-1 rounded-md px-2 py-0.5 text-xs text-white ${
                      badgeStyle || 'bg-purple-600'
                    }`}
                  >
                    {badge}
                  </span>
                )}
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}
