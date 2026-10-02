import { useDNSAccountsListQuery } from '@/hooks/dns-account/useDNSAccountsListQuery.ts'
import { useAuthStore } from '@/stores/authStore.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { dnsAccountsColumns } from '@/features/dns-accounts/components/dns-accounts-columns.tsx'
import { DNSAccountsDialogs } from '@/features/dns-accounts/components/dns-accounts-dialogs.tsx'
import { DNSAccountsPrimaryButtons } from '@/features/dns-accounts/components/dns-accounts-primary-buttons.tsx'
import DNSAccountsProvider from '@/features/dns-accounts/context/dns-accounts-context.tsx'
import { useAssignedDNSPanelSummariesQuery } from '@/hooks/dns-account/useAssignedDNSPanelSummariesQuery.ts'
import { DataTable } from '@/features/shared-components/table/data-table.tsx'
import { DataTableSkeleton } from '@/features/shared-components/table/data-table-skeleton.tsx'

export default function DNSAccounts() {
  const admin = useAuthStore((state) => state.auth.admin)
  const isReseller = admin?.role === 'reseller'
  const isAdmin = admin?.role === 'admin'

  const {
    data: accountsList,
    isLoading: isAccountsListLoading,
    refetch: refetchAccountsList,
    isRefetching: isAccountsListRefetching,
  } = useDNSAccountsListQuery()

  // DNS has no dedicated "canResellDns" flag on the reseller model -- panel
  // assignment alone gates creation, same effective rule V2Ray applies via
  // its candidatePanels.length check inside the form itself, surfaced here
  // too so the "Add Account" button is disabled up front instead of only
  // failing once the (empty) panel dropdown is opened.
  const { data: assignedPanelSummaries } = useAssignedDNSPanelSummariesQuery(
    isReseller ? (admin?.reseller_id ?? undefined) : undefined
  )

  const canCreate = isAdmin || Boolean(assignedPanelSummaries?.length)

  return (
    <DNSAccountsProvider>
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
              حساب‌های Smart DNS
            </h2>
            <p className='text-muted-foreground'>
              مدیریت حساب‌های Smart DNS ایجادشده روی پنل‌های DNS ثبت‌شده‌ی
              شما در این بخش -- سیستمی کاملاً جدا از وایرگاردها، یوزرمنجیر و
              V2Ray.
            </p>
          </div>
          <DNSAccountsPrimaryButtons
            refetchAccountsList={refetchAccountsList}
            isAccountsListRefetching={isAccountsListRefetching}
            canCreate={canCreate}
            accountsList={accountsList ?? []}
          />
        </div>

        {isReseller && !canCreate && (
          <p className='text-muted-foreground mb-4 text-sm'>
            شما در حال حاضر اجازه‌ی ایجاد حساب DNS را ندارید. برای درخواست
            دسترسی با ادمین خود تماس بگیرید.
          </p>
        )}

        <div className='-mx-4 flex-1 overflow-auto px-4 py-1'>
          {isAccountsListLoading ? (
            <DataTableSkeleton columns={7} />
          ) : (
            <DataTable data={accountsList ?? []} columns={dnsAccountsColumns} />
          )}
        </div>
      </Main>

      <DNSAccountsDialogs accountsList={accountsList ?? []} />
    </DNSAccountsProvider>
  )
}
