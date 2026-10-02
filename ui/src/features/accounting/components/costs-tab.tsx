import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { IconPlus, IconRefresh, IconTrash } from '@tabler/icons-react'
import { toast } from 'sonner'
import {
  advanceRecurringCost,
  createCost,
  deleteCost,
  fetchCosts,
  updateCost,
} from '@/api/accounting.ts'
import { fetchPartners } from '@/api/accounting.ts'
import { fetchServersList } from '@/api/servers.ts'
import { AccountingCost } from '@/schema/accounting.ts'
import { formatCurrencyFa } from '@/features/reports/lib/format.ts'
import { Badge } from '@/components/ui/badge'
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

function backendErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof AxiosError) {
    const message = err.response?.data?.message
    if (typeof message === 'string' && message.length > 0) return message
  }
  return fallback
}

const PROTOCOL_LABELS_FA: Record<string, string> = {
  wireguard: 'وایرگارد',
  user_manager: 'یوزرمنجیر',
  v2ray: 'V2Ray',
}

const NONE = '__none__'

export function CostsTab() {
  const queryClient = useQueryClient()
  const { data: costs = [], isLoading } = useQuery({
    queryKey: ['accounting_costs'],
    queryFn: fetchCosts,
  })
  const { data: partners = [] } = useQuery({
    queryKey: ['accounting_partners'],
    queryFn: fetchPartners,
  })
  const { data: servers = [] } = useQuery({
    queryKey: ['servers_list'],
    queryFn: fetchServersList,
  })

  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<AccountingCost | null>(null)
  const [partnerId, setPartnerId] = useState<string>('')
  const [serverId, setServerId] = useState<string>(NONE)
  const [protocol, setProtocol] = useState<string>(NONE)
  const [locationKey, setLocationKey] = useState('')
  const [amount, setAmount] = useState<number | string>('')
  const [description, setDescription] = useState('')
  const [paidAt, setPaidAt] = useState('')
  const [dueAt, setDueAt] = useState('')
  const [recurrenceDays, setRecurrenceDays] = useState<number | string>('')

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ['accounting_costs'] })

  const createMutation = useMutation({
    mutationFn: createCost,
    onSuccess: () => {
      toast.success('هزینه ثبت شد.')
      invalidate()
      setOpen(false)
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'ثبت هزینه ناموفق بود.')),
  })
  const updateMutation = useMutation({
    mutationFn: updateCost,
    onSuccess: () => {
      toast.success('هزینه به‌روزرسانی شد.')
      invalidate()
      setOpen(false)
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'به‌روزرسانی ناموفق بود.')),
  })
  const deleteMutation = useMutation({
    mutationFn: deleteCost,
    onSuccess: () => {
      toast.success('هزینه حذف شد.')
      invalidate()
    },
  })
  const advanceMutation = useMutation({
    mutationFn: advanceRecurringCost,
    onSuccess: () => {
      toast.success('تاریخ سررسید بعدی ثبت شد.')
      invalidate()
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'عملیات ناموفق بود.')),
  })

  const resetForm = () => {
    setEditing(null)
    setPartnerId('')
    setServerId(NONE)
    setProtocol(NONE)
    setLocationKey('')
    setAmount('')
    setDescription('')
    setPaidAt('')
    setDueAt('')
    setRecurrenceDays('')
  }

  const openCreate = () => {
    resetForm()
    setOpen(true)
  }

  const openEdit = (cost: AccountingCost) => {
    setEditing(cost)
    setPartnerId(String(cost.partner_id))
    setServerId(cost.server_id ? String(cost.server_id) : NONE)
    setProtocol(cost.protocol ?? NONE)
    setLocationKey(cost.location_key ?? '')
    setAmount(cost.amount_toman)
    setDescription(cost.description ?? '')
    setPaidAt(cost.paid_at ?? '')
    setDueAt(cost.due_at ?? '')
    setRecurrenceDays(cost.recurrence_interval_days ?? '')
    setOpen(true)
  }

  const handleSubmit = () => {
    const parsedAmount = Number(amount)
    if (!partnerId) {
      toast.error('یک همکار انتخاب کنید.')
      return
    }
    if (!Number.isFinite(parsedAmount) || parsedAmount <= 0) {
      toast.error('مبلغ باید عددی مثبت باشد.')
      return
    }

    const req = {
      partner_id: Number(partnerId),
      server_id: serverId === NONE ? null : Number(serverId),
      protocol: protocol === NONE ? null : protocol,
      location_key: locationKey.trim() || null,
      amount_toman: parsedAmount,
      description: description || null,
      paid_at: paidAt || null,
      due_at: dueAt || null,
      recurrence_interval_days: recurrenceDays ? Number(recurrenceDays) : null,
    }

    if (editing) {
      updateMutation.mutate({ id: editing.id, ...req })
    } else {
      createMutation.mutate(req)
    }
  }

  return (
    <Card>
      <CardHeader className='flex flex-row items-center justify-between'>
        <CardTitle>هزینه‌ها</CardTitle>
        <Button onClick={openCreate} className='gap-2'>
          <IconPlus className='h-4 w-4' />
          ثبت هزینه
        </Button>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className='h-40 w-full rounded-lg' />
        ) : costs.length === 0 ? (
          <EmptyState message='هنوز هیچ هزینه‌ای ثبت نشده است.' />
        ) : (
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>همکار</TableHead>
                  <TableHead>سرور</TableHead>
                  <TableHead>پروتکل/لوکیشن</TableHead>
                  <TableHead>مبلغ</TableHead>
                  <TableHead>تاریخ پرداخت</TableHead>
                  <TableHead>سررسید بعدی</TableHead>
                  <TableHead className='w-32'></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {costs.map((cost) => (
                  <TableRow key={cost.id}>
                    <TableCell
                      className='cursor-pointer font-medium'
                      onClick={() => openEdit(cost)}
                    >
                      {cost.partner_name}
                    </TableCell>
                    <TableCell>{cost.server_name || '—'}</TableCell>
                    <TableCell>
                      {cost.protocol ? (
                        <div className='flex flex-col gap-1'>
                          <Badge variant='secondary'>
                            {PROTOCOL_LABELS_FA[cost.protocol] ?? cost.protocol}
                          </Badge>
                          {(cost.location_label || cost.location_key) && (
                            <span className='text-muted-foreground text-xs'>
                              {cost.location_label || cost.location_key}
                            </span>
                          )}
                        </div>
                      ) : (
                        '—'
                      )}
                    </TableCell>
                    <TableCell className='tabular-nums'>
                      {formatCurrencyFa(cost.amount_toman)}
                    </TableCell>
                    <TableCell>{cost.paid_at || '—'}</TableCell>
                    <TableCell>
                      {cost.next_due_at ? (
                        <Badge variant='outline'>{cost.next_due_at}</Badge>
                      ) : (
                        '—'
                      )}
                    </TableCell>
                    <TableCell className='flex gap-1'>
                      {cost.recurrence_interval_days && (
                        <Button
                          variant='ghost'
                          size='icon'
                          title='ثبت پرداخت و تعیین سررسید بعدی'
                          onClick={() => advanceMutation.mutate(cost.id)}
                        >
                          <IconRefresh className='h-4 w-4' />
                        </Button>
                      )}
                      <Button
                        variant='ghost'
                        size='icon'
                        className='text-destructive'
                        onClick={() => deleteMutation.mutate(cost.id)}
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
        <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-lg'>
          <DialogHeader>
            <DialogTitle>{editing ? 'ویرایش هزینه' : 'ثبت هزینه'}</DialogTitle>
          </DialogHeader>
          <div className='space-y-4'>
            <div className='space-y-2'>
              <Label>همکار</Label>
              <Select value={partnerId} onValueChange={setPartnerId}>
                <SelectTrigger className='w-full'>
                  <SelectValue placeholder='انتخاب همکار' />
                </SelectTrigger>
                <SelectContent>
                  {partners.map((p) => (
                    <SelectItem key={p.id} value={String(p.id)}>
                      {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className='grid grid-cols-2 gap-3'>
              <div className='space-y-2'>
                <Label>سرور (اختیاری)</Label>
                <Select value={serverId} onValueChange={setServerId}>
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={NONE}>بدون سرور</SelectItem>
                    {servers.map((s) => (
                      <SelectItem key={s.id} value={String(s.id)}>
                        {s.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className='space-y-2'>
                <Label>پروتکل (اختیاری)</Label>
                <Select value={protocol} onValueChange={setProtocol}>
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={NONE}>بدون پروتکل</SelectItem>
                    <SelectItem value='wireguard'>وایرگارد</SelectItem>
                    <SelectItem value='user_manager'>یوزرمنجیر</SelectItem>
                    <SelectItem value='v2ray'>V2Ray</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            {protocol !== NONE && (
              <div className='space-y-2'>
                <Label>
                  کلید لوکیشن (اختیاری -- برای گروه‌بندی سود هر لوکیشن)
                </Label>
                <Input
                  placeholder={
                    protocol === 'wireguard'
                      ? 'شناسه عددی اینترفیس (مثلاً 5)'
                      : protocol === 'user_manager'
                        ? 'نام گروه (مثلاً default)'
                        : 'شناسه عددی پنل (مثلاً 2)'
                  }
                  value={locationKey}
                  onChange={(e) => setLocationKey(e.target.value)}
                />
                <p className='text-muted-foreground text-xs'>
                  باید دقیقاً با کلید لوکیشنِ فروش‌های همان بخش یکی باشد تا
                  در گزارش سود هر لوکیشن با هم جمع بسته شوند.
                </p>
              </div>
            )}

            <div className='space-y-2'>
              <Label>مبلغ (تومان)</Label>
              <Input
                type='number'
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
              />
            </div>

            <div className='space-y-2'>
              <Label>توضیحات (اختیاری)</Label>
              <Input value={description} onChange={(e) => setDescription(e.target.value)} />
            </div>

            <div className='grid grid-cols-2 gap-3'>
              <div className='space-y-2'>
                <Label>تاریخ پرداخت (YYYY-MM-DD)</Label>
                <Input
                  placeholder='2026-08-30'
                  value={paidAt}
                  onChange={(e) => setPaidAt(e.target.value)}
                />
              </div>
              <div className='space-y-2'>
                <Label>تاریخ سررسید (YYYY-MM-DD)</Label>
                <Input
                  placeholder='2026-09-30'
                  value={dueAt}
                  onChange={(e) => setDueAt(e.target.value)}
                />
              </div>
            </div>

            <div className='space-y-2'>
              <Label>بازه تکرار (روز، اختیاری -- برای هزینه‌های دوره‌ای مثل تمدید سرور)</Label>
              <Input
                type='number'
                placeholder='مثلاً 30'
                value={recurrenceDays}
                onChange={(e) => setRecurrenceDays(e.target.value)}
              />
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
