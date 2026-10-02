import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { IconTrash, IconUserPlus } from '@tabler/icons-react'
import { toast } from 'sonner'
import {
  createPartner,
  deletePartner,
  fetchPartners,
  updatePartner,
} from '@/api/accounting.ts'
import { AccountingPartner } from '@/schema/accounting.ts'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { EmptyState } from '@/components/ui/empty-state.tsx'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

function backendErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof AxiosError) {
    const message = err.response?.data?.message
    if (typeof message === 'string' && message.length > 0) return message
  }
  return fallback
}

export function PartnersTab() {
  const queryClient = useQueryClient()
  const { data: partners = [], isLoading } = useQuery({
    queryKey: ['accounting_partners'],
    queryFn: fetchPartners,
  })

  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<AccountingPartner | null>(null)
  const [name, setName] = useState('')
  const [comment, setComment] = useState('')

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ['accounting_partners'] })

  const createMutation = useMutation({
    mutationFn: createPartner,
    onSuccess: () => {
      toast.success('همکار جدید ثبت شد.')
      invalidate()
      setOpen(false)
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'ثبت همکار ناموفق بود.')),
  })

  const updateMutation = useMutation({
    mutationFn: updatePartner,
    onSuccess: () => {
      toast.success('همکار به‌روزرسانی شد.')
      invalidate()
      setOpen(false)
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'به‌روزرسانی ناموفق بود.')),
  })

  const deleteMutation = useMutation({
    mutationFn: deletePartner,
    onSuccess: () => {
      toast.success('همکار حذف شد.')
      invalidate()
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'حذف ناموفق بود.')),
  })

  const openCreate = () => {
    setEditing(null)
    setName('')
    setComment('')
    setOpen(true)
  }

  const openEdit = (partner: AccountingPartner) => {
    setEditing(partner)
    setName(partner.name)
    setComment(partner.comment ?? '')
    setOpen(true)
  }

  const handleSubmit = () => {
    if (!name.trim()) {
      toast.error('نام همکار الزامی است.')
      return
    }
    if (editing) {
      updateMutation.mutate({ id: editing.id, name, comment: comment || null })
    } else {
      createMutation.mutate({ name, comment: comment || null })
    }
  }

  return (
    <Card>
      <CardHeader className='flex flex-row items-center justify-between'>
        <CardTitle>همکاران / تأمین‌کنندگان</CardTitle>
        <Button onClick={openCreate} className='gap-2'>
          <IconUserPlus className='h-4 w-4' />
          افزودن همکار
        </Button>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className='h-40 w-full rounded-lg' />
        ) : partners.length === 0 ? (
          <EmptyState message='هنوز هیچ همکاری ثبت نشده است.' />
        ) : (
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>نام</TableHead>
                  <TableHead>توضیحات</TableHead>
                  <TableHead className='w-24'></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {partners.map((partner) => (
                  <TableRow
                    key={partner.id}
                    className='cursor-pointer'
                    onClick={() => openEdit(partner)}
                  >
                    <TableCell className='font-medium'>{partner.name}</TableCell>
                    <TableCell>{partner.comment || '—'}</TableCell>
                    <TableCell onClick={(e) => e.stopPropagation()}>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='text-destructive'
                        onClick={() => deleteMutation.mutate(partner.id)}
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

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>{editing ? 'ویرایش همکار' : 'افزودن همکار'}</DialogTitle>
          </DialogHeader>
          <div className='space-y-4'>
            <div className='space-y-2'>
              <Label>نام</Label>
              <Input value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className='space-y-2'>
              <Label>توضیحات (اختیاری)</Label>
              <Input value={comment} onChange={(e) => setComment(e.target.value)} />
            </div>
          </div>
          <DialogFooter>
            <Button
              onClick={handleSubmit}
              disabled={createMutation.isPending || updateMutation.isPending}
            >
              ذخیره
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  )
}
