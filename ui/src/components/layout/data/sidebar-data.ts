import {
  IconChartBar,
  IconCloud,
  IconCoin,
  IconFileText,
  IconHelp,
  IconLayoutDashboard,
  IconReceipt,
  IconLink,
  IconPackage,
  IconPalette,
  IconBrandTelegram,
  IconDeviceMobile,
  IconRadar,
  IconActivityHeartbeat,
  IconServerBolt,
  IconSettings,
  IconShieldLock,
  IconTool,
  IconUsers,
  IconWallet,
  IconWorld,
} from '@tabler/icons-react'
import { type SidebarData } from '../types'

// adminSidebarData was reorganized from a single flat 30-item "General"
// list into logically-titled groups -- a confirmed, reported bug: "چون
// در حال ارتقا بودیم همش در حال اظافه بودیم و ترتیب منو ها... به صورت
// استاندارد نیست" (every new feature was appended to the same flat list
// as it shipped, with no structural grouping at all, so navigating this
// menu meant scanning WireGuard/UserManager/V2Ray/billing/reports/
// security/admin-only items all interleaved together in ship order
// rather than by what they actually relate to). Every URL below is
// UNCHANGED -- this only regroups existing items under new group
// headers, so no route/link anywhere else in the app needed to change.
//
// Full Persian localization pass (item 13): every group/item title below
// is now Persian, except protocol/technical names (WireGuard, V2Ray,
// X-UI, UM) which stay as-is per the admin's own explicit instruction to
// keep technical/protocol terminology untranslated.
const adminSidebarData: SidebarData = {
  navGroups: [
    {
      title: 'عمومی',
      items: [
        {
          title: 'داشبورد',
          url: '/',
          icon: IconLayoutDashboard,
        },
        {
          title: 'سرورها',
          url: '/servers',
          icon: IconCloud,
        },
        {
          title: 'نمایندگان',
          url: '/resellers',
          icon: IconUsers,
        },
      ],
    },
    {
      title: 'WireGuard',
      items: [
        {
          title: 'اینترفیس‌ها',
          url: '/interfaces',
          icon: IconWorld,
        },
        {
          title: 'استخرهای IP',
          url: '/pools',
          icon: IconLink,
        },
        {
          title: 'وایرگارد',
          url: '/peers',
          icon: IconUsers,
        },
        {
          title: 'وایرگارد نمایندگان',
          url: '/reseller-peers',
          icon: IconUsers,
        },
      ],
    },
    {
      title: 'User Manager',
      items: [
        {
          title: 'User Manager',
          url: '/user-manager',
          icon: IconShieldLock,
        },
        {
          title: 'User Manager نمایندگان',
          url: '/reseller-user-manager',
          icon: IconShieldLock,
        },
      ],
    },
    {
      title: 'V2Ray',
      items: [
        {
          title: 'پنل‌های X-UI',
          url: '/xui-panels',
          icon: IconServerBolt,
        },
        {
          title: 'پکیج‌های V2Ray',
          url: '/v2ray-packages',
          icon: IconShieldLock,
        },
        {
          title: 'V2Ray نمایندگان',
          url: '/reseller-v2ray',
          icon: IconShieldLock,
        },
      ],
    },
    {
      // Smart DNS (doctor-dns) -- architecturally parallel to the V2Ray
      // group above, but each account lives on exactly one registered DNS
      // panel instead of being fanned out across many.
      title: 'DNS',
      items: [
        {
          title: 'پنل‌های DNS',
          url: '/dns-panels',
          icon: IconServerBolt,
        },
        {
          title: 'پلن‌های DNS',
          url: '/dns-plans',
          icon: IconServerBolt,
        },
        {
          title: 'حساب‌های DNS',
          url: '/dns-accounts',
          icon: IconShieldLock,
        },
        {
          title: 'DNS نمایندگان',
          url: '/reseller-dns',
          icon: IconShieldLock,
        },
      ],
    },
    {
      title: 'اپلیکیشن‌ها',
      items: [
        {
          // Deliberate Persian label -- the admin explicitly required
          // this feature's entire UI to be 100% Persian.
          title: 'اپلیکیشن',
          url: '/applications',
          icon: IconDeviceMobile,
        },
        {
          // Mirrors "پلن‌های DNS" exactly -- see model.ApplicationPlan's
          // own doc comment (phase 2: plan selection + reseller quota,
          // same pattern already shipped for DNS).
          title: 'پلن‌های اپلیکیشن',
          url: '/application-plans',
          icon: IconDeviceMobile,
        },
        {
          title: 'موقعیت اپلیکیشن',
          url: '/application-locations',
          icon: IconWorld,
        },
        {
          // Same Persian-label exception -- publishing new mobile-app
          // releases, marking an update mandatory, and toggling global
          // maintenance mode. Deliberately admin-only, matching this
          // feature's own confirmed requirement ("فقط برای مدیر") --
          // lives only in adminSidebarData, never resellerSidebarData.
          title: 'مدیریت اپلیکیشن',
          url: '/application-management',
          icon: IconDeviceMobile,
        },
      ],
    },
    {
      title: 'مالی',
      items: [
        {
          title: 'کیف پول',
          url: '/billing/wallet',
          icon: IconWallet,
        },
        {
          title: 'فاکتورها',
          url: '/billing/invoices',
          icon: IconReceipt,
        },
        {
          title: 'بسته‌های ترافیک',
          url: '/billing/traffic-packages',
          icon: IconPackage,
        },
        {
          title: 'بسته‌های ترافیک User Manager',
          url: '/billing/user-manager-traffic-packages',
          icon: IconPackage,
        },
        {
          title: 'بسته‌های ترافیک V2Ray',
          url: '/billing/v2ray-traffic-packages',
          icon: IconPackage,
        },
        {
          // Same Persian-label exception as حسابداری/گزارش‌ها below --
          // the admin's own private profit/loss bookkeeping (partners,
          // costs, customer payments), deliberately admin-only: this
          // whole section lives only in adminSidebarData, never
          // resellerSidebarData.
          title: 'حسابداری',
          url: '/accounting',
          icon: IconCoin,
        },
      ],
    },
    {
      title: 'نظارت و گزارش‌ها',
      items: [
        {
          // Deliberate exception to this sidebar's otherwise-English
          // labels -- the Reports section itself is 100% Persian per its
          // own requirement, and that extends to naming its own sidebar
          // entry in Persian too.
          title: 'گزارش‌ها',
          url: '/reports',
          icon: IconChartBar,
        },
        {
          // Same Persian-label exception as Reports above -- the Security
          // page is 100% Persian per its own requirement.
          title: 'امنیت',
          url: '/security',
          icon: IconRadar,
        },
        {
          // Same Persian-label exception -- "tunnel-ai" phase-1's own
          // detection dashboard (discovery + health + alert history for
          // WireGuard tunnels).
          title: 'سلامت تانل‌ها',
          url: '/tunnel-health',
          icon: IconActivityHeartbeat,
        },
        {
          title: 'مشاهده لاگ‌ها',
          url: '/logs',
          icon: IconFileText,
        },
      ],
    },
    {
      title: 'ابزارهای مدیریتی',
      items: [
        {
          // Admin-only per-reseller enable/disable + billing period for
          // the Faoxima "bot-as-a-service" feature -- the admin's own
          // explicit request: running each reseller's Faoxima instance
          // costs the panel real resources (traffic + memory), so it
          // must not be free/unmanaged for every reseller by default.
          title: 'مدیریت ربات ایکس نمایندگان',
          url: '/faoxima-management',
          icon: IconBrandTelegram,
        },
      ],
    },
    {
      title: 'سایر',
      items: [
        {
          title: 'تنظیمات',
          icon: IconSettings,
          items: [
            {
              title: 'حساب کاربری',
              url: '/settings/account',
              icon: IconTool,
            },
            {
              title: 'ظاهر',
              url: '/settings/appearance',
              icon: IconPalette,
            },
          ],
        },
        {
          title: 'مرکز راهنمایی',
          url: '/help-center',
          icon: IconHelp,
        },
      ],
    },
  ],
}

