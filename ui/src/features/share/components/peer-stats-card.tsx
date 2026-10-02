'use client'

import { PeerStats } from '@/schema/peers.ts'
import {
  ArrowDownIcon,
  ArrowUpIcon,
  ClockFadingIcon,
  EthernetPortIcon,
  GaugeIcon,
  MapPinIcon,
  WifiHighIcon,
} from 'lucide-react'
import { useCountUp } from '@/features/share/hooks/use-count-up.ts'

interface StatsCardProps {
  isLoading: boolean
  stats: PeerStats | undefined
}

function remainingDays(expireTime: string | null | undefined): number {
  if (!expireTime) return 0
  const expireDate = new Date(expireTime)
  const now = new Date()
  const diffTime = expireDate.getTime() - now.getTime()
  return Math.ceil(diffTime / (1000 * 60 * 60 * 24))
}

// Redesigned to match v2ray-sub/dns-share's card language and translated
// to Persian (matching the rest of the panel and the other two share
// pages) -- previously the only share page still in English with plain
// shadcn styling, per the explicit "redesign to match the other protocols"
// ask.
export default function PeerStatsCard({ isLoading, stats }: StatsCardProps) {
  const isOnline = !!stats?.is_online
  const usagePercent = stats?.traffic_limit ? Number(stats.usage_percent) : 0
  const totalUsed = useCountUp(Number(stats?.total_usage ?? 0))

  return (
    <div className='wgs-card'>
      <div className='wgs-row'>
        <div className='wgs-header-title'>وضعیت اتصال</div>
        <span className={`wgs-pill ${isOnline ? 'wgs-pill-active' : 'wgs-pill-inactive'}`}>
          <span className={`wgs-pulse-dot ${isOnline ? '' : 'wgs-dot-danger'}`} />
          {isOnline ? 'آنلاین' : 'آفلاین'}
        </span>
      </div>

      {isLoading ? (
        <p className='wgs-meter-note' style={{ marginTop: 16 }}>در حال بارگذاری...</p>
      ) : (
        <>
          {stats?.traffic_limit && (
            <div style={{ marginTop: 16 }}>
              <div className='wgs-label'>
                <GaugeIcon className='h-3.5 w-3.5' />
                مجموع مصرف
              </div>
              <div className='wgs-meter-value'>
                {totalUsed.toFixed(2)} گیگابایت
              </div>
              <div className='wgs-meter-note'>
                از {stats.traffic_limit} گیگابایت ({usagePercent}%)
              </div>
              <div className='wgs-progress-track'>
                <div className='wgs-progress-fill' style={{ width: `${usagePercent}%` }} />
              </div>
            </div>
          )}

          <div className={stats?.traffic_limit ? 'wgs-divider' : ''} style={{ marginTop: stats?.traffic_limit ? undefined : 16 }}>
            {stats?.location && (
              <div className='wgs-row'>
                <span className='wgs-label'>
                  <MapPinIcon className='h-3.5 w-3.5' />
                  موقعیت
                </span>
                <span className='wgs-value'>{stats.location}</span>
              </div>
            )}

            <div className='wgs-row'>
              <span className='wgs-label'>
                <WifiHighIcon className='h-3.5 w-3.5' />
                سقف حجم
              </span>
              <span className='wgs-value'>
                {stats?.traffic_limit ? `${stats.traffic_limit} گیگابایت` : 'نامحدود'}
              </span>
            </div>

            <div className='wgs-row'>
              <span className='wgs-label'>
                <ClockFadingIcon className='h-3.5 w-3.5' />
                زمان انقضا
              </span>
              <span className='wgs-value'>
                {stats?.expire_time
                  ? `${stats.expire_time} (${remainingDays(stats.expire_time)} روز مانده)`
                  : 'هرگز'}
              </span>
            </div>

            <div className='wgs-row'>
              <span className='wgs-label'>
                <ArrowDownIcon className='h-3.5 w-3.5' />
                دانلود
              </span>
              <span className='wgs-value'>{stats?.download_usage ?? 0} گیگابایت</span>
            </div>

            <div className='wgs-row'>
              <span className='wgs-label'>
                <ArrowUpIcon className='h-3.5 w-3.5' />
                آپلود
              </span>
              <span className='wgs-value'>{stats?.upload_usage ?? 0} گیگابایت</span>
            </div>

            <div className='wgs-row'>
              <span className='wgs-label'>
                <EthernetPortIcon className='h-3.5 w-3.5' />
                وضعیت شبکه
              </span>
              <span className='wgs-value'>{isOnline ? 'متصل' : 'قطع'}</span>
            </div>
          </div>
        </>
      )}
    </div>
  )
}
