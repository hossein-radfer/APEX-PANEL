import { useState } from 'react'
import { IconArrowsExchange } from '@tabler/icons-react'
import { RotateCcw } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { HighlightStatsCard } from '@/features/dashboard/components/highlight-stats-card.tsx'
import { useResetTotalTrafficMutation } from '@/hooks/dashboard/useResetTotalTrafficMutation.ts'

type TrafficStatsCardProps = {
  value: string | undefined
  wireguardUsage?: string
  userManagerUsage?: string
  isLoading: boolean
}

export function TrafficStatsCard({
  value,
  wireguardUsage,
  userManagerUsage,
  isLoading,
}: TrafficStatsCardProps) {
  const [open, setOpen] = useState(false)
  const { mutateAsync: resetTotalTraffic, isPending } =
    useResetTotalTrafficMutation()

  const handleReset = async () => {
    await resetTotalTraffic()
    toast.success('مصرف کل ترافیک با موفقیت بازنشانی شد', {
      duration: 5000,
    })
    setOpen(false)
  }

  return (
    <>
      <HighlightStatsCard
        title='کل ترافیک'
        icon={<IconArrowsExchange />}
        value={value}
        suffix='گیگابایت'
        isLoading={isLoading}
        subtitle={
          wireguardUsage !== undefined && userManagerUsage !== undefined
            ? `وایرگارد ${wireguardUsage} گیگابایت · User Manager ${userManagerUsage} گیگابایت`
            : undefined
        }
        action={
          <Button
            variant='outline'
            size='sm'
            className='shrink-0 gap-2'
            aria-label='بازنشانی مصرف کل ترافیک'
            disabled={isLoading || isPending}
            onClick={() => setOpen(true)}
          >
            <RotateCcw className='size-4' />
            <span className='hidden sm:inline'>بازنشانی</span>
          </Button>
        }
      />

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader className='items-start'>
            <DialogTitle>مصرف کل ترافیک بازنشانی شود؟</DialogTitle>
            <DialogDescription className='text-right'>
              این کار شمارنده‌ی تجمیعی ترافیک را صفر می‌کند. آمار مصرف
              کاربران تحت تأثیر قرار نمی‌گیرد.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant='outline' onClick={() => setOpen(false)}>
              انصراف
            </Button>
            <Button
              onClick={handleReset}
              disabled={isPending}
              className='bg-destructive dark:bg-destructive/60 hover:bg-destructive focus-visible:ring-destructive text-white'
            >
              بازنشانی
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
