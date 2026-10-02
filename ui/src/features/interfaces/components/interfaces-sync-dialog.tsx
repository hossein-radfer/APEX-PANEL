import { useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'
import { useQueryClient } from '@tanstack/react-query'
import { CheckIcon } from 'lucide-react'
import { useInterfaces } from '@/features/interfaces/context/interfaces-context.tsx'
import { useSyncInterfacesQuery } from '@/hooks/interfaces/useSyncInterfacesQuery.ts'
import { syncSelectedInterfaces } from '@/api/interfaces.ts'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog.tsx'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table.tsx'

export function InterfacesSyncDialog() {
  const { open, setOpen } = useInterfaces()
  const queryClient = useQueryClient()
  const enabled = open === 'sync'
  const { data, isLoading } = useSyncInterfacesQuery(enabled)
  const [selected, setSelected] = useState<Record<string, boolean>>({})

  useEffect(() => {
    if (!enabled) return
    setSelected({})
  }, [enabled])

  const interfaceIds = useMemo(
    () => Object.entries(selected).filter(([, checked]) => checked).map(([id]) => id),
    [selected]
  )

  const handleSubmit = async () => {
    try {
      await syncSelectedInterfaces({ interface_ids: interfaceIds })
      await queryClient.invalidateQueries({ queryKey: ['interfaces_list'] })
      toast.success('اینترفیس‌های انتخاب‌شده با موفقیت همگام‌سازی شدند', { duration: 5000 })
      setOpen(null)
    } catch (error) {
      toast.error('همگام‌سازی اینترفیس‌های انتخاب‌شده با خطا مواجه شد')
      throw error
    }
  }

  return (
    <Dialog open={enabled} onOpenChange={(value) => !value && setOpen(null)}>
      <DialogContent className='max-w-3xl'>
        <DialogHeader>
          <DialogTitle>همگام‌سازی اینترفیس‌ها</DialogTitle>
          <DialogDescription>
            اینترفیس‌ها را از Mikrotik دریافت کنید، موارد مورد نظر را انتخاب کنید و فقط موارد تیک‌خورده اضافه می‌شوند.
          </DialogDescription>
        </DialogHeader>

        <div className='max-h-[60vh] overflow-auto rounded-md border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className='w-12'>
                  <CheckIcon className='h-4 w-4 opacity-0' />
                </TableHead>
                <TableHead>نام</TableHead>
                <TableHead>پورت شنود</TableHead>
                <TableHead>MTU</TableHead>
                <TableHead>وضعیت</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={5}>در حال بارگذاری اینترفیس‌ها...</TableCell>
                </TableRow>
              ) : data?.length ? (
                data.map((iface) => (
                  <TableRow key={iface.id}>
                    <TableCell>
                      <Checkbox
                        checked={selected[iface.id] ?? false}
                        onCheckedChange={(checked) =>
                          setSelected((prev) => ({
                            ...prev,
                            [iface.id]: Boolean(checked),
                          }))
                        }
                      />
                    </TableCell>
                    <TableCell>{iface.name}</TableCell>
                    <TableCell>{iface.listen_port}</TableCell>
                    <TableCell>{iface.mtu}</TableCell>
                    <TableCell>{iface.is_running ? 'در حال اجرا' : 'غیرفعال'}</TableCell>
                  </TableRow>
                ))
              ) : (
                <TableRow>
                  <TableCell colSpan={5}>هیچ اینترفیسی یافت نشد.</TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => setOpen(null)}>
            انصراف
          </Button>
          <Button onClick={handleSubmit} disabled={!interfaceIds.length}>
            همگام‌سازی موارد انتخاب‌شده
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

