import { IconDeviceMobile, IconRefresh } from '@tabler/icons-react'
import { Loader2Icon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { useApplication } from '@/features/applications/context/application-context.tsx'

interface Props {
  refetchApplicationsList: () => void
  isApplicationsListRefetching: boolean
  canCreate: boolean
}

export function ApplicationPrimaryButtons({
  refetchApplicationsList,
  isApplicationsListRefetching,
  canCreate,
}: Props) {
  const { setOpen } = useApplication()

  return (
    <div className='flex flex-wrap items-center gap-3'>
      <Button
        variant='outline'
        className={cn(
          'shadow-none',
          isApplicationsListRefetching && 'cursor-not-allowed opacity-70'
        )}
        disabled={isApplicationsListRefetching}
        onClick={refetchApplicationsList}
      >
        {isApplicationsListRefetching ? (
          <Loader2Icon className='h-4 w-4 animate-spin' />
        ) : (
          <IconRefresh className='h-4 w-4' />
        )}
        <span className='text-sm font-medium'>بروزرسانی</span>
      </Button>

      <Button
        onClick={() => setOpen('add')}
        className='gap-2 transition-all'
        disabled={!canCreate}
        title={canCreate ? undefined : 'شما اجازه‌ی ساخت اپلیکیشن ندارید'}
      >
        <span className='text-sm font-medium'>ساخت اپلیکیشن</span>
        <IconDeviceMobile className='h-4 w-4' />
      </Button>
    </div>
  )
}
