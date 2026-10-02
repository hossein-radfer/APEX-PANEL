import { IconRefresh, IconServerBolt } from '@tabler/icons-react'
import { Loader2Icon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { useDNSPanels } from '@/features/dns-panels/context/dns-panels-context.tsx'

interface Props {
  refetchPanelsList: () => void
  isPanelsListRefetching: boolean
}

export function DNSPanelsPrimaryButtons({
  refetchPanelsList,
  isPanelsListRefetching,
}: Props) {
  const { setOpen } = useDNSPanels()

  return (
    <div className='flex flex-wrap items-center gap-3'>
      <Button
        variant='outline'
        className={cn(
          'shadow-none',
          isPanelsListRefetching && 'cursor-not-allowed opacity-70'
        )}
        disabled={isPanelsListRefetching}
        onClick={refetchPanelsList}
      >
        {isPanelsListRefetching ? (
          <Loader2Icon className='h-4 w-4 animate-spin' />
        ) : (
          <IconRefresh className='h-4 w-4' />
        )}
        <span className='text-sm font-medium'>بارگذاری مجدد</span>
      </Button>

      <Button onClick={() => setOpen('add')} className='gap-2 transition-all'>
        <span className='text-sm font-medium'>افزودن پنل</span>
        <IconServerBolt className='h-4 w-4' />
      </Button>
    </div>
  )
}
