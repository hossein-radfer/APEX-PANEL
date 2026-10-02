'use client'

import { useState } from 'react'
import {
  UserManagerAccountShareDetails,
  UserManagerShareProtocolInfo,
} from '@/schema/user-manager.ts'
import {
  ClipboardCopyIcon,
  DownloadIcon,
  EyeIcon,
  EyeOffIcon,
  GaugeIcon,
} from 'lucide-react'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/clipboard.ts'
import {
  filenameWithFallbackExtension,
  triggerBlobDownload,
} from '@/lib/download.ts'
import {
  fetchUserManagerAccountConfigPublic,
  fetchUserManagerProtocolCertificateFilePublic,
  fetchUserManagerProtocolClientAppFilePublic,
} from '@/api/user-manager.ts'
import { useCountUp } from '@/features/user-manager-share/hooks/use-count-up.ts'

interface Props {
  isLoading: boolean
  details: UserManagerAccountShareDetails | undefined
  uuid: string | undefined
}

function CopyableField({ label, value }: { label: string; value: string }) {
  const handleCopy = async () => {
    const succeeded = await copyToClipboard(value)
    if (succeeded) {
      toast.success(`${label} کپی شد`, { duration: 5000 })
    } else {
      toast.error('کپی ناموفق بود', { duration: 5000 })
    }
  }

  return (
    <div className='ums-row'>
      <span className='ums-label'>{label}</span>
      <div className='ums-copy-row'>
        <span className='ums-value ums-value-mono'>{value}</span>
        <button className='ums-icon-btn' onClick={handleCopy}>
          <ClipboardCopyIcon className='h-3.5 w-3.5' />
        </button>
      </div>
    </div>
  )
}

