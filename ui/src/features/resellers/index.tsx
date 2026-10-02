import { useMemo } from 'react'
import { useResellersListQuery } from '@/hooks/resellers/useResellersListQuery.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { DataTable } from '@/features/shared-components/table/data-table.tsx'
import { DataTableSkeleton } from '@/features/shared-components/table/data-table-skeleton.tsx'
import { ResellersProvider } from '@/features/resellers/context/resellers-context.tsx'
import { ResellersDialogs } from '@/features/resellers/components/resellers-dialogs.tsx'
import { resellersColumns } from '@/features/resellers/components/resellers-columns.tsx'
import { ResellersPrimaryButtons } from '@/features/resellers/components/resellers-primary-buttons.tsx'
import { Button } from '@/components/ui/button.tsx'

export default function Resellers() {
  const {
    data: resellers,
    isLoading,
    refetch,
    isRefetching,
  } = useResellersListQuery()

  const exportCsv = () => {
    const rows = resellers ?? []
    const header = ['نام', 'نام کاربری', 'ایمیل', 'فعال', 'سهمیه (بایت)', 'مصرف‌شده (بایت)']
    const csv = [header.join(',')]
      .concat(
        rows.map((reseller) =>
          [
            reseller.name,
            reseller.username,
            reseller.email ?? '',
            reseller.isActive ? 'فعال' : 'غیرفعال',
            reseller.quotaBytes?.toString() ?? 'نامحدود',
            reseller.usedBytes.toString(),
          ]
            .map((value) => `"${String(value).replace(/"/g, '""')}"`)
            .join(',')
        )
      )
      .join('\n')

    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.setAttribute('download', `resellers-${new Date().toISOString().slice(0, 10)}.csv`)
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
  }

  const tableData = useMemo(() => resellers ?? [], [resellers])

  return (
    <ResellersProvider>
      <Header fixed>
        <div className='ml-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main>
        <div className='mb-2 flex flex-wrap items-center justify-between gap-4'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>نمایندگان</h2>
            <p className='text-muted-foreground'>مدیریت حساب‌های نمایندگان و سهمیه‌ها</p>
          </div>
          <div className='flex flex-wrap items-center gap-3'>
            <Button variant='outline' onClick={exportCsv}>
              خروجی CSV
            </Button>
            <ResellersPrimaryButtons
              refetchResellersList={refetch}
              isResellersListRefetching={isRefetching}
            />
          </div>
        </div>

        <div className='-mx-4 flex-1 overflow-auto px-4 py-1'>
          {isLoading ? (
            <DataTableSkeleton columns={6} />
          ) : (
            <DataTable
              data={tableData}
              columns={resellersColumns}
              initialSorting={[{ id: 'name', desc: false }]}
            />
          )}
        </div>
      </Main>

      <ResellersDialogs />
    </ResellersProvider>
  )
}
