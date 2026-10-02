import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useRouter } from '@tanstack/react-router'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { StartTrialSchema } from '@/schema/license.ts'
import { useStartTrialMutation } from '@/hooks/license/useStartTrialMutation.ts'
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

function backendErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof AxiosError) {
    const message = err.response?.data?.message
    if (typeof message === 'string' && message.length > 0) return message
  }
  return fallback
}

export function FreeTrialForm() {
  const router = useRouter()
  const { mutateAsync: startTrial, isPending } = useStartTrialMutation()

  const form = useForm<z.infer<typeof StartTrialSchema>>({
    resolver: zodResolver(StartTrialSchema),
    defaultValues: {
      username: '',
      password: '',
      email: '',
    },
  })

  async function onSubmit(data: z.infer<typeof StartTrialSchema>) {
    try {
      const status = await startTrial(data)
      if (!status.valid) {
        toast.error(status.reason || 'شروع دوره‌ی آزمایشی رایگان ممکن نشد.')
        return
      }
      toast.success('دوره‌ی آزمایشی رایگان ۳۰ روزه با موفقیت شروع شد. لطفاً وارد شوید.')
      form.reset()
      router.navigate({ to: '/sign-in' })
    } catch (err) {
      toast.error(
        backendErrorMessage(
          err,
          'شروع دوره‌ی آزمایشی رایگان ممکن نشد. لطفاً اطلاعات ورود را بررسی و دوباره تلاش کنید.'
        )
      )
    }
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className='grid gap-3'>
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
          name='email'
          render={({ field }) => (
            <FormItem>
              <FormLabel>ایمیل</FormLabel>
              <FormControl>
                <Input type='email' placeholder='you@example.com' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <p className='text-muted-foreground text-xs'>
          پس از ثبت، بلافاصله به مدت ۳۰ روز به‌صورت رایگان فعال می‌شوید --
          بدون نیاز به کارت بانکی یا پرداخت.
        </p>
        <Button className='mt-1' disabled={isPending}>
          {isPending ? 'در حال شروع دوره‌ی آزمایشی...' : 'شروع ۳۰ روز رایگان'}
        </Button>
      </form>
    </Form>
  )
}
