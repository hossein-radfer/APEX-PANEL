import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { CostsTab } from './components/costs-tab'
import { DashboardTab } from './components/dashboard-tab'
import { LocationProfitabilityTab } from './components/location-profitability-tab'
import { PartnersTab } from './components/partners-tab'
import { PaymentsTab } from './components/payments-tab'
import { UserProfitabilityTab } from './components/user-profitability-tab'

export default function Accounting() {
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
        <div className='mb-4 space-y-1'>
          <h1 className='text-2xl font-bold tracking-tight'>حسابداری</h1>
          <p className='text-muted-foreground'>
            هزینه‌های پرداختی به همکاران، پرداخت‌های دریافتی از مشتریان، و
            سود/زیان کلی -- کاملاً مستقل از کیف پول و فاکتور نمایندگان.
          </p>
        </div>

        <Tabs defaultValue='dashboard' className='space-y-4'>
          <TabsList>
            <TabsTrigger value='dashboard'>داشبورد</TabsTrigger>
            <TabsTrigger value='users'>سود هر کاربر</TabsTrigger>
            <TabsTrigger value='locations'>سود هر لوکیشن</TabsTrigger>
            <TabsTrigger value='payments'>پرداخت‌های دریافتی</TabsTrigger>
            <TabsTrigger value='costs'>هزینه‌ها</TabsTrigger>
            <TabsTrigger value='partners'>همکاران</TabsTrigger>
          </TabsList>

          <TabsContent value='dashboard'>
            <DashboardTab />
          </TabsContent>
          <TabsContent value='users'>
            <UserProfitabilityTab />
          </TabsContent>
          <TabsContent value='locations'>
            <LocationProfitabilityTab />
          </TabsContent>
          <TabsContent value='payments'>
            <PaymentsTab />
          </TabsContent>
          <TabsContent value='costs'>
            <CostsTab />
          </TabsContent>
          <TabsContent value='partners'>
            <PartnersTab />
          </TabsContent>
        </Tabs>
      </Main>
    </>
  )
}
