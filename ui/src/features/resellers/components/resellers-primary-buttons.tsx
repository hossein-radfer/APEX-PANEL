import { IconUserPlus, IconRefresh } from '@tabler/icons-react'
import { Loader2Icon } from 'lucide-react'
import { Button } from '@/components/ui/button.tsx'
import { useResellers } from '@/features/resellers/context/resellers-context.tsx'

interface Props {
  refetchResellersList: () => void
  isResellersListRefetching: boolean
}

export function ResellersPrimaryButtons({
  refetchResellersList,
  isResellersListRefetching,
}: Props) {
  const { setOpen } = useResellers()

  return (
    <div className='flex flex-wrap items-center gap-3'>
      <div className='inline-flex w-fit -space-x-px rounded-md shadow-xs rtl:space-x-reverse'>
        <Button
          variant='outline'
          className='rounded-none rounded-s-md shadow-none transition-all focus-visible:z-10'
          onClick={refetchResellersList}
          disabled={isResellersListRefetching}
        >
          {isResellersListRefetching ? (
            <Loader2Icon className='h-4 w-4 animate-spin' />
          ) : (
            <IconRefresh className='h-4 w-4' />
          )}
          <span className='text-sm font-medium'>به‌روزرسانی</span>
        </Button>
      </div>

      <Button onClick={() => setOpen('add')} className='gap-2 transition-all'>
        <IconUserPlus className='h-4 w-4' />
        <span className='text-sm font-medium'>افزودن نماینده</span>
      </Button>
    </div>
  )
}
