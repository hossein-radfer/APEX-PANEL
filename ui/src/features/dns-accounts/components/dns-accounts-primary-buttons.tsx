import { IconAlertTriangle, IconServerBolt, IconRefresh } from '@tabler/icons-react'
import { Loader2Icon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { DNSAccount } from '@/schema/dns-account.ts'
import { Button } from '@/components/ui/button'
import { useDNSAccounts } from '@/features/dns-accounts/context/dns-accounts-context.tsx'

interface Props {
  refetchAccountsList: () => void
  isAccountsListRefetching: boolean
  canCreate: boolean
  accountsList: DNSAccount[]
}

export function DNSAccountsPrimaryButtons({
  refetchAccountsList,
  isAccountsListRefetching,
  canCreate,
  accountsList,
}: Props) {
  const { setOpen } = useDNSAccounts()
  const expiredCount = accountsList.filter(
    (a) =>
      a.status === 'expired' ||
      a.status === 'suspended' ||
      (a.total_volume_bytes > 0 && a.used_bytes >= a.total_volume_bytes)
  ).length

  return (
    <div className='flex flex-wrap items-center gap-3'>
      <Button
        variant='outline'
        className={cn(
          'shadow-none',
          isAccountsListRefetching && 'cursor-not-allowed opacity-70'
        )}
        disabled={isAccountsListRefetching}
        onClick={refetchAccountsList}
      >
        {isAccountsListRefetching ? (
          <Loader2Icon className='h-4 w-4 animate-spin' />
        ) : (
          <IconRefresh className='h-4 w-4' />
        )}
        <span className='text-sm font-medium'>به‌روزرسانی</span>
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
        title={canCreate ? undefined : 'شما اجازه‌ی ایجاد حساب DNS را ندارید'}
      >
        <span className='text-sm font-medium'>افزودن حساب</span>
        <IconServerBolt className='h-4 w-4' />
      </Button>
    </div>
  )
}