// Redesigned to match v2ray-sub/dns-share/share's card language and
// translated to Persian (matching the rest of the panel), per the
// explicit "redesign every remaining page to match the other protocols"
// ask -- this was the last share page still in English with plain shadcn
// Cards.
export default function ConnectionInfoCard({ isLoading, details, uuid }: Props) {
  const [isPasswordVisible, setIsPasswordVisible] = useState(false)
  const [isDownloading, setIsDownloading] = useState(false)
  const [isCertificateDownloading, setIsCertificateDownloading] =
    useState(false)
  const [isClientAppDownloading, setIsClientAppDownloading] = useState(false)

  const isOnline = !!details?.is_online
  const hasLimit = !!details?.traffic_limit
  const totalUsed = useCountUp(Number(details?.total_usage ?? 0))
  const usagePercent = hasLimit ? Number(details?.usage_percent ?? 0) : 0

  const handleDownloadConfig = async () => {
    if (!uuid) return
    setIsDownloading(true)
    try {
      const blob = await fetchUserManagerAccountConfigPublic(uuid)
      triggerBlobDownload(blob, `${details?.username || 'account'}.ovpn`)
    } catch {
      toast.error('دانلود فایل تنظیمات ناموفق بود.', { duration: 5000 })
    } finally {
      setIsDownloading(false)
    }
  }

  const handleDownloadCertificate = async (protocol: string) => {
    if (!uuid) return
    setIsCertificateDownloading(true)
    try {
      const { blob, filename } =
        await fetchUserManagerProtocolCertificateFilePublic(uuid, protocol)
      triggerBlobDownload(
        blob,
        filename ?? filenameWithFallbackExtension(`${protocol}-certificate`, blob)
      )
    } catch {
      toast.error('دانلود فایل ناموفق بود.', { duration: 5000 })
    } finally {
      setIsCertificateDownloading(false)
    }
  }

  const handleDownloadClientApp = async (protocol: string) => {
    if (!uuid) return
    setIsClientAppDownloading(true)
    try {
      const { blob, filename } =
        await fetchUserManagerProtocolClientAppFilePublic(uuid, protocol)
      triggerBlobDownload(
        blob,
        filename ?? filenameWithFallbackExtension(`${protocol}-client-app`, blob)
      )
    } catch {
      toast.error('دانلود فایل ناموفق بود.', { duration: 5000 })
    } finally {
      setIsClientAppDownloading(false)
    }
  }

  return (
    <div className='ums-card'>
      <div className='ums-row'>
        <div className='ums-header-title'>اطلاعات اتصال</div>
        <span className={`ums-pill ${isOnline ? 'ums-pill-active' : 'ums-pill-inactive'}`}>
          <span className={`ums-pulse-dot ${isOnline ? '' : 'ums-dot-danger'}`} />
          {isOnline ? 'آنلاین' : 'آفلاین'}
        </span>
      </div>

      {isLoading ? (
        <p className='ums-meter-note' style={{ marginTop: 16 }}>در حال بارگذاری...</p>
      ) : (
        <>
          {(details?.protocols ?? []).map((info: UserManagerShareProtocolInfo) => (
            <div key={info.protocol} className='ums-box' style={{ marginTop: 16 }}>
              <div className='ums-row'>
                <span className='ums-label'>پروتکل</span>
                <span className='ums-protocol-tag'>{info.protocol}</span>
              </div>
              <CopyableField label='آدرس سرور' value={info.server_address} />
              <CopyableField label='پورت' value={String(info.port)} />

              {(info.has_client_app_file || info.has_certificate_file) && (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginTop: 10 }}>
                  {info.has_client_app_file && (
                    <button
                      className='ums-btn'
                      disabled={isClientAppDownloading}
                      onClick={() => handleDownloadClientApp(info.protocol)}
                    >
                      <DownloadIcon className='h-3.5 w-3.5' />
                      {isClientAppDownloading ? 'در حال دانلود...' : 'اپلیکیشن'}
                    </button>
                  )}
                  {info.has_certificate_file && (
                    <button
                      className='ums-btn'
                      disabled={isCertificateDownloading}
                      onClick={() => handleDownloadCertificate(info.protocol)}
                    >
                      <DownloadIcon className='h-3.5 w-3.5' />
                      {isCertificateDownloading ? 'در حال دانلود...' : 'گواهی'}
                    </button>
                  )}
                </div>
              )}

              {info.notes && (
                <p className='ums-meter-note' style={{ marginTop: 10, borderTop: '1px solid rgba(255,255,255,0.08)', paddingTop: 8, whiteSpace: 'pre-wrap' }}>
                  {info.notes}
                </p>
              )}
            </div>
          ))}

          <div className='ums-divider'>
            <CopyableField label='یوزرنیم' value={details?.username ?? ''} />

            <div className='ums-row'>
              <span className='ums-label'>پسورد</span>
              <div className='ums-copy-row'>
                <span className='ums-value ums-value-mono'>
                  {isPasswordVisible ? details?.password : '••••••••'}
                </span>
                <button
                  className='ums-icon-btn'
                  onClick={() => setIsPasswordVisible((prev) => !prev)}
                >
                  {isPasswordVisible ? (
                    <EyeOffIcon className='h-3.5 w-3.5' />
                  ) : (
                    <EyeIcon className='h-3.5 w-3.5' />
                  )}
                </button>
                <button
                  className='ums-icon-btn'
                  onClick={async () => {
                    const succeeded = await copyToClipboard(details?.password ?? '')
                    toast[succeeded ? 'success' : 'error'](
                      succeeded ? 'پسورد کپی شد' : 'کپی ناموفق بود',
                      { duration: 5000 }
                    )
                  }}
                >
                  <ClipboardCopyIcon className='h-3.5 w-3.5' />
                </button>
              </div>
            </div>

            <div className='ums-row'>
              <span className='ums-label'>زمان انقضا</span>
              <span className='ums-value'>{details?.expire_time ?? 'هرگز'}</span>
            </div>
          </div>

          <div className='ums-box' style={{ marginTop: 12 }}>
            <div className='ums-label'>
              <GaugeIcon className='h-3.5 w-3.5' />
              مجموع مصرف
            </div>
            <div className='ums-meter-value'>
              {totalUsed.toFixed(2)} گیگابایت
            </div>
            <div className='ums-meter-note'>
              {hasLimit
                ? `از ${details?.traffic_limit} گیگابایت (${usagePercent}%)`
                : 'نامحدود'}
            </div>
            {hasLimit && (
              <div className='ums-progress-track'>
                <div className='ums-progress-fill' style={{ width: `${usagePercent}%` }} />
              </div>
            )}
          </div>

          {details?.has_config_file && (
            <button
              className='ums-btn ums-btn-primary'
              style={{ marginTop: 14 }}
              disabled={isDownloading}
              onClick={handleDownloadConfig}
            >
              <DownloadIcon className='h-4 w-4' />
              {isDownloading ? 'در حال دانلود...' : 'دانلود فایل تنظیمات'}
            </button>
          )}
        </>
      )}
    </div>
  )
}
