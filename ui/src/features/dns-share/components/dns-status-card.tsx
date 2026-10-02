'use client'

import { DNSAccountShareDetails } from '@/schema/dns-account.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { GaugeIcon, GlobeIcon, ShieldCheckIcon } from 'lucide-react'
import { useCountUp } from '@/features/dns-share/hooks/use-count-up.ts'

interface Props {
  details: DNSAccountShareDetails
}

function toGb(bytes: number): number {
  return bytes / BYTES_PER_GB
}

function formatSpeed(kbps: number): string {
  if (kbps <= 0) return 'نامحدود'
  return kbps >= 1000 ? `${(kbps / 1000).toFixed(1)} مگابیت/ثانیه` : `${kbps} کیلوبیت/ثانیه`
}

function formatToman(amount: number): string {
  return `${amount.toLocaleString('fa-IR')} تومان`
}

// The main status box on the DNS share page -- redesigned to match
// v2ray-sub's own visual language (animated meters, pulse-dot status,
// gradient progress bars) instead of a plain shadcn Card, per the explicit
// "redesign DNS to match the other protocols' pages, more appealing" ask.
// is_online here is a coarse "has a registered IP and is active" proxy,
// NOT a live connection signal -- see
// DNSAccountShareDetailsResponse.IsOnline's own doc comment -- so it's
// labeled "IP ثبت‌شده" (IP registered), never "آنلاین" (online).
export default function DNSStatusCard({ details }: Props) {
  const remainingRegistrationsToday =
    details.daily_ip_registration_limit > 0
      ? Math.max(
          0,
          details.daily_ip_registration_limit - details.registrations_used_today
        )
      : null

  const hasVolume = details.total_volume_bytes > 0
  const usedGb = useCountUp(hasVolume ? toGb(details.used_bytes) : 0)
  const totalGb = toGb(details.total_volume_bytes)
  const usagePercent = hasVolume ? Number(details.usage_percent ?? 0) : 0

  return (
    <div className='dnss-card'>
      <div className='dnss-row'>
        <div className='dnss-header-title'>{details.plan_title}</div>
        <span className={`dnss-pill ${details.is_online ? 'dnss-pill-active' : 'dnss-pill-inactive'}`}>
          <span className={`dnss-pulse-dot ${details.is_online ? '' : 'dnss-dot-muted'}`} />
          {details.is_online ? 'IP ثبت‌شده' : 'IP ثبت‌نشده'}
        </span>
      </div>

      {hasVolume && (
        <div className='dnss-box' style={{ marginTop: 16 }}>
          <div className='dnss-label'>
            <GaugeIcon className='h-3.5 w-3.5' />
            مجموع مصرف
          </div>
          <div className='dnss-meter-value'>
            {usedGb.toFixed(2)} گیگابایت
          </div>
          <div className='dnss-meter-note'>
            از {totalGb.toFixed(2)} گیگابایت ({usagePercent}%)
          </div>
          <div className='dnss-progress-track'>
            <div className='dnss-progress-fill' style={{ width: `${usagePercent}%` }} />
          </div>
        </div>
      )}

      <div className={hasVolume ? 'dnss-divider' : ''} style={{ marginTop: hasVolume ? undefined : 16 }}>
        <div className='dnss-row'>
          <span className='dnss-label'>IP فعلی</span>
          <span className='dnss-value dnss-value-mono'>
            {details.current_ip ?? 'ثبت‌نشده'}
          </span>
        </div>

        <div className='dnss-row'>
          <span className='dnss-label'>زمان انقضا</span>
          <span className='dnss-value'>
            {details.expire_at
              ? details.days_remaining != null
                ? `${details.expire_at} (${details.days_remaining} روز مانده)`
                : details.expire_at
              : 'هرگز'}
          </span>
        </div>

        <div className='dnss-row'>
          <span className='dnss-label'>حداکثر IP هم‌زمان</span>
          <span className='dnss-value'>{details.max_concurrent_ips}</span>
        </div>

        {details.speed_kbps != null && details.speed_kbps > 0 && (
          <div className='dnss-row'>
            <span className='dnss-label'>سرعت</span>
            <span className='dnss-value'>{formatSpeed(details.speed_kbps)}</span>
          </div>
        )}

        {details.price_amount != null && details.price_amount > 0 && (
          <div className='dnss-row'>
            <span className='dnss-label'>قیمت</span>
            <span className='dnss-value'>{formatToman(details.price_amount)}</span>
          </div>
        )}

        <div className='dnss-row'>
          <span className='dnss-label'>ثبت امروز</span>
          <span className='dnss-value'>
            {details.daily_ip_registration_limit > 0
              ? `${details.registrations_used_today} / ${details.daily_ip_registration_limit} (${remainingRegistrationsToday} باقی‌مانده)`
              : `${details.registrations_used_today} (نامحدود)`}
          </span>
        </div>
      </div>

      {details.allowed_countries && details.allowed_countries.length > 0 && (
        <div className='dnss-divider'>
          <div className='dnss-label' style={{ marginBottom: 8 }}>
            <GlobeIcon className='h-3.5 w-3.5' />
            کشورهای مجاز برای ثبت IP
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
            {details.allowed_countries.map((country) => (
              <span key={country} className='dnss-badge-outline'>
                <ShieldCheckIcon className='h-3 w-3' />
                {country}
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
