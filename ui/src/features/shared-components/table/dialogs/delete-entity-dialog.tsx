import { useState } from 'react'
import { IconAlertTriangle } from '@tabler/icons-react'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert.tsx'
import { Input } from '@/components/ui/input.tsx'
import { Label } from '@/components/ui/label.tsx'
import { ConfirmDialog } from '@/components/confirm-dialog.tsx'

interface Props<T> {
  open: boolean
  onOpenChange: (open: boolean) => void
  entity: T & { id: string; name: string }
  entityType: string
  mutationFn: (id: string) => Promise<void>
}

export function DeleteEntityDialog<T>({
  open,
  onOpenChange,
  entity,
  entityType,
  mutationFn,
}: Props<T>) {
  const [loading, setLoading] = useState(false)
  const [value, setValue] = useState('')

  const handleDelete = async () => {
    if (value.trim() !== entity.name) return
    try {
      setLoading(true)
      await mutationFn(entity.id)
      onOpenChange(false)
      toast.success(`${entityType} «${entity.name}» با موفقیت حذف شد`, {
        duration: 5000,
      })
    } finally {
      setLoading(false)
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      handleConfirm={handleDelete}
      disabled={value.trim() !== entity.name}
      title={
        <span className='text-destructive'>
          <IconAlertTriangle
            className='stroke-destructive mr-1 inline-block'
            size={18}
          />
          حذف {entityType}
        </span>
      }
      desc={
        <div className='space-y-4'>
          <p className='mb-2'>
            آیا از حذف <span className='font-bold'>{entity.name}</span>{' '}
            مطمئن هستید؟
            <br />
            این عملیات {entityType} با نام{' '}
            <span className='font-bold'>{entity.name}</span> را برای همیشه از
            سیستم حذف می‌کند. این عملیات قابل بازگشت نیست.
          </p>

          <Label className='my-2'>
            نام:
            <Input
              value={value}
              onChange={(e) => setValue(e.target.value)}
              placeholder={`برای تأیید حذف، نام ${entityType} را وارد کنید.`}
            />
          </Label>

          <Alert variant='destructive'>
            <AlertTitle>هشدار!</AlertTitle>
            <AlertDescription>
              لطفاً دقت کنید، این عملیات قابل بازگشت نیست.
            </AlertDescription>
          </Alert>
        </div>
      }
      isLoading={loading}
      confirmText='حذف'
      destructive
    />
  )
}
