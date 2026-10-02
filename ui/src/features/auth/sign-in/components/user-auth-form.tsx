import { HTMLAttributes } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useRouter } from '@tanstack/react-router'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { loginRequestSchema } from '@/schema/authentication.ts'
import { useAuthStore } from '@/stores/authStore.ts'
import { cn } from '@/lib/utils'
import { useLoginMutation } from '@/hooks/authentication/useLoginMutation.tsx'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox.tsx'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label.tsx'
import { PasswordInput } from '@/components/password-input'

type UserAuthFormProps = HTMLAttributes<HTMLFormElement>

export function UserAuthForm({ className, ...props }: UserAuthFormProps) {
  const authStore = useAuthStore()
  const router = useRouter()

  const { mutateAsync: loginMutation, isPending: isLoginPending } =
    useLoginMutation()

  const form = useForm<z.infer<typeof loginRequestSchema>>({
    resolver: zodResolver(loginRequestSchema),
    defaultValues: {
      username: '',
      password: '',
    },
  })

  async function onSubmit(data: z.infer<typeof loginRequestSchema>) {
    try {
      const response = await loginMutation(data)

      if (response?.otp_required && response.otp_token) {
        router.navigate({
          to: '/otp',
          search: { otp_token: response.otp_token },
        })
        return
      }

      if (response && response.access_token && response.user_id !== undefined) {
        const admin = {
          user_id: response.user_id,
          username: response.username ?? data.username,
          role: response.role ?? 'admin',
          reseller_id: response.reseller_id ?? null,
        }

        authStore.auth.setAccessToken(response.access_token)
        authStore.auth.setAdmin(admin)

        form.reset()

        router.navigate({ to: '/' })
      }
    } catch (err) {
      // A 403 with error_code "license_invalid" means LicenseMiddleware
      // blocked this request before it ever reached the login handler --
      // this specific install has no valid license yet, so there is no
      // "wrong password" to report; send the admin to activate it instead
      // of showing a misleading auth error.
      if (
        err instanceof AxiosError &&
        err.response?.status === 403 &&
        err.response.data?.error_code === 'license_invalid'
      ) {
        toast.error('این نصب دارای مجوز نیست. در حال انتقال به صفحه فعال‌سازی...')
        router.navigate({ to: '/license-activation' })
        return
      }
      throw err
    }
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-3', className)}
        {...props}
      >
        <FormField
          control={form.control}
          name='username'
          render={({ field }) => (
            <FormItem>
              <FormLabel>نام کاربری</FormLabel>
              <FormControl>
                <Input placeholder='نام کاربری' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='password'
          render={({ field }) => (
            <FormItem className='relative'>
              <FormLabel>رمز عبور</FormLabel>
              <FormControl>
                <PasswordInput placeholder='********' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <div className='flex items-center gap-3'>
          <Checkbox id='remember' />
          <Label htmlFor='remember'>مرا به خاطر بسپار</Label>
        </div>
        <Button className='mt-2' disabled={isLoginPending}>
          {isLoginPending ? 'در حال ورود...' : 'ورود'}
        </Button>
      </form>
    </Form>
  )
}
