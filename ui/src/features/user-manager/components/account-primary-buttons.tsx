import { IconAlertTriangle, IconRefresh, IconUpload, IconUsersPlus, IconUserPlus } from '@tabler/icons-react'
import { Loader2Icon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { UserManagerAccount } from '@/schema/user-manager.ts'
import { Button } from '@/components/ui/button'
import { useUserManager } from '@/features/user-manager/context/user-manager-context.tsx'

interface Props {
  refetchAccountsList: () => void
  isAccountsListRefetching: boolean
  canCreate: boolean
  // Bulk import for the general (Super Admin) User Manager page is
  // restricted strictly to admins -- resellers only ever get it scoped to
  // their own accounts via the separate Reseller User Manager page.
  canBulkImport: boolean
  accountsList: UserManagerAccount[]
}

export function AccountPrimaryButtons({
  refetchAccountsList,
  isAccountsListRefetching,
  canCreate,
  canBulkImport,
  accountsList,
}: Props) {
  const { setOpen } = useUserManager()
  const expiredCount = accountsList.filter(
    (a) => a.status.includes('expired') || a.status.includes('suspended')
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
        <span className='text-sm font-medium'>تازه‌سازی</span>
      </Button>

      {canBulkImport && (
        <Button
          variant='outline'
          onClick={() => setOpen('bulk-import')}
          className='gap-2'
        >
          <span className='text-sm font-medium'>وارد کردن گروهی</span>
          <IconUpload className='h-4 w-4' />
        </Button>
      )}

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
        variant='outline'
        onClick={() => setOpen('bulk-create')}
        className='gap-2 transition-all shadow-none'
        disabled={!canCreate}
        title={
          canCreate
            ? undefined
            : 'شما اجازه‌ی ایجاد حساب‌های User Manager را ندارید'
        }
      >
        <span className='text-sm font-medium'>ساخت گروهی</span>
        <IconUsersPlus className='h-4 w-4' />
      </Button>

      <Button
        onClick={() => setOpen('add')}
        className='gap-2 transition-all'
        disabled={!canCreate}
        title={
          canCreate
            ? undefined
            : 'شما اجازه‌ی ایجاد حساب‌های User Manager را ندارید'
        }
      >
        <span className='text-sm font-medium'>افزودن حساب</span>
        <IconUserPlus className='h-4 w-4' />
      </Button>
    </div>
  )
}
