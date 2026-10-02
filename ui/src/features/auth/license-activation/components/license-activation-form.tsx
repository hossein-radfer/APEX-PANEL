import { HTMLAttributes } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useRouter } from '@tanstack/react-router'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { ActivateLicenseSchema } from '@/schema/license.ts'
import { cn } from '@/lib/utils'
import { useActivateLicenseMutation } from '@/hooks/license/useActivateLicenseMutation.ts'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { PasswordInput } from '@/components/password-input'

type LicenseActivationFormProps = HTMLAttributes<HTMLFormElement>

function backendErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof AxiosError) {
    const message = err.response?.data?.message
    if (typeof message === 'string' && message.length > 0) return message
  }
  return fallback
}

export function LicenseActivationForm({
  className,
  ...props
}: LicenseActivationFormProps) {
  const router = useRouter()
  const { mutateAsync: activate, isPending } = useActivateLicenseMutation()

  const form = useForm<z.infer<typeof ActivateLicenseSchema>>({
    resolver: zodResolver(ActivateLicenseSchema),
    defaultValues: {
      username: '',
      password: '',
      license_key: '',
    },
  })

  async function onSubmit(data: z.infer<typeof ActivateLicenseSchema>) {
    try {
      const status = await activate(data)
      if (!status.valid) {
        toast.error(status.reason || 'فعال‌سازی لایسنس رد شد.')
        return
      }
      toast.success('لایسنس با موفقیت فعال شد. لطفاً وارد شوید.')
      form.reset()
      router.navigate({ to: '/sign-in' })
    } catch (err) {
      toast.error(
        backendErrorMessage(err, 'فعال‌سازی لایسنس ناموفق بود. لطفاً اطلاعات ورود و کلید لایسنس را بررسی کنید.')
      )
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
              <FormLabel>نام کاربری مدیر</FormLabel>
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
              <FormLabel>رمز عبور مدیر</FormLabel>
              <FormControl>
                <PasswordInput placeholder='********' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='license_key'
          render={({ field }) => (
            <FormItem>
              <FormLabel>کلید لایسنس</FormLabel>
              <FormControl>
                <Input placeholder='کلید لایسنس' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <Button className='mt-2' disabled={isPending}>
          {isPending ? 'در حال فعال‌سازی...' : 'فعال‌سازی'}
        </Button>
      </form>
    </Form>
  )
}
