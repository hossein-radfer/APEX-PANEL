import '@fontsource/vazirmatn/400.css'
import '@fontsource/vazirmatn/500.css'
import '@fontsource/vazirmatn/600.css'
import '@fontsource/vazirmatn/700.css'
import './v2ray-sub.css'

import { useMemo, useState } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { AxiosError } from 'axios'
import { useV2RayPackageShareDetailsQuery } from '@/hooks/v2ray/useV2RayPackageShareDetailsQuery.ts'
import { V2SubAppsAccordion } from '@/features/v2ray-sub/components/v2sub-apps-accordion.tsx'
import { V2SubConfigsAccordion } from '@/features/v2ray-sub/components/v2sub-configs-accordion.tsx'
import { V2SubFaqCard } from '@/features/v2ray-sub/components/v2sub-faq-card.tsx'
import { V2SubLoginForm } from '@/features/v2ray-sub/components/v2sub-login-form.tsx'
import { V2SubProfileCard } from '@/features/v2ray-sub/components/v2sub-profile-card.tsx'
import { V2SubQrModal } from '@/features/v2ray-sub/components/v2sub-qr-modal.tsx'
import { V2SubToast } from '@/features/v2ray-sub/components/v2sub-toast.tsx'
import { detectOS } from '@/features/v2ray-sub/lib/detect-os.ts'

export default function V2RaySubscriptionPage() {
  const { shareId } = useSearch({ from: '/v2ray-sub' })
  const navigate = useNavigate({ from: '/v2ray-sub' })

  const [qrValue, setQrValue] = useState<string | null>(null)
  const [toastMessage, setToastMessage] = useState<string | null>(null)
  const detectedOS = useMemo(() => detectOS(), [])

  const {
    data: details,
    error: detailsError,
    isLoading,
  } = useV2RayPackageShareDetailsQuery(shareId)

  const handleManualSubmit = (id: string) => {
    navigate({ search: { shareId: id } })
  }

  // Login state -- no shareId in the URL yet.
  if (!shareId) {
    return <V2SubLoginForm onSubmit={handleManualSubmit} />
  }

  const notFound =
    detailsError && (detailsError as AxiosError)?.response?.status === 404

  // Loading state.
  if (isLoading || (!details && !notFound)) {
    return (
      <div className='v2sub-root'>
        <div className='v2sub-center-screen'>
          <div className='v2sub-spinner' />
          <p className='v2sub-header-sub'>در حال دریافت اطلاعات...</p>
        </div>
      </div>
    )
  }

  // Error / not found -- offer the manual entry form again rather than a
  // dead end, matching the "login state shown when there's no valid
  // token" requirement.
  if (notFound || !details) {
    return <V2SubLoginForm onSubmit={handleManualSubmit} />
  }

  return (
    <div className='v2sub-root'>
      <div className='v2sub-container'>
        <div className='v2sub-header'>
          <div className='v2sub-header-title'>اشتراک V2Ray</div>
          <div className='v2sub-header-sub'>
            برای اتصال، یکی از اپلیکیشن‌های زیر را نصب کرده و لینک اشتراک را وارد کنید.
          </div>
        </div>

        <V2SubProfileCard details={details} />

        <V2SubConfigsAccordion
          details={details}
          onShowQr={setQrValue}
          onCopied={setToastMessage}
        />

        <V2SubAppsAccordion
          subscriptionUrl={details.subscription_url}
          detectedOS={detectedOS}
        />

        <V2SubFaqCard />
      </div>

      <V2SubQrModal value={qrValue} onClose={() => setQrValue(null)} />
      <V2SubToast message={toastMessage} onDismiss={() => setToastMessage(null)} />
    </div>
  )
}
