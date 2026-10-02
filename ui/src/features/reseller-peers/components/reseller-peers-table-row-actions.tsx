import { DotsHorizontalIcon } from '@radix-ui/react-icons'
import { Row } from '@tanstack/react-table'
import {
  IconDownload,
  IconEdit,
  IconFileCode,
  IconQrcode,
  IconShare,
  IconTrash,
} from '@tabler/icons-react'
import { Peer } from '@/schema/peers.ts'
import { Button } from '@/components/ui/button.tsx'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu.tsx'
import { useResellerPeers } from '@/features/reseller-peers/context/reseller-peers-context.tsx'

interface Props {
  row: Row<Peer>
}

export function ResellerPeersTableRowActions({ row }: Props) {
  const { setOpen, setCurrentRow } = useResellerPeers()
  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <Button
          variant='ghost'
          className='data-[state=open]:bg-muted flex h-8 w-8 p-0'
        >
          <DotsHorizontalIcon className='h-4 w-4' />
          <span className='sr-only'>باز کردن منو</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        <DropdownMenuItem
          onClick={() => {
            setCurrentRow(row.original)
            setOpen('share')
          }}
        >
          اشتراک‌گذاری
          <DropdownMenuShortcut>
            <IconShare size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => {
            setCurrentRow(row.original)
            setOpen('qrCode')
          }}
        >
          نمایش QR Code
          <DropdownMenuShortcut>
            <IconQrcode size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => {
            setCurrentRow(row.original)
            setOpen('show_config')
          }}
        >
          نمایش کانفیگ
          <DropdownMenuShortcut className='pl-5'>
            <IconFileCode size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => {
            setCurrentRow(row.original)
            setOpen('download_config')
          }}
        >
          دانلود کانفیگ
          <DropdownMenuShortcut className='pl-5'>
            <IconDownload size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => {
            setCurrentRow(row.original)
            setOpen('edit')
          }}
        >
          ویرایش وایرگارد
          <DropdownMenuShortcut>
            <IconEdit size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          onClick={() => {
            setCurrentRow(row.original)
            setOpen('delete')
          }}
          className='text-red-500!'
        >
          حذف وایرگارد
          <DropdownMenuShortcut>
            <IconTrash size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
