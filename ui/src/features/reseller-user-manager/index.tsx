import { useMemo, useState } from 'react'
import { IconRefresh, IconUpload, IconUserPlus } from '@tabler/icons-react'
import { Loader2Icon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useUserManagerAccountsByResellerQuery } from '@/hooks/user-manager/useUserManagerAccountsByResellerQuery.ts'
import { useResellersListQuery } from '@/hooks/resellers/useResellersListQuery.ts'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { DataTable } from '@/features/shared-components/table/data-table.tsx'
import { DataTableSkeleton } from '@/features/shared-components/table/data-table-skeleton.tsx'
import { resellerAccountColumns } from '@/features/reseller-user-manager/components/reseller-account-columns.tsx'
import { ResellerAccountDialogs } from '@/features/reseller-user-manager/components/reseller-account-dialogs.tsx'
import ResellerUserManagerProvider, {
  useResellerUserManager,
} from '@/features/reseller-user-manager/context/reseller-user-manager-context.tsx'

function ResellerUserManagerPrimaryButtons({
  resellerId,
  refetch,
  isRefetching,
}: {
  resellerId: number
  refetch: () => void
  isRefetching: boolean
}) {
  const { setOpen } = useResellerUserManager()

  return (
    <div className='flex flex-wrap items-center gap-3'>
      <Button
        variant='outline'
        className={cn(
          'shadow-none',
          isRefetching && 'cursor-not-allowed opacity-70'
        )}
        disabled={isRefetching}
        onClick={refetch}
      >
        {isRefetching ? (
          <Loader2Icon className='h-4 w-4 animate-spin' />
        ) : (
          <IconRefresh className='h-4 w-4' />
        )}
        <span className='text-sm font-medium'>به‌روزرسانی</span>
      </Button>

      <Button
        variant='outline'
        onClick={() => setOpen('bulk-import')}
        className='gap-2'
        disabled={!resellerId}
      >
        <span className='text-sm font-medium'>ورود دسته‌ای</span>
        <IconUpload className='h-4 w-4' />
      </Button>

      <Button
        onClick={() => setOpen('add')}
        className='gap-2 transition-all'
        disabled={!resellerId}
      >
        <span className='text-sm font-medium'>افزودن حساب</span>
        <IconUserPlus className='h-4 w-4' />
      </Button>
    </div>
  )
}

function ResellerUserManagerContent() {
  const [pickedResellerID, setPickedResellerID] = useState('')
  const { data: resellers = [] } = useResellersListQuery()
  const { setOpen, setCurrentRow } = useResellerUserManager()

  const resellerId = useMemo(() => {
    const parsed = Number.parseInt(pickedResellerID, 10)
    return Number.isFinite(parsed) && parsed > 0 ? parsed : null
  }, [pickedResellerID])

  const handleResellerChange = (value: string) => {
    setPickedResellerID(value)
    setOpen(null)
    setCurrentRow(null)
  }

  const {
    data: accountsList,
    isLoading: isAccountsListLoading,
    refetch: refetchAccountsList,
    isRefetching: isAccountsListRefetching,
  } = useUserManagerAccountsByResellerQuery(resellerId ?? undefined)

  return (
    <>
      <Header fixed>
        <Search />
        <div className='ml-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>
              User Manager نمایندگان
            </h2>
            <p className='text-muted-foreground'>
              یک نماینده را انتخاب کنید تا حساب‌های L2TP/PPTP/SSTP/OpenVPN
              او را مشاهده و مدیریت کنید
            </p>
          </div>
          <ResellerUserManagerPrimaryButtons
            resellerId={resellerId ?? 0}
            refetch={refetchAccountsList}
            isRefetching={isAccountsListRefetching}
          />
        </div>

        <Card className='mb-4'>
          <CardHeader className='pb-2'>
            <CardTitle className='text-base'>نماینده</CardTitle>
          </CardHeader>
          <CardContent className='max-w-sm space-y-2'>
            <Label htmlFor='reseller-user-manager-select'>
              یک نماینده را انتخاب کنید
            </Label>
            <Select
              value={pickedResellerID}
              onValueChange={handleResellerChange}
            >
              <SelectTrigger id='reseller-user-manager-select' className='w-full'>
                <SelectValue placeholder='یک نماینده را انتخاب کنید' />
              </SelectTrigger>
              <SelectContent>
                {resellers.length === 0 ? (
                  <div className='text-muted-foreground px-2 py-1.5 text-sm'>
                    هیچ نماینده‌ای یافت نشد.
                  </div>
                ) : (
                  resellers.map((reseller) => (
                    <SelectItem key={reseller.id} value={String(reseller.id)}>
                      {reseller.name} ({reseller.username})
                      {!reseller.canCreateUserManagerAccounts
                        ? ' — ایجاد حساب غیرفعال است'
                        : ''}
                    </SelectItem>
                  ))
                )}
              </SelectContent>
            </Select>
          </CardContent>
        </Card>

        <div className='-mx-4 flex-1 overflow-auto px-4 py-1'>
          {!resellerId ? (
            <Card>
              <CardContent className='pt-6'>
                <p className='text-muted-foreground'>
                  برای مشاهده حساب‌ها، ابتدا یک نماینده را از بالا انتخاب کنید.
                </p>
              </CardContent>
            </Card>
          ) : isAccountsListLoading ? (
            <DataTableSkeleton columns={5} />
          ) : (
            <DataTable
              data={accountsList ?? []}
              columns={resellerAccountColumns}
              initialSorting={[{ id: 'username', desc: false }]}
            />
          )}
        </div>
      </Main>

      {resellerId && <ResellerAccountDialogs resellerId={resellerId} />}
    </>
  )
}

export default function ResellerUserManager() {
  return (
    <ResellerUserManagerProvider>
      <ResellerUserManagerContent />
    </ResellerUserManagerProvider>
  )
}
