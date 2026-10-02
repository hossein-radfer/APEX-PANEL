import { useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'
import { useQueryClient } from '@tanstack/react-query'
import { usePeers } from '@/features/peers/context/peers-context.tsx'
import { useSyncPeersQuery } from '@/hooks/peers/useSyncPeersQuery.ts'
import { syncSelectedPeers } from '@/api/peers.ts'
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

export function PeersSyncDialog() {
  const { open, setOpen } = usePeers()
  const queryClient = useQueryClient()
  const enabled = open === 'sync'
  const { data: interfaces = [] } = useInterfacesListQuery()
  const [selectedInterface, setSelectedInterface] = useState('all')
  const { data, isLoading } = useSyncPeersQuery(
    enabled,
    selectedInterface === 'all' ? undefined : selectedInterface
  )
  const [selected, setSelected] = useState<Record<string, boolean>>({})

  useEffect(() => {
    if (!enabled) return
    setSelected({})
  }, [enabled, selectedInterface])

  const peerIds = useMemo(
    () => Object.entries(selected).filter(([, checked]) => checked).map(([id]) => id),
    [selected]
  )

  const handleSubmit = async () => {
    try {
      await syncSelectedPeers({ peer_ids: peerIds })
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
                  <SelectItem key={iface.id} value={iface.name}>
                    {iface.name}
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
                  <TableHead>اینترفیس</TableHead>
                  <TableHead>آدرس مجاز</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isLoading ? (
                  <TableRow>
                    <TableCell colSpan={4}>در حال بارگذاری وایرگاردها...</TableCell>
                  </TableRow>
                ) : data?.length ? (
                  data.map((peer) => (
                    <TableRow key={peer.id}>
                      <TableCell>
                        <Checkbox
                          checked={selected[peer.id] ?? false}
                          onCheckedChange={(checked) =>
                            setSelected((prev) => ({
                              ...prev,
                              [peer.id]: Boolean(checked),
                            }))
                          }
                        />
                      </TableCell>
                      <TableCell>{peer.name}</TableCell>
                      <TableCell>{peer.interface}</TableCell>
                      <TableCell>{peer.allowed_address}</TableCell>
                    </TableRow>
                  ))
                ) : (
                  <TableRow>
                    <TableCell colSpan={4}>هیچ وایرگاردی برای این اینترفیس یافت نشد.</TableCell>
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
          <Button onClick={handleSubmit} disabled={!peerIds.length}>
            همگام‌سازی موارد انتخاب‌شده
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

