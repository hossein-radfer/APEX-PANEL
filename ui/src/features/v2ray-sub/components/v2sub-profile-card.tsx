import { V2RayPackageShareDetails } from '@/schema/v2ray.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { useCountUp } from '@/features/v2ray-sub/hooks/use-count-up.ts'

interface Props {
  details: V2RayPackageShareDetails
}

function toGb(bytes: number): number {
  return bytes / BYTES_PER_GB
}

export function V2SubProfileCard({ details }: Props) {
  const isActive = details.status === 'active'
  const remainingBytes = Math.max(0, details.total_volume_bytes - details.used_bytes)
  const remainingGb = useCountUp(toGb(remainingBytes))
  const totalGb = toGb(details.total_volume_bytes)
  const usagePercent = details.total_volume_bytes > 0
    ? Math.min(100, (details.used_bytes / details.total_volume_bytes) * 100)
    : 0

  const daysRemaining = useCountUp(details.days_remaining ?? 0)
  const hasExpiry = details.days_remaining != null
  // No "total duration" figure is available here (only days-remaining),
  // so this bar is a simple presence indicator, not a proportional
  // countdown: full while time remains, empty once expired.
  const daysBarPercent = !hasExpiry || details.days_remaining! > 0 ? 100 : 0

  return (
    <div className='v2sub-card'>
      <div className='v2sub-row'>
        <div className='v2sub-header-title'>
          {details.customer_label || 'اشتراک شما'}
        </div>
        <span className={`v2sub-pill ${isActive ? 'v2sub-pill-active' : 'v2sub-pill-inactive'}`}>
          <span
            className='v2sub-pulse-dot'
            style={!isActive ? { background: 'var(--v2sub-danger)' } : undefined}
          />
          {isActive ? 'فعال' : 'غیرفعال'}
        </span>
      </div>

      <div className='v2sub-meters'>
        <div className='v2sub-box'>
          <div className='v2sub-label'>حجم باقی‌مانده</div>
          <div className='v2sub-meter-value'>
            {remainingGb.toFixed(2)} گیگابایت
          </div>
          <div className='v2sub-meter-note'>
            حجم کل {totalGb.toFixed(2)} گیگابایت
          </div>
          <div className='v2sub-progress-track'>
            <div
              className='v2sub-progress-fill'
              style={{ width: `${usagePercent}%` }}
            />
          </div>
        </div>

        <div className='v2sub-box'>
          <div className='v2sub-label'>زمان باقی‌مانده</div>
          <div className='v2sub-meter-value'>
            {hasExpiry ? `${Math.round(daysRemaining)} روز` : 'نامحدود'}
          </div>
          <div className='v2sub-meter-note'>
            {details.expire_at ? `انقضا: ${details.expire_at}` : 'بدون تاریخ انقضا'}
          </div>
          <div className='v2sub-progress-track'>
            <div
              className='v2sub-progress-fill'
              style={{ width: `${daysBarPercent}%` }}
            />
          </div>
        </div>
      </div>
    </div>
  )
}
