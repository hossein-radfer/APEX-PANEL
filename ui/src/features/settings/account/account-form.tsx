import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import {
  UpdateProfileRequest,
  updateProfileSchema,
} from '@/schema/authentication.ts'
import { useUpdateProfileMutation } from '@/hooks/authentication/useUpdateProfileMutation.tsx'
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
import { PasswordField } from '@/features/shared-components/password-field.tsx'

export function AccountForm() {
  const updateProfile = useUpdateProfileMutation()

  const form = useForm<UpdateProfileRequest>({
    resolver: zodResolver(updateProfileSchema),
  })

  function onSubmit(data: UpdateProfileRequest) {
    updateProfile.mutate(data)
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
        <PasswordField
          name='old_password'
          label='رمز عبور فعلی'
          control={form.control}
          placeholder='رمز عبور شما (الزامی)'
        />
        <FormField
          control={form.control}
          name='new_username'
          render={({ field }) => (
            <FormItem>
              <FormLabel>نام کاربری جدید</FormLabel>
              <FormControl>
                <Input placeholder='نام کاربری جدید شما (اختیاری)' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <PasswordField
          name='new_password'
          label='رمز عبور جدید'
          control={form.control}
          placeholder='رمز عبور جدید شما (اختیاری)'
        />
        <Button type='submit'>به‌روزرسانی حساب</Button>
      </form>
    </Form>
  )
}
