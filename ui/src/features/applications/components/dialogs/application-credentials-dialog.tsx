import { Application } from '@/schema/application.ts'
import { CheckCircle2, ClipboardCopyIcon } from 'lucide-react'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/clipboard.ts'
import { Button } from '@/components/ui/button.tsx'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog.tsx'
import { Input } from '@/components/ui/input.tsx'
import { Label } from '@/components/ui/label.tsx'

type Props = {
  open: boolean
  onOpenChange: (state: boolean) => void
  application: Application | null
}

// ApplicationCredentialsDialog shows the just-created Application's
// app_username/app_password ONE TIME, right after creation -- a
// confirmed, reported bug this fixes: the create dialog previously
// closed itself immediately on success (ApplicationForm.onSubmit called
// onClose() right after the mutation resolved), so an admin who left
// the username/password fields blank (the documented "خالی = رندوم"
// placeholder) never actually saw the randomly-generated credentials
// anywhere -- there was no way to retrieve them afterward short of
// editing the Application again. This dialog is the ONLY place those
// two fields get surfaced post-creation; ApplicationDialogs is
// responsible for opening it with the mutation's own response instead
// of calling onClose directly.
function CopyField({ label, value }: { label: string; value: string }) {
  const handleCopy = async () => {
    const succeeded = await copyToClipboard(value)
    if (succeeded) {
      toast.success('کپی شد', { duration: 3000 })
    } else {
      toast.error('کپی ناموفق بود', { duration: 3000 })
    }
  }

  return (
    <div className='space-y-1.5'>
      <Label>{label}</Label>
      <div className='flex gap-2'>
        <Input value={value} readOnly className='font-mono' />
        <Button
          type='button'
          variant='outline'
          size='icon'
          onClick={handleCopy}
          aria-label={`کپی ${label}`}
        >
          <ClipboardCopyIcon className='h-4 w-4' />
        </Button>
      </div>
    </div>
  )
}

export function ApplicationCredentialsDialog({
  open,
  onOpenChange,
  application,
}: Props) {
  if (!application) return null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader className='text-left'>
          <DialogTitle className='flex items-center gap-2'>
            <CheckCircle2 className='h-5 w-5 text-green-500' />
            اپلیکیشن ساخته شد
          </DialogTitle>
          <DialogDescription>
            این اطلاعات ورود اپلیکیشن موبایل «{application.name}» است. همین
            الان آن را کپی/یادداشت کنید -- پسورد به‌صورت متن خام دوباره در
            هیچ‌جای دیگری نمایش داده نمی‌شود مگر با ویرایش این اپلیکیشن.
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4'>
          <CopyField label='یوزرنیم' value={application.app_username} />
          <CopyField label='پسورد' value={application.app_password} />
        </div>

        <DialogFooter>
          <Button onClick={() => onOpenChange(false)}>باشه، ذخیره کردم</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
