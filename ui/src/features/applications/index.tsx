import { useApplicationsListQuery } from '@/hooks/applications/useApplicationsListQuery.ts'
import { useResellerQuery } from '@/hooks/resellers/useResellerQuery.ts'
import { useAuthStore } from '@/stores/authStore.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { applicationColumns } from '@/features/applications/components/application-columns.tsx'
import { ApplicationDialogs } from '@/features/applications/components/application-dialogs.tsx'
import { ApplicationPrimaryButtons } from '@/features/applications/components/application-primary-buttons.tsx'
import ApplicationProvider from '@/features/applications/context/application-context.tsx'
import { DataTable } from '@/features/shared-components/table/data-table.tsx'
import { DataTableSkeleton } from '@/features/shared-components/table/data-table-skeleton.tsx'

export default function Applications() {
  const admin = useAuthStore((state) => state.auth.admin)
  const isReseller = admin?.role === 'reseller'
  const isAdmin = admin?.role === 'admin'

  const {
    data: applicationsList,
    isLoading: isApplicationsListLoading,
    refetch: refetchApplicationsList,
    isRefetching: isApplicationsListRefetching,
  } = useApplicationsListQuery()

  const { data: selfReseller } = useResellerQuery(
    isReseller ? admin?.reseller_id : null
  )

  // مثل V2Ray/UserManager، ساخت اپلیکیشن هم اکنون یک سوییچ سطح-reseller
  // اختصاصی دارد (Reseller.CanCreateApplications) که فقط ادمین آن را
  // تنظیم می‌کند؛ اگر خاموش باشد دکمه‌ی «ساخت اپلیکیشن» اصلاً نمایش داده
  // نمی‌شود. حتی وقتی روشن است، ساخت هر Application همچنان به تخصیص
  // جداگانه‌ی هر پروتکل (اینترفیس وایرگارد، یا اجازه‌ی یوزرمنجیر/V2Ray)
  // نیاز دارد که داخل فرم ساخت بررسی می‌شود.
  const canCreate = isAdmin || Boolean(selfReseller?.canCreateApplications)

  return (
    <ApplicationProvider>
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
            <h2 className='text-2xl font-bold tracking-tight'>اپلیکیشن</h2>
            <p className='text-muted-foreground'>
              یوزر/پسورد اپلیکیشن موبایل کاربران نهایی را اینجا مدیریت کنید --
              هر اپلیکیشن می‌تواند به چند پروتکل/موقعیت هم‌زمان دسترسی داشته
              باشد.
            </p>
          </div>
          <ApplicationPrimaryButtons
            refetchApplicationsList={refetchApplicationsList}
            isApplicationsListRefetching={isApplicationsListRefetching}
            canCreate={canCreate}
          />
        </div>

        <div className='-mx-4 flex-1 overflow-auto px-4 py-1'>
          {isApplicationsListLoading ? (
            <DataTableSkeleton columns={7} />
          ) : (
            <DataTable
              data={applicationsList ?? []}
              columns={applicationColumns}
            />
          )}
        </div>
      </Main>

      <ApplicationDialogs />
    </ApplicationProvider>
  )
}
