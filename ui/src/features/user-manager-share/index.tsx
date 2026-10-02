'use client'

import '@fontsource/vazirmatn/400.css'
import '@fontsource/vazirmatn/500.css'
import '@fontsource/vazirmatn/600.css'
import '@fontsource/vazirmatn/700.css'
import './user-manager-share.css'

import { AxiosError } from 'axios'
import { useSearch } from '@tanstack/react-router'
import { IconRoute } from '@tabler/icons-react'
import { useUserManagerAccountShareDetailsQuery } from '@/hooks/user-manager/useUserManagerAccountShareDetailsQuery.ts'
import NotFoundError from '@/features/errors/not-found-error.tsx'
import { userManagerShareFaq } from '@/features/share/lib/faq-content.ts'
import { SmartFaq } from '@/features/shared-components/smart-faq.tsx'
import ConnectionInfoCard from '@/features/user-manager-share/components/connection-info-card.tsx'

// Redesigned to match v2ray-sub's/dns-share's/share's own visual language
// (fixed dark theme, gradient cards) and translated to Persian, per the
// explicit "redesign every remaining page to match the other protocols"
// ask -- this was the last share page still in English with plain shadcn
// Cards on the panel's own light/dark toggle.
export default function UserManagerAccountShare() {
  const { shareId } = useSearch({ from: '/user-manager-share' })

  const {
    data: details,
    error: detailsError,
    isLoading: detailsLoading,
  } = useUserManagerAccountShareDetailsQuery(shareId)

  if (detailsError && (detailsError as AxiosError)?.response?.status === 404) {
    return <NotFoundError />
  }

  if (detailsLoading || !details) {
    return (
      <div className='ums-root'>
        <div className='ums-center-screen'>
          <div className='ums-spinner' />
          <p className='ums-header-sub'>در حال دریافت اطلاعات...</p>
        </div>
      </div>
    )
  }

  return (
    <div className='ums-root'>
      <div className='ums-container'>
        <div className='ums-header'>
          <div className='ums-header-brand'>
            <IconRoute className='h-6 w-6 shrink-0' />
            <div className='ums-header-title' style={{ fontSize: 20 }}>
              ApexPanel
            </div>
            <IconRoute className='h-6 w-6 shrink-0' />
          </div>
          <div className='ums-header-sub' style={{ fontSize: 15, marginTop: 10 }}>
            {details.username ? `حساب شما: ${details.username}` : 'حساب VPN شما'}
          </div>
          <div className='ums-header-sub'>
            از اطلاعات زیر برای برقراری اتصال{' '}
            {details.protocols?.map((p) => p.protocol.toUpperCase()).join(' / ')}{' '}
            استفاده کنید.
          </div>
        </div>

        <ConnectionInfoCard
          isLoading={detailsLoading}
          details={details}
          uuid={shareId}
        />

        <div className='dark'>
          <SmartFaq items={userManagerShareFaq} />
        </div>
      </div>
    </div>
  )
}
