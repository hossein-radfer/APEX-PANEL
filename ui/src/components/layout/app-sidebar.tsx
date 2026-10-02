import { IconHeartFilled, IconRoute } from '@tabler/icons-react'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuItem,
  SidebarRail,
  useSidebar,
} from '@/components/ui/sidebar'
import { NavGroup } from '@/components/layout/nav-group'
import React from 'react'
import { useAuthStore } from '@/stores/authStore'
import { getSidebarData } from './data/sidebar-data'

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const { open } = useSidebar()
  const role = useAuthStore((state) => state.auth.admin?.role)
  const sidebarData = getSidebarData(role)

  return (
    <Sidebar collapsible='icon' variant='floating' {...props}>
      {open && (
        <SidebarHeader className='flex-row items-center justify-center pt-5'>
          <IconRoute />
          <h1 className='text-2xl font-bold tracking-tight'>ApexPanel</h1>
        </SidebarHeader>
      )}

      <SidebarContent>
        {sidebarData.navGroups.map((props) => (
          <NavGroup key={props.title} {...props} />
        ))}
      </SidebarContent>

      {open && (
        <SidebarFooter className='flex items-center justify-center p-4 text-white/80 shadow-inner'>
          <SidebarMenu>
            <SidebarMenuItem>
              <div className='flex items-center justify-center gap-1 text-sm text-xs font-normal'>
                <span>Made with</span>
                <IconHeartFilled size='13' />
                <span>by Hossein</span>
              </div>
            </SidebarMenuItem>
            <SidebarMenuItem>
              <div className='text-xs text-white/70 text-center w-full'>v{__APP_VERSION__}</div>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarFooter>
      )}

      <SidebarRail />
    </Sidebar>
  )
}
