import { Application } from '@/schema/application.ts'
import { CheckIcon, XIcon } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'

type Props = {
  open: boolean
  onOpenChange: (state: boolean) => void
  currentRow: Application
}

function ResourceList({
  title,
  resources,
}: {
  title: string
  resources: Application['wireguard_peers']
}) {
  if (resources.length === 0) return null
  return (
    <div className='space-y-2'>
      <p className='text-sm font-medium'>{title}</p>
      <div className='space-y-1'>
        {resources.map((r, i) => (
          <div
            key={i}
            className='flex items-center justify-between rounded-md border px-3 py-2 text-sm'
          >
            <div className='flex flex-col'>
              <span>
                {r.label ? `${r.label} (${r.resource_name})` : r.resource_name}
              </span>
              {r.profile_name && (
                <span className='text-muted-foreground text-xs'>
                  پروفایل: {r.profile_name}
                </span>
              )}
            </div>
            {r.enabled ? (
              <Badge className='gap-1 bg-green-500 text-white hover:bg-green-500'>
                <CheckIcon className='h-3 w-3' />
                فعال
              </Badge>
            ) : (
              <Badge variant='secondary' className='gap-1'>
                <XIcon className='h-3 w-3' />
                غیرفعال
              </Badge>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

export function ApplicationDetailsDialog({
  open,
  onOpenChange,
  currentRow,
}: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[85vh] overflow-y-auto sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>جزئیات اپلیکیشن «{currentRow.name}»</DialogTitle>
        </DialogHeader>

        <div className='space-y-4'>
          <ResourceList
            title='وایرگارد'
            resources={currentRow.wireguard_peers}
          />
          <ResourceList
            title='اکانت‌های یوزرمنجیر'
            resources={currentRow.user_manager_accounts}
          />
          <ResourceList
            title='پکیج‌های V2Ray'
            resources={currentRow.v2ray_packages}
          />
          {currentRow.wireguard_peers.length === 0 &&
            currentRow.user_manager_accounts.length === 0 &&
            currentRow.v2ray_packages.length === 0 && (
              <p className='text-muted-foreground py-4 text-center text-sm'>
                هیچ منبعی برای این اپلیکیشن ساخته نشده است.
              </p>
            )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
