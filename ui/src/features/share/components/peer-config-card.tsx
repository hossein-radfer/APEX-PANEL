'use client'

import { useEffect, useState } from 'react'
import { CopyIcon, DownloadIcon } from 'lucide-react'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/clipboard.ts'

interface ConfigCardProps {
  isLoading: boolean
  blob?: Blob
  peerName?: string
}

// Redesigned to match v2ray-sub/dns-share's card language and translated
// to Persian (matching the rest of the panel), per the explicit "redesign
// to match the other protocols" ask.
export default function PeerConfigCard({
  isLoading,
  blob,
  peerName,
}: ConfigCardProps) {
  const [configText, setConfigText] = useState<string>('')
  const [isBlurred, setIsBlurred] = useState(true)

  useEffect(() => {
    if (blob) {
      const reader = new FileReader()
      reader.onload = () => setConfigText(reader.result as string)
      reader.readAsText(blob)
    }
  }, [blob])

  const handleCopy = async () => {
    if (!configText) return
    const succeeded = await copyToClipboard(configText)
    if (succeeded) {
      toast.success('در کلیپ‌بورد کپی شد.', { duration: 5000 })
    } else {
      toast.error('کپی ناموفق بود.', { duration: 5000 })
    }
  }

  const handleDownload = () => {
    if (!configText) return
    const file = new Blob([configText], {
      type: 'application/octet-stream',
    })
    const link = document.createElement('a')
    link.href = URL.createObjectURL(file)
    link.download = `${peerName || 'peer'}.conf`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  }

  return (
    <div className='wgs-card'>
      <div className='wgs-header-title' style={{ marginBottom: 16 }}>فایل تنظیمات</div>

      {isLoading ? (
        <p className='wgs-meter-note'>در حال بارگذاری...</p>
      ) : (
        <>
          <div
            className='wgs-config-box'
            onClick={() => setIsBlurred((prev) => !prev)}
            title={isBlurred ? 'برای نمایش کلیک کنید' : 'برای پنهان‌کردن کلیک کنید'}
          >
            <button
              className='wgs-icon-btn'
              onClick={(e) => {
                e.stopPropagation()
                handleCopy()
              }}
              title='کپی'
            >
              <CopyIcon className='h-4 w-4' />
            </button>
            <pre className={`wgs-config-pre ${isBlurred ? 'wgs-blurred' : ''}`}>
              <code>{configText}</code>
            </pre>
            {isBlurred && (
              <div className='wgs-config-overlay'>برای نمایش کلیک کنید</div>
            )}
          </div>

          <button className='wgs-btn-primary' onClick={handleDownload}>
            <DownloadIcon className='h-4 w-4' />
            دانلود فایل تنظیمات
          </button>
        </>
      )}
    </div>
  )
}
