import '@fontsource/vazirmatn/400.css'
import '@fontsource/vazirmatn/500.css'
import '@fontsource/vazirmatn/600.css'
import '@fontsource/vazirmatn/700.css'
import '@/features/reports/reports.css'
import { useNavigate } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/authStore.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { AnomalyAlertsTable } from '@/features/reports/components/anomaly-alerts-table.tsx'
import { DailyUsageChart } from '@/features/reports/components/daily-usage-chart.tsx'
import { ExpiringSoonTables } from '@/features/reports/components/expiring-soon-tables.tsx'
import { FinancialChart } from '@/features/reports/components/financial-chart.tsx'
import { OnlineUsersCard } from '@/features/reports/components/online-users-card.tsx'
import { PanelHealthTable } from '@/features/reports/components/panel-health-table.tsx'
import { PeakUsageChart } from '@/features/reports/components/peak-usage-chart.tsx'
import { PopularLocationsChart } from '@/features/reports/components/popular-locations-chart.tsx'
import { ProtocolShareDonut } from '@/features/reports/components/protocol-share-donut.tsx'
import { RenewalRateCard } from '@/features/reports/components/renewal-rate-card.tsx'
import { ReportsToolbar } from '@/features/reports/components/reports-toolbar.tsx'
import { ResellerActivityReport } from '@/features/reports/components/reseller-activity-report.tsx'
import { ResellerRankingReport } from '@/features/reports/components/reseller-ranking-report.tsx'
import { ResourceUsageChart } from '@/features/reports/components/resource-usage-chart.tsx'
import { UserRankingReport } from '@/features/reports/components/user-ranking-report.tsx'
import { ReportsRange } from '@/schema/reports.ts'

type ReportsSearch = {
  range: ReportsRange
}

type ReportsPageProps = {
  search: ReportsSearch
  onSearchChange: (search: ReportsSearch) => void
}

function SectionTitle({ children }: { children: React.ReactNode }) {
  return (
    <h3 className='text-muted-foreground mb-3 text-sm font-semibold tracking-tight'>
      {children}
    </h3>
  )
}

export default function ReportsPage({
  search,
  onSearchChange,
}: ReportsPageProps) {
  const admin = useAuthStore((state) => state.auth.admin)
  const navigate = useNavigate()
  const isAdmin = admin?.role !== 'reseller'
  const range = search.range

  if (!isAdmin) {
    void navigate({ to: '/403' })
    return null
  }

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
        <div
          dir='rtl'
          className='reports-root -mx-4 space-y-6 px-4 pb-8 sm:mx-0 sm:px-0'
        >
          <div className='animate-in fade-in slide-in-from-bottom-2 flex flex-col gap-1 duration-500'>
            <h2 className='text-2xl font-bold tracking-tight'>گزارش‌ها</h2>
            <p className='text-muted-foreground'>
              گزارش‌های تحلیلی و آماری پنل به‌صورت یکجا
            </p>
          </div>

          <ReportsToolbar
            range={range}
            onRangeChange={(newRange) => onSearchChange({ range: newRange })}
          />

          {/* Live / "right now" reports -- no range param */}
          <section>
            <SectionTitle>وضعیت زنده</SectionTitle>
            <div className='space-y-4'>
              <OnlineUsersCard />
              <div className='grid grid-cols-1 gap-4 lg:grid-cols-2'>
                <PanelHealthTable />
                <AnomalyAlertsTable />
              </div>
              <ExpiringSoonTables />
            </div>
          </section>

          {/* Usage trends */}
          <section>
            <SectionTitle>روند مصرف</SectionTitle>
            <div className='grid grid-cols-1 gap-4 lg:grid-cols-3'>
              <DailyUsageChart range={range} />
              <ProtocolShareDonut range={range} />
            </div>
          </section>

          <section>
            <div className='grid grid-cols-1 gap-4 lg:grid-cols-2'>
              <PeakUsageChart range={range} />
              <PopularLocationsChart range={range} />
            </div>
          </section>

          {/* Rankings */}
          <section>
            <SectionTitle>رتبه‌بندی‌ها</SectionTitle>
            <div className='grid grid-cols-1 gap-4 lg:grid-cols-2'>
              <ResellerRankingReport range={range} />
              <ResellerActivityReport range={range} />
            </div>
            <div className='mt-4'>
              <UserRankingReport range={range} />
            </div>
          </section>

          {/* Financial + renewal */}
          <section>
            <SectionTitle>مالی و تمدید</SectionTitle>
            <div className='grid grid-cols-1 gap-4 lg:grid-cols-3'>
              <FinancialChart range={range} />
              <RenewalRateCard range={range} />
            </div>
          </section>

          {/* Infrastructure */}
          <section>
            <SectionTitle>زیرساخت</SectionTitle>
            <ResourceUsageChart range={range} />
          </section>
        </div>
      </Main>
    </>
  )
}
