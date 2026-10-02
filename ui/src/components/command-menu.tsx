import React from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
  IconArrowRightDashed,
  IconChevronRight,
  IconDeviceLaptop,
  IconMoon,
  IconSun,
} from '@tabler/icons-react'
import { useSearch } from '@/context/search-context'
import { useTheme } from '@/context/theme-context'
import { useAuthStore } from '@/stores/authStore.ts'
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command'
import { getSidebarData } from './layout/data/sidebar-data'
import { ScrollArea } from './ui/scroll-area'

export function CommandMenu() {
  const navigate = useNavigate()
  const { setTheme } = useTheme()
  const { open, setOpen } = useSearch()
  const role = useAuthStore((state) => state.auth.admin?.role)
  const sidebarData = getSidebarData(role)

  const runCommand = React.useCallback(
    (command: () => unknown) => {
      setOpen(false)
      command()
    },
    [setOpen]
  )

  return (
    <CommandDialog modal open={open} onOpenChange={setOpen}>
      <CommandInput placeholder='یک دستور تایپ کنید یا جستجو کنید...' />
      <CommandList>
        <ScrollArea type='hover' className='h-72 pr-1'>
          <CommandEmpty>نتیجه‌ای یافت نشد.</CommandEmpty>
          {sidebarData.navGroups.map((group) => (
            <CommandGroup key={group.title} heading={group.title}>
              {group.items.map((navItem, i) => {
                if (navItem.url)
                  return (
                    <CommandItem
                      key={`${navItem.url}-${i}`}
                      value={navItem.title}
                      onSelect={() => {
                        runCommand(() => navigate({ to: navItem.url }))
                      }}
                    >
                      <div className='mr-2 flex h-4 w-4 items-center justify-center'>
                        <IconArrowRightDashed className='text-muted-foreground/80 size-2' />
                      </div>
                      {navItem.title}
                    </CommandItem>
                  )

                return navItem.items?.map((subItem, i) => (
                  <CommandItem
                    key={`${navItem.title}-${subItem.url}-${i}`}
                    value={`${navItem.title}-${subItem.url}`}
                    onSelect={() => {
                      runCommand(() => navigate({ to: subItem.url }))
                    }}
                  >
                    <div className='mr-2 flex h-4 w-4 items-center justify-center'>
                      <IconArrowRightDashed className='text-muted-foreground/80 size-2' />
                    </div>
                    {navItem.title} <IconChevronRight /> {subItem.title}
                  </CommandItem>
                ))
              })}
            </CommandGroup>
          ))}
          <CommandSeparator />
          <CommandGroup heading='پوسته'>
            <CommandItem onSelect={() => runCommand(() => setTheme('light'))}>
              <IconSun /> <span>روشن</span>
            </CommandItem>
            <CommandItem onSelect={() => runCommand(() => setTheme('dark'))}>
              <IconMoon className='scale-90' />
              <span>تیره</span>
            </CommandItem>
            <CommandItem onSelect={() => runCommand(() => setTheme('system'))}>
              <IconDeviceLaptop />
              <span>سیستم</span>
            </CommandItem>
          </CommandGroup>
        </ScrollArea>
      </CommandList>
    </CommandDialog>
  )
}
