import { IconClockPause, IconLock } from '@tabler/icons-react'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import AuthLayout from '../auth-layout'
import { LicenseActivationForm } from './components/license-activation-form'
import { FreeTrialForm } from './components/free-trial-form'

interface Props {
  // The backend's PublicLicenseStatus.reason, when the lockdown guard has
  // one available -- e.g. license-panel's ErrTrialExpired message for a
  // lapsed trial. Distinguished from a generic lockout (never activated,
  // revoked license, etc.) so a customer whose trial just ended sees a
  // specific, actionable prompt instead of a one-size-fits-all message.
  reason?: string
}

// A trial-expiry reason is always exactly this string (see license-
// panel's ErrTrialExpired) -- checked by substring rather than an exact
// match so backend wording tweaks don't silently break this detection.
function isTrialExpiredReason(reason: string | undefined): boolean {
  return !!reason && reason.toLowerCase().includes('trial has ended')
}

export default function LicenseActivation({ reason }: Props) {
  const trialExpired = isTrialExpiredReason(reason)

  return (
    <AuthLayout>
      <Card className='gap-4 shadow-lg'>
        <CardHeader>
          <div className='bg-primary/10 text-primary mx-auto mb-2 flex size-12 items-center justify-center rounded-full'>
            {trialExpired ? (
              <IconClockPause className='size-6' />
            ) : (
              <IconLock className='size-6' />
            )}
          </div>
          <CardTitle className='text-center text-lg tracking-tight'>
            {trialExpired ? 'دوره آزمایشی رایگان شما به پایان رسیده است' : 'فعال‌سازی این نصب'}
          </CardTitle>
          <CardDescription className='text-center'>
            {trialExpired
              ? 'دوره آزمایشی رایگان شما به پایان رسیده است. برای ادامه استفاده از این پنل، لطفا یک لایسنس خریداری کنید.'
              : 'این پنل تا فعال‌سازی یک لایسنس معتبر قفل است. کلید لایسنس خود را در زیر وارد کنید، یا یک دوره آزمایشی رایگان ۳۰ روزه را برای بررسی پنل آغاز کنید.'}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue='activate' className='w-full'>
            <TabsList className='grid w-full grid-cols-2'>
              <TabsTrigger value='activate'>کلید لایسنس</TabsTrigger>
              <TabsTrigger value='trial'>۳۰ روز رایگان</TabsTrigger>
            </TabsList>
            <TabsContent value='activate' className='pt-2'>
              <LicenseActivationForm />
            </TabsContent>
            <TabsContent value='trial' className='pt-2'>
              <FreeTrialForm />
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>
    </AuthLayout>
  )
}
