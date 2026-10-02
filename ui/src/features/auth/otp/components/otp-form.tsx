import { HTMLAttributes, useEffect, useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useNavigate } from '@tanstack/react-router'
import { AxiosError } from 'axios'
import { ShieldCheckIcon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/authStore.ts'
import { useVerifyOtpMutation } from '@/hooks/authentication/useVerifyOtpMutation.tsx'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSlot,
} from '@/components/ui/input-otp'

type OtpFormProps = HTMLAttributes<HTMLFormElement> & {
  otpToken: string
}

const formSchema = z.object({
  otp: z.string().length(4, { message: 'کد باید ۴ رقمی باشد.' }),
})

const OTP_VALIDITY_SECONDS = 99

export function OtpForm({ className, otpToken, ...props }: OtpFormProps) {
  const navigate = useNavigate()
  const authStore = useAuthStore()
  const { mutateAsync: verifyOtp, isPending } = useVerifyOtpMutation()

  const [secondsLeft, setSecondsLeft] = useState(OTP_VALIDITY_SECONDS)
  const [bannedUntil, setBannedUntil] = useState<number | null>(null)
  const [banSecondsLeft, setBanSecondsLeft] = useState(0)

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: { otp: '' },
  })

  const otp = form.watch('otp')

  // Live countdown toward the code's own 99-second server-side expiry --
  // purely informational (the server is the real source of truth on
  // expiry), but showing it is what makes "your code just expired" a
  // legible outcome instead of a confusing generic error after the fact.
  useEffect(() => {
    if (secondsLeft <= 0) return
    const timer = setInterval(() => {
      setSecondsLeft((s) => Math.max(0, s - 1))
    }, 1000)
    return () => clearInterval(timer)
  }, [secondsLeft])

  // Live countdown for an active IP ban -- same purpose as above, purely
  // informational; the server enforces the actual lockout window.
  useEffect(() => {
    if (!bannedUntil) return
    const tick = () => {
      const remaining = Math.max(0, Math.ceil((bannedUntil - Date.now()) / 1000))
      setBanSecondsLeft(remaining)
      if (remaining <= 0) setBannedUntil(null)
    }
    tick()
    const timer = setInterval(tick, 1000)
    return () => clearInterval(timer)
  }, [bannedUntil])

  const expired = secondsLeft <= 0
  const isBanned = bannedUntil !== null && banSecondsLeft > 0

  async function onSubmit(data: z.infer<typeof formSchema>) {
    try {
      const response = await verifyOtp({ otp_token: otpToken, code: data.otp })

      if (response.access_token && response.user_id !== undefined) {
        authStore.auth.setAccessToken(response.access_token)
        authStore.auth.setAdmin({
          user_id: response.user_id,
          username: response.username ?? '',
          role: response.role ?? 'admin',
          reseller_id: response.reseller_id ?? null,
        })
        navigate({ to: '/' })
      }
    } catch (err) {
      if (err instanceof AxiosError && err.response?.status === 429) {
        setBannedUntil(Date.now() + 3 * 60 * 1000)
        form.setError('otp', {
          message: 'تعداد تلاش‌های ناموفق بیش از حد مجاز بود. ۳ دقیقه صبر کنید.',
        })
        return
      }
      form.setError('otp', { message: 'کد وارد شده نادرست یا منقضی‌شده است.' })
      form.resetField('otp')
    }
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-3', className)}
        {...props}
      >
        <div className='flex flex-col items-center gap-2 py-2'>
          <div
            className={cn(
              'flex h-14 w-14 items-center justify-center rounded-full border-2',
              isBanned
                ? 'border-destructive/40 text-destructive'
                : expired
                  ? 'border-muted-foreground/30 text-muted-foreground'
                  : 'border-primary/40 text-primary animate-pulse'
            )}
          >
            <ShieldCheckIcon className='h-7 w-7' />
          </div>
          {isBanned ? (
            <p className='text-destructive text-sm font-medium'>
              دسترسی شما به‌طور موقت مسدود شده — {banSecondsLeft} ثانیه دیگر
              دوباره تلاش کنید.
            </p>
          ) : expired ? (
            <p className='text-muted-foreground text-sm'>
              زمان اعتبار کد به پایان رسید. یک کد جدید درخواست کنید.
            </p>
          ) : (
            <p className='text-muted-foreground text-sm'>
              اعتبار کد: <span className='font-mono'>{secondsLeft}</span> ثانیه
            </p>
          )}
        </div>

        <FormField
          control={form.control}
          name='otp'
          render={({ field }) => (
            <FormItem>
              <FormLabel className='sr-only'>کد تایید</FormLabel>
              <FormControl>
                <InputOTP
                  maxLength={4}
                  {...field}
                  disabled={isBanned || expired}
                  containerClassName='justify-center'
                >
                  <InputOTPGroup>
                    <InputOTPSlot index={0} className='h-12 w-12 text-lg' />
                    <InputOTPSlot index={1} className='h-12 w-12 text-lg' />
                    <InputOTPSlot index={2} className='h-12 w-12 text-lg' />
                    <InputOTPSlot index={3} className='h-12 w-12 text-lg' />
                  </InputOTPGroup>
                </InputOTP>
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <Button
          className='mt-2'
          disabled={otp.length < 4 || isPending || isBanned || expired}
        >
          {isPending ? 'در حال بررسی...' : 'تایید'}
        </Button>
      </form>
    </Form>
  )
}
