import { useUserManagerAccountsListQuery } from '@/hooks/user-manager/useUserManagerAccountsListQuery.ts'
import { useResellerQuery } from '@/hooks/resellers/useResellerQuery.ts'
import { useAssignedUserManagerGroupsQuery } from '@/hooks/resellers/useAssignedUserManagerGroupsQuery.ts'
import { useAssignedUserManagerProfilesQuery } from '@/hooks/resellers/useAssignedUserManagerProfilesQuery.ts'
import { useAuthStore } from '@/stores/authStore.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ThemeSwitch } from '@/components/theme-switch'
import { accountColumns } from '@/features/user-manager/components/account-columns.tsx'
import { AccountDialogs } from '@/features/user-manager/components/account-dialogs.tsx'
import { AccountPrimaryButtons } from '@/features/user-manager/components/account-primary-buttons.tsx'
import { GroupsTab } from '@/features/user-manager/components/groups-tab.tsx'
import { ProfilesTab } from '@/features/user-manager/components/profiles-tab.tsx'
import { ProtocolSettingsTab } from '@/features/user-manager/components/protocol-settings-tab.tsx'
import UserManagerProvider from '@/features/user-manager/context/user-manager-context.tsx'
import { DataTable } from '@/features/shared-components/table/data-table.tsx'
import { DataTableSkeleton } from '../shared-components/table/data-table-skeleton.tsx'

export default function UserManager() {
  const admin = useAuthStore((state) => state.auth.admin)
  const isReseller = admin?.role === 'reseller'
  const isAdmin = admin?.role === 'admin'

  const {
    data: accountsList,
    isLoading: isAccountsListLoading,
    refetch: refetchAccountsList,
    isRefetching: isAccountsListRefetching,
  } = useUserManagerAccountsListQuery()

  const { data: selfReseller } = useResellerQuery(
    isReseller ? admin?.reseller_id : null
  )

  const canCreate = isAdmin || Boolean(selfReseller?.canCreateUserManagerAccounts)

  // A self-serve reseller only ever sees the groups/profiles the admin has
  // explicitly assigned to them (see ResellerUserManagerGroup/Profile) --
  // undefined for the admin's own page, which means "show everything".
  const { data: allowedGroupNames } = useAssignedUserManagerGroupsQuery(
    isReseller ? (admin?.reseller_id ?? undefined) : undefined
  )
  const { data: allowedProfileNames } = useAssignedUserManagerProfilesQuery(
    isReseller ? (admin?.reseller_id ?? undefined) : undefined
  )

  return (
    <UserManagerProvider>
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
            <h2 className='text-2xl font-bold tracking-tight'>User Manager</h2>
            <p className='text-muted-foreground'>
              مدیریت حساب‌های L2TP، PPTP، SSTP و OpenVPN در این بخش -- سیستمی
              کاملاً جدا از وایرگاردها.
            </p>
          </div>
          <AccountPrimaryButtons
            refetchAccountsList={refetchAccountsList}
            isAccountsListRefetching={isAccountsListRefetching}
            canCreate={canCreate}
            canBulkImport={isAdmin}
            accountsList={accountsList ?? []}
          />
        </div>

        {isReseller && !canCreate && (
          <p className='text-muted-foreground mb-4 text-sm'>
            شما در حال حاضر اجازه ایجاد حساب User Manager را ندارید. برای
            درخواست دسترسی با مدیر خود تماس بگیرید.
          </p>
        )}

        <Tabs defaultValue='accounts' className='space-y-4'>
          <TabsList>
            <TabsTrigger value='accounts'>حساب‌ها</TabsTrigger>
            <TabsTrigger value='groups'>گروه‌ها</TabsTrigger>
            <TabsTrigger value='profiles'>پروفایل‌ها</TabsTrigger>
            {isAdmin && (
              <TabsTrigger value='protocol-settings'>
                تنظیمات پروتکل
              </TabsTrigger>
            )}
          </TabsList>

          <TabsContent value='accounts'>
            <div className='-mx-4 flex-1 overflow-auto px-4 py-1'>
              {isAccountsListLoading ? (
                <DataTableSkeleton columns={5} />
              ) : (
                <DataTable
                  data={accountsList ?? []}
                  columns={accountColumns}
                  initialSorting={[{ id: 'username', desc: false }]}
                />
              )}
            </div>
          </TabsContent>

          <TabsContent value='groups'>
            <GroupsTab
              allowedGroupNames={isReseller ? (allowedGroupNames ?? []) : undefined}
            />
          </TabsContent>

          <TabsContent value='profiles'>
            <ProfilesTab
              allowedProfileNames={
                isReseller ? (allowedProfileNames ?? []) : undefined
              }
            />
          </TabsContent>

          {isAdmin && (
            <TabsContent value='protocol-settings'>
              <ProtocolSettingsTab />
            </TabsContent>
          )}
        </Tabs>
      </Main>

      <AccountDialogs accountsList={accountsList ?? []} />
    </UserManagerProvider>
  )
}
