import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  addRedlineEntry,
  fetchRedlineEntries,
  removeRedlineEntry,
} from '@/api/tunnel-health.ts'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { IconPlus, IconTrash } from '@tabler/icons-react'

const KIND_LABELS_FA: Record<string, string> = {
  interface: 'اینترفیس',
  ip: 'آی‌پی',
  port: 'پورت',
}

// RedlineTab is spec section ب-6's "منطقه امن مدیریتی" -- interfaces/
// IPs/ports the admin uses for their OWN management connection. This
// list is checked independently by TunnelRedlineValidator on the
// backend before any mutating command is even constructed, so nothing
// entered here can ever be toggled/touched by the self-healing engine
// regardless of dry-run state.
export function RedlineTab() {
  const queryClient = useQueryClient()
  const { data: entries = [], isLoading } = useQuery({
    queryKey: ['tunnel_ai_redline'],
    queryFn: () => fetchRedlineEntries(),
  })

  const [kind, setKind] = useState<'interface' | 'ip' | 'port'>('interface')
  const [value, setValue] = useState('')
  const [comment, setComment] = useState('')

  const addMutation = useMutation({
    mutationFn: addRedlineEntry,
    onSuccess: () => {
      toast.success('مورد جدید به منطقه امن اضافه شد.')
      queryClient.invalidateQueries({ queryKey: ['tunnel_ai_redline'] })
      setValue('')
      setComment('')
    },
    onError: () => toast.error('افزودن ناموفق بود.'),
  })

  const removeMutation = useMutation({
    mutationFn: removeRedlineEntry,
    onSuccess: () => {
      toast.success('مورد حذف شد.')
      queryClient.invalidateQueries({ queryKey: ['tunnel_ai_redline'] })
    },
    onError: () => toast.error('حذف ناموفق بود.'),
  })

  const handleAdd = () => {
    if (!value.trim()) {
      toast.error('مقدار را وارد کنید.')
      return
    }
    addMutation.mutate({ kind, value: value.trim(), comment: comment.trim() || undefined })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>منطقه امن مدیریتی</CardTitle>
      </CardHeader>
      <CardContent className='space-y-4'>
        <p className='text-muted-foreground text-sm'>
          اینترفیس/آی‌پی/پورتی که خودتان برای مدیریت به سرور وصل می‌شوید --
          هرگز توسط موتور تصمیم‌گیری تغییر داده نمی‌شود، حتی در حالت اجرای
          واقعی.
        </p>

        <div className='flex flex-wrap items-end gap-2'>
          <Select value={kind} onValueChange={(v) => setKind(v as typeof kind)}>
            <SelectTrigger className='w-32'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='interface'>اینترفیس</SelectItem>
              <SelectItem value='ip'>آی‌پی</SelectItem>
              <SelectItem value='port'>پورت</SelectItem>
            </SelectContent>
          </Select>
          <Input
            placeholder='مقدار (مثلاً ether1 یا 192.168.1.1)'
            value={value}
            onChange={(e) => setValue(e.target.value)}
            className='w-64'
          />
          <Input
            placeholder='توضیح (اختیاری)'
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            className='w-64'
          />
          <Button onClick={handleAdd} disabled={addMutation.isPending}>
            <IconPlus className='ml-1 h-4 w-4' />
            افزودن
          </Button>
        </div>

        {isLoading ? (
          <Skeleton className='h-32 w-full rounded-lg' />
        ) : entries.length === 0 ? (
          <p className='text-muted-foreground py-8 text-center text-sm'>
            هنوز هیچ موردی در منطقه امن مدیریتی ثبت نشده است.
          </p>
        ) : (
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>نوع</TableHead>
                  <TableHead>مقدار</TableHead>
                  <TableHead>توضیح</TableHead>
                  <TableHead className='w-12' />
                </TableRow>
              </TableHeader>
              <TableBody>
                {entries.map((entry) => (
                  <TableRow key={entry.id}>
                    <TableCell>
                      {KIND_LABELS_FA[entry.kind] ?? entry.kind}
                    </TableCell>
                    <TableCell className='font-mono text-sm'>
                      {entry.value}
                    </TableCell>
                    <TableCell className='text-muted-foreground text-sm'>
                      {entry.comment || '—'}
                    </TableCell>
                    <TableCell>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='text-muted-foreground hover:text-destructive h-8 w-8'
                        onClick={() => removeMutation.mutate(entry.id)}
                      >
                        <IconTrash className='h-4 w-4' />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
