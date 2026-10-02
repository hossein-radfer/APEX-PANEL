import { IconAlertTriangle, IconCloudDown, IconRefresh, IconUsersPlus, IconUserPlus } from '@tabler/icons-react'
import { FileSpreadsheet, Loader2Icon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useResetUsagesMutation } from '@/hooks/peers/useResetUsagesMutation.ts'
import { useTrafficExcelMutation } from '@/hooks/peers/useTrafficExcelMutation.ts'
import { Peer } from '@/schema/peers.ts'
import { Button } from '@/components/ui/button'
import { ResetUsagesDialog } from '@/features/peers/components/dialogs/peers-reset-usages-dialogs.tsx'
import { usePeers } from '@/features/peers/context/peers-context.tsx'
import { useAuthStore } from '@/stores/authStore.ts'

interface Props {
  refetchPeersList: () => void
  isPeersListRefetching: boolean
  peersList: Peer[]
}

export function PeersPrimaryButtons({
  refetchPeersList,
  isPeersListRefetching,
  peersList,
}: Props) {
  const { setOpen } = usePeers()
  const role = useAuthStore((state) => state.auth.admin?.role)
  const isReseller = role === 'reseller'
  const expiredCount = peersList.filter(
    (p) => p.status.includes('expired') || p.status.includes('suspended')
  ).length

  const { mutateAsync: resetUsages, isPending: isResetUsagesPending } =
    useResetUsagesMutation()

  const {
    mutateAsync: exportTrafficExcel,
    isPending: isExportingTrafficExcel,
  } = useTrafficExcelMutation()

  return (
    <div className='flex flex-wrap items-center gap-3'>
      <div className='inline-flex w-fit -space-x-px rounded-md shadow-xs rtl:space-x-reverse'>
        {!isReseller && (
          <Button
            variant='outline'
            className={cn('rounded-none rounded-s-md shadow-none transition-all focus-visible:z-10')}
            onClick={() => setOpen('sync')}
          >
            <IconCloudDown className='h-4 w-4' />
            <span className='text-sm font-medium'>همگام‌سازی</span>
          </Button>
        )}

        <Button
          variant='outline'
          className={cn(
            isReseller
              ? 'rounded-md shadow-none focus-visible:z-10'
              : 'rounded-none rounded-e-md shadow-none focus-visible:z-10',
            isPeersListRefetching && 'cursor-not-allowed opacity-70'
          )}
          disabled={isPeersListRefetching}
          onClick={refetchPeersList}
        >
          {isPeersListRefetching ? (
            <Loader2Icon className='h-4 w-4 animate-spin' />
          ) : (
            <IconRefresh className='h-4 w-4' />
          )}
          <span className='text-sm font-medium'>تازه‌سازی</span>
        </Button>
      </div>

      {!isReseller && (
        <ResetUsagesDialog
          isPending={isResetUsagesPending}
          resetUsages={resetUsages}
        />
      )}

      {!isReseller && (
        // Confirmed, reported inconsistency: this was colored the same
        // green family used for "success" states elsewhere, even though
        // exporting a file isn't a success/positive-outcome action --
        // kept as a plain neutral outline button, matching
        // "همگام‌سازی"/"تازه‌سازی" above and "ساخت گروهی" below (every
        // other routine, non-destructive, non-alerting action in this
        // toolbar).
        <Button
          variant='outline'
          className='gap-2 shadow-none transition-all'
          disabled={isExportingTrafficExcel}
          onClick={() => exportTrafficExcel()}
        >
          <span className='text-sm font-medium'>خروجی اکسل ترافیک</span>
          {isExportingTrafficExcel ? (
            <Loader2Icon className='h-4 w-4 animate-spin' />
          ) : (
            <FileSpreadsheet className='h-4 w-4' />
          )}
        </Button>
      )}

      {expiredCount > 0 && (
        // Confirmed, reported inconsistency: this used the exact same
        // amber as "بازنشانی مصرف‌ها" (ResetUsagesDialog) below, even
        // though the two mean very different things -- one is a genuine
        // irreversible-action warning (about to reset data), the other is
        // a status indicator (N peers already expired, no action taken by
        // clicking it beyond opening a list). Moved to red/rose -- this
        // codebase's convention for "needs attention / problem exists"
        // (see e.g. the destructive confirm button in
        // peers-reset-usages-dialogs.tsx) -- so the two are visually
        // distinguishable at a glance.
        <Button
          variant='outline'
          className='gap-2 border-rose-500 text-rose-600 hover:bg-rose-100/60 dark:border-rose-500 dark:text-rose-400 dark:hover:bg-rose-400/10'
          onClick={() => setOpen('expired')}
        >
          <IconAlertTriangle className='h-4 w-4' />
          <span className='text-sm font-medium'>منقضی‌شده ({expiredCount})</span>
        </Button>
      )}

      <Button
        variant='outline'
        onClick={() => setOpen('bulk-create')}
        className='gap-2 transition-all shadow-none'
      >
        <span className='text-sm font-medium'>ساخت گروهی</span>
        <IconUsersPlus className='h-4 w-4' />
      </Button>

      <Button onClick={() => setOpen('add')} className='gap-2 transition-all'>
        <span className='text-sm font-medium'>افزودن وایرگارد</span>
        <IconUserPlus className='h-4 w-4' />
      </Button>
    </div>
  )
}
