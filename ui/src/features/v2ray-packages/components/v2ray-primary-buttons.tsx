import { IconAlertTriangle, IconPackage, IconPackages, IconRefresh } from '@tabler/icons-react'
import { Loader2Icon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { V2RayPackage } from '@/schema/v2ray.ts'
import { Button } from '@/components/ui/button'
import { useV2Ray } from '@/features/v2ray-packages/context/v2ray-context.tsx'

interface Props {
  refetchPackagesList: () => void
  isPackagesListRefetching: boolean
  canCreate: boolean
  packagesList: V2RayPackage[]
}

export function V2RayPrimaryButtons({
  refetchPackagesList,
  isPackagesListRefetching,
  canCreate,
  packagesList,
}: Props) {
  const { setOpen } = useV2Ray()
  const expiredCount = packagesList.filter(
    (p) =>
      p.status === 'expired' ||
      p.status === 'suspended' ||
      p.used_bytes >= p.total_volume_bytes
  ).length

  return (
    <div className='flex flex-wrap items-center gap-3'>
      <Button
        variant='outline'
        className={cn(
          'shadow-none',
          isPackagesListRefetching && 'cursor-not-allowed opacity-70'
        )}
        disabled={isPackagesListRefetching}
        onClick={refetchPackagesList}
      >
        {isPackagesListRefetching ? (
          <Loader2Icon className='h-4 w-4 animate-spin' />
        ) : (
          <IconRefresh className='h-4 w-4' />
        )}
        <span className='text-sm font-medium'>به‌روزرسانی</span>
      </Button>

      <Button
        variant='outline'
        onClick={() => setOpen('bulk-create')}
        className='gap-2 transition-all shadow-none'
        disabled={!canCreate}
        title={
          canCreate ? undefined : 'شما اجازه‌ی ایجاد بسته‌ی V2Ray را ندارید'
        }
      >
        <span className='text-sm font-medium'>ایجاد گروهی</span>
        <IconPackages className='h-4 w-4' />
      </Button>

      {expiredCount > 0 && (
        <Button
          variant='outline'
          className='gap-2 border-amber-600 text-amber-600 hover:bg-amber-100/60 dark:border-amber-500 dark:text-amber-400 dark:hover:bg-amber-400/10'
          onClick={() => setOpen('expired')}
        >
          <IconAlertTriangle className='h-4 w-4' />
          <span className='text-sm font-medium'>منقضی‌شده ({expiredCount})</span>
        </Button>
      )}

      <Button
        onClick={() => setOpen('add')}
        className='gap-2 transition-all'
        disabled={!canCreate}
        title={
          canCreate ? undefined : 'شما اجازه‌ی ایجاد بسته‌ی V2Ray را ندارید'
        }
      >
        <span className='text-sm font-medium'>افزودن بسته</span>
        <IconPackage className='h-4 w-4' />
      </Button>
    </div>
  )
}
