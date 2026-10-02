import { useMemo, useState } from 'react'
import { IconPackage, IconRefresh } from '@tabler/icons-react'
import { Loader2Icon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useDNSAccountsByResellerQuery } from '@/hooks/dns-account/useDNSAccountsByResellerQuery.ts'
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
import { resellerDNSColumns } from '@/features/reseller-dns/components/reseller-dns-columns.tsx'
import { ResellerDNSDialogs } from '@/features/reseller-dns/components/reseller-dns-dialogs.tsx'
import ResellerDNSProvider, {
  useResellerDNS,
} from '@/features/reseller-dns/context/reseller-dns-context.tsx'

function ResellerDNSPrimaryButtons({
  resellerId,
  refetch,
  isRefetching,
}: {
  resellerId: number
  refetch: () => void
  isRefetching: boolean
}) {
  const { setOpen } = useResellerDNS()

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
        <span className='text-sm font-medium'>بازخوانی</span>
      </Button>

      <Button
        onClick={() => setOpen('add')}
        className='gap-2 transition-all'
        disabled={!resellerId}
      >
        <span className='text-sm font-medium'>افزودن حساب</span>
        <IconPackage className='h-4 w-4' />
      </Button>
    </div>
  )
}

function ResellerDNSContent() {
  const [pickedResellerID, setPickedResellerID] = useState('')
  const { data: resellers = [] } = useResellersListQuery()
  const { setOpen, setCurrentRow } = useResellerDNS()

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
  } = useDNSAccountsByResellerQuery(resellerId ?? undefined)

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
              DNS نمایندگان
            </h2>
            <p className='text-muted-foreground'>
              یک نماینده را انتخاب کنید تا حساب‌های DNS او را مشاهده و مدیریت کنید
            </p>
          </div>
          <ResellerDNSPrimaryButtons
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
            <Label htmlFor='reseller-dns-select'>یک نماینده را انتخاب کنید</Label>
            <Select
              value={pickedResellerID}
              onValueChange={handleResellerChange}
            >
              <SelectTrigger id='reseller-dns-select' className='w-full'>
                <SelectValue placeholder='یک نماینده را انتخاب کنید' />
              </SelectTrigger>
              <SelectContent>
                {resellers.length === 0 ? (
                  <div className='text-muted-foreground px-2 py-1.5 text-sm'>
                    نماینده‌ای یافت نشد.
                  </div>
                ) : (
                  resellers.map((reseller) => (
                    <SelectItem key={reseller.id} value={String(reseller.id)}>
                      {reseller.name} ({reseller.username})
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
                  برای مشاهده حساب‌ها، یکی از نمایندگان بالا را انتخاب کنید.
                </p>
              </CardContent>
            </Card>
          ) : isAccountsListLoading ? (
            <DataTableSkeleton columns={5} />
          ) : (
            <DataTable
              data={accountsList ?? []}
              columns={resellerDNSColumns}
            />
          )}
        </div>
      </Main>

      {resellerId && <ResellerDNSDialogs resellerId={resellerId} />}
    </>
  )
}

export default function ResellerDNS() {
  return (
    <ResellerDNSProvider>
      <ResellerDNSContent />
    </ResellerDNSProvider>
  )
}