// resellerSidebarData was reorganized the same way as adminSidebarData
// above -- see that constant's own doc comment for the confirmed,
// reported grouping bug this fixes. URLs are unchanged.
const resellerSidebarData: SidebarData = {
  navGroups: [
    {
      title: 'عمومی',
      items: [
        {
          title: 'داشبورد',
          url: '/',
          icon: IconLayoutDashboard,
        },
      ],
    },
    {
      title: 'سرویس‌ها',
      items: [
        {
          title: 'وایرگارد',
          url: '/peers',
          icon: IconUsers,
        },
        {
          title: 'User Manager',
          url: '/user-manager',
          icon: IconShieldLock,
        },
        {
          title: 'پکیج‌های V2Ray',
          url: '/v2ray-packages',
          icon: IconShieldLock,
        },
        {
          title: 'حساب‌های DNS',
          url: '/dns-accounts',
          icon: IconShieldLock,
        },
        {
          // Deliberate Persian label -- matches the admin sidebar's own
          // identical entry.
          title: 'اپلیکیشن',
          url: '/applications',
          icon: IconDeviceMobile,
        },
      ],
    },
    {
      title: 'مالی',
      items: [
        {
          title: 'فاکتورها',
          url: '/billing/invoices',
          icon: IconReceipt,
        },
        {
          title: 'خرید ترافیک',
          url: '/billing/traffic-packages',
          icon: IconPackage,
        },
        {
          title: 'خرید ترافیک User Manager',
          url: '/billing/user-manager-traffic-packages',
          icon: IconPackage,
        },
        {
          title: 'خرید ترافیک V2Ray',
          url: '/billing/v2ray-traffic-packages',
          icon: IconPackage,
        },
      ],
    },
    {
      title: 'سایر',
      items: [
        {
          title: 'تنظیمات',
          icon: IconSettings,
          items: [
            {
              title: 'حساب کاربری',
              url: '/settings/account',
              icon: IconTool,
            },
            {
              title: 'ظاهر',
              url: '/settings/appearance',
              icon: IconPalette,
            },
            {
              // Bot-as-a-service (item 6) -- Persian label, matches the
              // "اپلیکیشن" entry above's identical precedent for a
              // feature-specific Persian nav item inside an otherwise
              // English reseller sidebar.
              title: 'ربات فروش',
              url: '/settings/reseller-bot',
              icon: IconBrandTelegram,
            },
          ],
        },
        {
          title: 'مرکز راهنمایی',
          url: '/help-center',
          icon: IconHelp,
        },
      ],
    },
  ],
}

export const getSidebarData = (role?: string): SidebarData =>
  role === 'reseller' ? resellerSidebarData : adminSidebarData
