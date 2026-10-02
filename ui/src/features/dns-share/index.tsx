'use client'

import '@fontsource/vazirmatn/400.css'
import '@fontsource/vazirmatn/500.css'
import '@fontsource/vazirmatn/600.css'
import '@fontsource/vazirmatn/700.css'
import './dns-share.css'

import { AxiosError } from 'axios'
import { useSearch } from '@tanstack/react-router'
import { IconRoute } from '@tabler/icons-react'
import { useDNSAccountShareDetailsQuery } from '@/hooks/dns-account/useDNSAccountShareDetailsQuery.ts'
import NotFoundError from '@/features/errors/not-found-error.tsx'
import { dnsShareFaq } from '@/features/share/lib/faq-content.ts'
import { SmartFaq } from '@/features/shared-components/smart-faq.tsx'
import DNSRegisterIPCard from '@/features/dns-share/components/dns-register-ip-card.tsx'
import DNSStatusCard from '@/features/dns-share/components/dns-status-card.tsx'

// Redesigned to match v2ray-sub's own visual language (fixed dark theme,
// gradient cards, animated meters) per the explicit "redesign DNS to look
// like the other protocols, more appealing" ask -- this was previously the
// only share page still using plain shadcn Cards on the panel's own
// light/dark toggle instead of a dedicated, always-dark customer-facing
// design.
export default function DNSAccountShare() {
  const { shareId } = useSearch({ from: '/dns-share' })

  const {
    data: details,
    error: detailsError,
    isLoading: detailsLoading,
  } = useDNSAccountShareDetailsQuery(shareId)

  if (detailsError && (detailsError as AxiosError)?.response?.status === 404) {
    return <NotFoundError />
  }

  if (detailsLoading || !details) {
    return (
      <div className='dnss-root'>
        <div className='dnss-center-screen'>
          <div className='dnss-spinner' />
          <p className='dnss-header-sub'>در حال دریافت اطلاعات...</p>
        </div>
      </div>
    )
  }

  return (
    <div className='dnss-root'>
      <div className='dnss-container'>
        <div className='dnss-header'>
          <div className='dnss-header-brand'>
            <IconRoute className='h-6 w-6 shrink-0' />
            <div className='dnss-header-title' style={{ fontSize: 20 }}>
              ApexPanel
            </div>
            <IconRoute className='h-6 w-6 shrink-0' />
          </div>
          <div className='dnss-header-sub' style={{ fontSize: 15, marginTop: 10 }}>
            {details.customer_label
              ? `سلام 👋 ${details.customer_label}`
              : 'حساب Smart DNS شما'}
          </div>
          <div className='dnss-header-sub'>
            برای استفاده از این سرویس، آی‌پی دستگاه فعلی خود را با دکمه‌ی
            «ثبت IP من» ثبت کنید.
          </div>
        </div>

        <DNSStatusCard details={details} />
        <DNSRegisterIPCard uuid={shareId} />

        <div className='dark'>
          <SmartFaq items={dnsShareFaq} />
        </div>
      </div>
    </div>
  )
}
