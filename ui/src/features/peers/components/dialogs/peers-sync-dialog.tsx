import { useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'
import { useQueryClient } from '@tanstack/react-query'
import { usePeers } from '@/features/peers/context/peers-context.tsx'
import { useSyncPeersQuery } from '@/hooks/peers/useSyncPeersQuery.ts'
import { syncSelectedPeers } from '@/api/peers.ts'
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table.tsx'
import { useInterfacesListQuery } from '@/hooks/interfaces/useInterfacesListQuery.ts'

// Row keys are "<server_id>:<id>" rather than the bare RouterOS ".id" --
// that id is only unique within one router, so two servers' peers can share
// the same ".id" and a bare-id keyed checkbox map would conflate them.
const rowKey = (serverId: number, id: string) => `${serverId}:${id}`

// Interface filter options are "<server_id>:<name>" -- an interface name
// like "wg0" is only unique within one server, so filtering by bare name
// across a merged, all-servers preview list would match the wrong server's
// interface too.
const interfaceFilterValue = (serverId: number, name: string) => `${serverId}:${name}`

export function PeersSyncDialog() {
  const { open, setOpen } = usePeers()
  const queryClient = useQueryClient()
  const enabled = open === 'sync'
  const { data: interfaces = [] } = useInterfacesListQuery()
  const [selectedInterface, setSelectedInterface] = useState('all')
  // The server-side `interface` query filter isn't server-aware (matches by
  // bare name against every server), so fetch everything and filter
  // client-side by the (server_id, name) pair instead.
  const { data: allPeers, isLoading } = useSyncPeersQuery(enabled)
  const data = useMemo(() => {
    if (!allPeers) return allPeers
    if (selectedInterface === 'all') return allPeers
    const [serverIdStr, ...rest] = selectedInterface.split(':')
    const serverId = Number(serverIdStr)
    const name = rest.join(':')
    return allPeers.filter((peer) => peer.server_id === serverId && peer.interface === name)
  }, [allPeers, selectedInterface])
  const showServerBadge = useMemo(
    () => new Set((allPeers ?? []).map((peer) => peer.server_id)).size > 1,
    [allPeers]
  )
  const [selected, setSelected] = useState<Record<string, boolean>>({})

  useEffect(() => {
    if (!enabled) return
    setSelected({})
  }, [enabled, selectedInterface])

  const selectedRows = useMemo(
    () => Object.entries(selected).filter(([, checked]) => checked).map(([key]) => key),
    [selected]
  )

  const handleSubmit = async () => {
    try {
      // A selection can span multiple servers (the preview list merges all
      // of them), but each sync call is pinned to exactly one server -- see
      // SyncSelectedPeersForServer. Group the checked rows back by server
      // and fire one call per server.
      const idsByServer = new Map<number, string[]>()
      for (const key of selectedRows) {
        const [serverIdStr, ...rest] = key.split(':')
        const serverId = Number(serverIdStr)
        const id = rest.join(':')
        idsByServer.set(serverId, [...(idsByServer.get(serverId) ?? []), id])
      }

      await Promise.all(
        Array.from(idsByServer.entries()).map(([serverId, peerIds]) =>
          syncSelectedPeers({ server_id: serverId, peer_ids: peerIds })
        )
      )
      await queryClient.invalidateQueries({ queryKey: ['peers_list'] })
      toast.success('وایرگاردهای انتخاب‌شده با موفقیت همگام‌سازی شدند', { duration: 5000 })
      setOpen(null)
    } catch (error) {
      toast.error('همگام‌سازی وایرگاردهای انتخاب‌شده ناموفق بود')
      throw error
    }
  }

  return (
    <Dialog open={enabled} onOpenChange={(value) => !value && setOpen(null)}>
      <DialogContent className='max-w-4xl'>
        <DialogHeader>
          <DialogTitle>همگام‌سازی وایرگاردها</DialogTitle>
          <DialogDescription>
            بر اساس اینترفیس فیلتر کنید، سپس فقط وایرگاردهایی که می‌خواهید همگام‌سازی شوند را انتخاب کنید.
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4'>
          <div className='max-w-sm'>
            <Select value={selectedInterface} onValueChange={setSelectedInterface}>
              <SelectTrigger className='w-full'>
                <SelectValue placeholder='فیلتر بر اساس اینترفیس' />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='all'>همه‌ی اینترفیس‌ها</SelectItem>
                {interfaces.map((iface) => (
                  <SelectItem
                    key={iface.id}
                    value={interfaceFilterValue(iface.server_id, iface.name)}
                  >
                    {showServerBadge ? `${iface.name} (${iface.server_name})` : iface.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className='max-h-[60vh] overflow-auto rounded-md border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='w-12' />
                  <TableHead>نام</TableHead>
                  {showServerBadge && <TableHead>سرور</TableHead>}
                  <TableHead>اینترفیس</TableHead>
                  <TableHead>آدرس مجاز</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isLoading ? (
                  <TableRow>
                    <TableCell colSpan={showServerBadge ? 5 : 4}>در حال بارگذاری وایرگاردها...</TableCell>
                  </TableRow>
                ) : data?.length ? (
                  data.map((peer) => {
                    const key = rowKey(peer.server_id, peer.id)
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
                        <TableCell>{peer.name}</TableCell>
                        {showServerBadge && (
                          <TableCell>
                            <ServerBadge serverId={peer.server_id} serverName={peer.server_name} />
                          </TableCell>
                        )}
                        <TableCell>{peer.interface}</TableCell>
                        <TableCell>{peer.allowed_address}</TableCell>
                      </TableRow>
                    )
                  })
                ) : (
                  <TableRow>
                    <TableCell colSpan={showServerBadge ? 5 : 4}>هیچ وایرگاردی برای این اینترفیس یافت نشد.</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
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

