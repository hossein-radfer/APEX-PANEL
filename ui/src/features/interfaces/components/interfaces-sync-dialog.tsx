import { useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'
import { useQueryClient } from '@tanstack/react-query'
import { CheckIcon } from 'lucide-react'
import { useInterfaces } from '@/features/interfaces/context/interfaces-context.tsx'
import { useSyncInterfacesQuery } from '@/hooks/interfaces/useSyncInterfacesQuery.ts'
import { syncSelectedInterfaces } from '@/api/interfaces.ts'
import { ServerBadge } from '@/features/shared-components/server-badge.tsx'
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

// Row keys are "<server_id>:<id>" rather than the bare RouterOS ".id" --
// that id is only unique within one router, so two servers' interfaces can
// share the same ".id" and a bare-id keyed checkbox map would conflate them
// (checking one would visually check both).
const rowKey = (serverId: number, id: string) => `${serverId}:${id}`

export function InterfacesSyncDialog() {
  const { open, setOpen } = useInterfaces()
  const queryClient = useQueryClient()
  const enabled = open === 'sync'
  const { data, isLoading } = useSyncInterfacesQuery(enabled)
  const showServerBadge = useMemo(
    () => new Set((data ?? []).map((iface) => iface.server_id)).size > 1,
    [data]
  )
  const [selected, setSelected] = useState<Record<string, boolean>>({})

  useEffect(() => {
    if (!enabled) return
    setSelected({})
  }, [enabled])

  const selectedRows = useMemo(
    () => Object.entries(selected).filter(([, checked]) => checked).map(([key]) => key),
    [selected]
  )

  const handleSubmit = async () => {
    try {
      // A selection can span multiple servers (the preview list merges all
      // of them), but each sync call is pinned to exactly one server -- see
      // SyncSelectedInterfacesForServer. Group the checked rows back by
      // server and fire one call per server.
      const idsByServer = new Map<number, string[]>()
      for (const key of selectedRows) {
        const [serverIdStr, ...rest] = key.split(':')
        const serverId = Number(serverIdStr)
        const id = rest.join(':')
        idsByServer.set(serverId, [...(idsByServer.get(serverId) ?? []), id])
      }

      await Promise.all(
        Array.from(idsByServer.entries()).map(([serverId, interfaceIds]) =>
          syncSelectedInterfaces({ server_id: serverId, interface_ids: interfaceIds })
        )
      )
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
                {showServerBadge && <TableHead>سرور</TableHead>}
                <TableHead>پورت شنود</TableHead>
                <TableHead>MTU</TableHead>
                <TableHead>وضعیت</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={showServerBadge ? 6 : 5}>در حال بارگذاری اینترفیس‌ها...</TableCell>
                </TableRow>
              ) : data?.length ? (
                data.map((iface) => {
                  const key = rowKey(iface.server_id, iface.id)
                  return (
                    <TableRow key={key}>
                      <TableCell>
                        <Checkbox
                          checked={selected[key] ?? false}
                          onCheckedChange={(checked) =>
                            setSelected((prev) => ({
                              ...prev,
                              [key]: Boolean(checked),
                            }))
                          }
                        />
                      </TableCell>
                      <TableCell>{iface.name}</TableCell>
                      {showServerBadge && (
                        <TableCell>
                          <ServerBadge serverId={iface.server_id} serverName={iface.server_name} />
                        </TableCell>
                      )}
                      <TableCell>{iface.listen_port}</TableCell>
                      <TableCell>{iface.mtu}</TableCell>
                      <TableCell>{iface.is_running ? 'در حال اجرا' : 'غیرفعال'}</TableCell>
                    </TableRow>
                  )
                })
              ) : (
                <TableRow>
                  <TableCell colSpan={showServerBadge ? 6 : 5}>هیچ اینترفیسی یافت نشد.</TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => setOpen(null)}>
            انصراف
          </Button>
          <Button onClick={handleSubmit} disabled={!selectedRows.length}>
            همگام‌سازی موارد انتخاب‌شده
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

