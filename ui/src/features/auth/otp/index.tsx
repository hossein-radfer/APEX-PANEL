import { Link, getRouteApi } from '@tanstack/react-router'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import AuthLayout from '../auth-layout'
import { OtpForm } from './components/otp-form'

const routeApi = getRouteApi('/(auth)/otp')

export default function Otp() {
  const { otp_token: otpToken } = routeApi.useSearch()

  return (
    <AuthLayout>
      <Card className='gap-4'>
        <CardHeader>
          <CardTitle className='text-base tracking-tight'>
            تایید هویت دو مرحله‌ای
          </CardTitle>
          <CardDescription>
            کد ۴ رقمی ارسال‌شده از طریق ربات تلگرام را وارد کنید.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <OtpForm otpToken={otpToken} />
        </CardContent>
        <CardFooter>
          <p className='text-muted-foreground px-8 text-center text-sm'>
            کد را دریافت نکردید؟{' '}
            <Link
              to='/sign-in'
              className='hover:text-primary underline underline-offset-4'
            >
              دوباره وارد شوید تا کد جدید ارسال شود.
            </Link>
          </p>
        </CardFooter>
      </Card>
    </AuthLayout>
  )
}
