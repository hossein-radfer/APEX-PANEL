'use client'

import { useState } from 'react'

interface QRCodeCardProps {
  isLoading: boolean
  qrCode?: string
}

// Redesigned to match v2ray-sub/dns-share's card language, per the
// explicit "redesign to match the other protocols" ask.
export default function PeerQRCodeCard({ isLoading, qrCode }: QRCodeCardProps) {
  const [isBlurred, setIsBlurred] = useState(true)

  return (
    <div className='wgs-card' style={{ display: 'flex', flexDirection: 'column' }}>
      <div className='wgs-header-title' style={{ marginBottom: 16 }}>کد QR</div>

      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        {isLoading ? (
          <div className='wgs-spinner-box' style={{ width: 260, height: 260 }} />
        ) : (
          <div
            className='wgs-qr-wrap'
            onClick={() => setIsBlurred((prev) => !prev)}
            title={isBlurred ? 'برای نمایش کلیک کنید' : 'برای پنهان‌کردن کلیک کنید'}
          >
            <img
              src={qrCode}
              alt='QR Code'
              className={`wgs-qr-img ${isBlurred ? 'wgs-blurred' : ''}`}
            />
            {isBlurred && (
              <div className='wgs-reveal-overlay'>برای نمایش کلیک کنید</div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
