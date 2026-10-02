'use client'

import '@fontsource/vazirmatn/400.css'
import '@fontsource/vazirmatn/500.css'
import '@fontsource/vazirmatn/600.css'
import '@fontsource/vazirmatn/700.css'
import './share.css'

import { AxiosError } from 'axios'
import { useSearch } from '@tanstack/react-router'
import { IconRoute } from '@tabler/icons-react'
import { useUserConfigQuery } from '@/hooks/user/useUserConfigQuery.ts'
import { useUserDetailsQuery } from '@/hooks/user/useUserDetailsQuery.ts'
import { useUserQRCodeQuery } from '@/hooks/user/useUserQRCodeQuery.ts'
import NotFoundError from '@/features/errors/not-found-error.tsx'
import PeerConfigCard from '@/features/share/components/peer-config-card.tsx'
import PeerQRCodeCard from '@/features/share/components/peer-qrcode-card.tsx'
import PeerStatsCard from '@/features/share/components/peer-stats-card.tsx'
import { wireguardShareFaq } from '@/features/share/lib/faq-content.ts'
import { SmartFaq } from '@/features/shared-components/smart-faq.tsx'

// Redesigned to match v2ray-sub's and dns-share's own visual language
// (fixed dark theme, gradient cards) and translated to Persian, per the
// explicit "redesign to match the other protocols, more appealing" ask --
// this was previously the only share page still in English with plain
// shadcn Cards on the panel's own light/dark toggle.
export default function PeerShare() {
  const { shareId } = useSearch({ from: '/share' })

  const {
    data: stats,
    error: statsError,
    isLoading: statsLoading,
  } = useUserDetailsQuery(shareId)

  const { data: configBlob, isLoading: configLoading } =
    useUserConfigQuery(shareId)

  const { data: qrCode, isLoading: qrCodeLoading } = useUserQRCodeQuery(shareId)

  if (statsError && (statsError as AxiosError)?.response?.status === 404) {
    return <NotFoundError />
  }

  return (
    <div className='wgs-root'>
      <div className='wgs-container'>
        <div className='wgs-header'>
          <div className='wgs-header-brand'>
            <IconRoute className='h-6 w-6 shrink-0' />
            <div className='wgs-header-title' style={{ fontSize: 20 }}>
              ApexPanel
            </div>
            <IconRoute className='h-6 w-6 shrink-0' />
          </div>
          <div className='wgs-header-sub' style={{ fontSize: 15, marginTop: 10 }}>
            {stats?.name ? `خوش آمدید: ${stats.name}` : 'خوش آمدید به پنل شما'}
          </div>
          <div className='wgs-header-sub'>
            کد QR را با اپلیکیشن WireGuard اسکن کنید یا فایل تنظیمات را دانلود
            و به‌صورت دستی وارد کنید.
          </div>
        </div>

        <div className='wgs-grid'>
          <PeerQRCodeCard isLoading={qrCodeLoading} qrCode={qrCode} />

          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <PeerConfigCard
              isLoading={configLoading}
              blob={
                configBlob
                  ? new Blob([configBlob], { type: 'text/plain' })
                  : undefined
              }
              peerName={stats?.name}
            />
            <PeerStatsCard isLoading={statsLoading} stats={stats} />
          </div>
        </div>

        <div className='dark'>
          <SmartFaq items={wireguardShareFaq} />
        </div>
      </div>
    </div>
  )
}
