import { Outlet } from '@tanstack/react-router'
import {
  IconBrandTelegram,
  IconPalette,
  IconServer,
  IconShieldLock,
  IconTool,
} from '@tabler/icons-react'
import { useAuthStore } from '@/stores/authStore.ts'
import { Separator } from '@/components/ui/separator'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import SidebarNav from './components/sidebar-nav'

export default function Settings() {
  const role = useAuthStore((state) => state.auth.admin?.role)
  const isReseller = role === 'reseller'
  // Bot-as-a-service (item 6) is reseller-only, so it's appended just for
  // resellers, mirroring how systemNavItem/licenseNavItem below are
  // appended just for admins.
  const navItems = isReseller
    ? [...sidebarNavItems, resellerBotNavItem]
    : [...sidebarNavItems, systemNavItem, licenseNavItem]

  return (
    <>
      {/* ===== Top Heading ===== */}
      <Header>
        <Search />
        <div className='ml-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main fixed>
        <div className='space-y-0.5'>
          <h1 className='text-2xl font-bold tracking-tight md:text-3xl'>
            تنظیمات
          </h1>
          <p className='text-muted-foreground'>مدیریت تنظیمات حساب شما</p>
        </div>
        <Separator className='my-4 lg:my-6' />
        <div className='flex flex-1 flex-col space-y-2 overflow-hidden md:space-y-2 lg:flex-row lg:space-y-0 lg:space-x-12'>
          <aside className='top-0 lg:sticky lg:w-1/5'>
            <SidebarNav items={navItems} />
          </aside>
          <div className='flex w-full overflow-y-hidden p-1'>
            <Outlet />
          </div>
        </div>
      </Main>
    </>
  )
}

const sidebarNavItems = [
  {
    title: 'حساب کاربری',
    icon: <IconTool size={18} />,
    href: '/settings/account',
  },
  {
    title: 'ظاهر',
    icon: <IconPalette size={18} />,
    href: '/settings/appearance',
  },
]

const systemNavItem = {
  title: 'سیستم',
  icon: <IconServer size={18} />,
  href: '/settings/system',
}

const licenseNavItem = {
  title: 'لایسنس',
  icon: <IconShieldLock size={18} />,
  href: '/settings/license',
}

const resellerBotNavItem = {
  title: 'ربات فروش',
  icon: <IconBrandTelegram size={18} />,
  href: '/settings/reseller-bot',
}
