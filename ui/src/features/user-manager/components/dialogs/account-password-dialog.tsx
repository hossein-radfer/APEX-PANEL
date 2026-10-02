import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import {
  ChangeUserManagerAccountPasswordSchema,
  UserManagerAccount,
} from '@/schema/user-manager.ts'
import { KeyRoundIcon } from 'lucide-react'
import { toast } from 'sonner'
import { useChangeUserManagerAccountPasswordMutation } from '@/hooks/user-manager/useChangeUserManagerAccountPasswordMutation.ts'
import { useChangeUserManagerAccountPasswordForResellerMutation } from '@/hooks/user-manager/useChangeUserManagerAccountPasswordForResellerMutation.ts'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { PasswordInput } from '@/components/password-input'

type Props = {
  open: boolean
  onOpenChange: (state: boolean) => void
  currentRow: UserManagerAccount
  targetResellerId?: number
}

// A dedicated dialog (mirroring AccountShareDialog's separate-action
// pattern) rather than a field inside the main edit form -- RouterOS User
// Manager account passwords are set via their own PATCH, not bundled into
// the account's other fields, and this action deserves its own explicit
// confirmation step rather than being one more silently-optional field in
// a big form.
export function AccountPasswordDialog({
  open,
  onOpenChange,
  currentRow,
  targetResellerId,
}: Props) {
  const [isSaving, setIsSaving] = useState(false)
  const changePassword = useChangeUserManagerAccountPasswordMutation()
  const changePasswordForReseller =
    useChangeUserManagerAccountPasswordForResellerMutation()

  const form = useForm<z.infer<typeof ChangeUserManagerAccountPasswordSchema>>({
    resolver: zodResolver(ChangeUserManagerAccountPasswordSchema),
    defaultValues: { password: '' },
  })

  const handleOpenChange = (state: boolean) => {
    if (!state) form.reset()
    onOpenChange(state)
  }

  const onSubmit = async (
    data: z.infer<typeof ChangeUserManagerAccountPasswordSchema>
  ) => {
    setIsSaving(true)
    try {
      if (targetResellerId !== undefined) {
        await changePasswordForReseller.mutateAsync({
          resellerId: targetResellerId,
          accountId: currentRow.id,
          password: data.password,
        })
      } else {
        await changePassword.mutateAsync({
          id: currentRow.id,
          password: data.password,
        })
      }
      toast.success('رمز عبور با موفقیت تغییر کرد', { duration: 5000 })
      handleOpenChange(false)
    } catch {
      toast.error('تغییر رمز عبور ناموفق بود. دوباره تلاش کنید.', {
        duration: 5000,
      })
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <KeyRoundIcon className='h-4 w-4' />
            تغییر رمز عبور
          </DialogTitle>
          <DialogDescription>
            یک رمز عبور جدید برای <span className='font-medium'>{currentRow.username}</span> تعیین کنید.
            این کار هم RouterOS و هم رکوردهای خود پنل را بلافاصله به‌روزرسانی می‌کند.
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
            <FormField
              control={form.control}
              name='password'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>رمز عبور جدید</FormLabel>
                  <FormControl>
                    <PasswordInput placeholder='رمز عبور جدید' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <DialogFooter>
              <Button type='submit' disabled={isSaving}>
                {isSaving ? 'در حال ذخیره...' : 'تغییر رمز عبور'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
