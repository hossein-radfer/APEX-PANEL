'use client'

import { WifiIcon } from 'lucide-react'
import { toast } from 'sonner'
import { useRegisterDNSIPMutation } from '@/hooks/dns-account/useRegisterDNSIPMutation.ts'

interface Props {
  uuid: string
}

// The DNS share page's own action, replacing V2Ray's subscription link/QR
// code -- DNS has no VPN client subscription format. Takes no parameters:
// the backend captures the caller's own real IP server-side for security,
// so there is nothing for this button to collect from the visitor.
// Redesigned to match v2ray-sub's card language, per the "redesign to be
// more appealing, match the other protocols" ask.
//
// ok:false is a NORMAL, expected outcome (daily cap reached, IP's country
// not allowed, or the IP already belongs to another account) -- it comes
// back as HTTP 200 with ok:false, not a thrown error, so it's shown as an
// inline info/warning toast rather than an error toast. The backend's own
// `message` is already localized (Persian) and displayed verbatim, never
// re-worded here.
export default function DNSRegisterIPCard({ uuid }: Props) {
  const { mutateAsync: registerIP, isPending } = useRegisterDNSIPMutation()

  const handleRegister = async () => {
    try {
      const result = await registerIP(uuid)
      if (result.ok) {
        toast.success(result.message, { duration: 6000 })
      } else {
        toast.warning(result.message, { duration: 6000 })
      }
    } catch {
      toast.error('ثبت IP ناموفق بود. لطفاً کمی بعد دوباره تلاش کنید.', {
        duration: 5000,
      })
    }
  }

  return (
    <div className='dnss-card'>
      <div className='dnss-header-title' style={{ marginBottom: 12 }}>ثبت IP</div>
      <p className='dnss-meter-note' style={{ marginBottom: 16 }}>
        برای استفاده از این سرویس روی دستگاه فعلی خود، روی دکمه‌ی زیر بزنید تا
        آی‌پی اینترنت فعلی شما به‌عنوان آی‌پی مجاز این حساب ثبت شود. نیازی به
        وارد کردن چیزی نیست.
      </p>
      <button
        className='dnss-btn-primary'
        onClick={handleRegister}
        disabled={isPending}
      >
        <WifiIcon className='h-4 w-4' />
        {isPending ? 'در حال ثبت...' : 'ثبت IP من'}
      </button>
      <p className='dnss-meter-note' style={{ marginTop: 12 }}>
        در صورت تغییر آی‌پی اینترنت شما (مثلاً با تعویض وای‌فای یا دیتا)، باید
        دوباره به این صفحه برگردید و این دکمه را بزنید.
      </p>
    </div>
  )
}
