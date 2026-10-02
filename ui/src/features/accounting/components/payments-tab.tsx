import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { IconPaperclip, IconPlus, IconTrash } from '@tabler/icons-react'
import { toast } from 'sonner'
import {
  createPayment,
  deletePayment,
  fetchPayments,
  fetchSellableResources,
  updatePayment,
  uploadReceipt,
} from '@/api/accounting.ts'
import { AccountingPayment } from '@/schema/accounting.ts'
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

const today = () => new Date().toISOString().split('T')[0]

const NONE = '__none__'
const PROTOCOL_LABELS_FA: Record<string, string> = {
  wireguard: 'وایرگارد',
  user_manager: 'یوزرمنجیر',
  v2ray: 'V2Ray',
}

export function PaymentsTab() {
  const queryClient = useQueryClient()
  const { data: payments = [], isLoading } = useQuery({
    queryKey: ['accounting_payments'],
    queryFn: fetchPayments,
  })

  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<AccountingPayment | null>(null)
  const [customerLabel, setCustomerLabel] = useState('')
  const [amount, setAmount] = useState<number | string>('')
  const [cost, setCost] = useState<number | string>('')
  const [protocol, setProtocol] = useState<string>(NONE)
  const [resourceId, setResourceId] = useState<string>(NONE)
  const [note, setNote] = useState('')
  const [paidAt, setPaidAt] = useState(today())
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [pendingReceiptId, setPendingReceiptId] = useState<number | null>(null)

  const { data: sellableResources = [] } = useQuery({
    queryKey: ['accounting_sellable_resources', protocol],
    queryFn: () => fetchSellableResources(protocol),
    enabled: protocol !== NONE,
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['accounting_payments'] })
    queryClient.invalidateQueries({ queryKey: ['accounting_summary'] })
    queryClient.invalidateQueries({ queryKey: ['accounting_user_profitability'] })
    queryClient.invalidateQueries({ queryKey: ['accounting_location_profitability'] })
  }

  const createMutation = useMutation({
    mutationFn: createPayment,
    onSuccess: () => {
      toast.success('پرداخت ثبت شد.')
      invalidate()
      setOpen(false)
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'ثبت پرداخت ناموفق بود.')),
  })
  const updateMutation = useMutation({
    mutationFn: updatePayment,
    onSuccess: () => {
      toast.success('پرداخت به‌روزرسانی شد.')
      invalidate()
      setOpen(false)
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'به‌روزرسانی ناموفق بود.')),
  })
  const deleteMutation = useMutation({
    mutationFn: deletePayment,
    onSuccess: () => {
      toast.success('پرداخت حذف شد.')
      invalidate()
    },
  })
  const uploadMutation = useMutation({
    mutationFn: uploadReceipt,
    onSuccess: () => {
      toast.success('رسید بارگذاری شد.')
      invalidate()
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'بارگذاری رسید ناموفق بود.')),
  })

  const resetForm = () => {
    setEditing(null)
    setCustomerLabel('')
    setAmount('')
    setCost('')
    setProtocol(NONE)
    setResourceId(NONE)
    setNote('')
    setPaidAt(today())
  }

  const openCreate = () => {
    resetForm()
    setOpen(true)
  }

  const openEdit = (payment: AccountingPayment) => {
    setEditing(payment)
    setCustomerLabel(payment.customer_label)
    setAmount(payment.amount_toman)
    setCost(payment.cost_toman || '')
    setProtocol(payment.protocol ?? NONE)
    setResourceId(payment.resource_id ? String(payment.resource_id) : NONE)
    setNote(payment.note ?? '')
    setPaidAt(payment.paid_at)
    setOpen(true)
  }

  const handleSubmit = () => {
    const parsedAmount = Number(amount)
    const parsedCost = cost ? Number(cost) : 0
    if (!customerLabel.trim()) {
      toast.error('نام/برچسب مشتری الزامی است.')
      return
    }
    if (!Number.isFinite(parsedAmount) || parsedAmount <= 0) {
      toast.error('مبلغ باید عددی مثبت باشد.')
      return
    }
    const req = {
      customer_label: customerLabel,
      amount_toman: parsedAmount,
      cost_toman: parsedCost,
      protocol: protocol === NONE ? null : protocol,
      resource_id: resourceId === NONE ? null : Number(resourceId),
      note: note || null,
      paid_at: paidAt,
    }
    if (editing) {
      updateMutation.mutate({ id: editing.id, ...req })
    } else {
      createMutation.mutate(req)
    }
  }

  const handlePickReceipt = (id: number) => {
    setPendingReceiptId(id)
    fileInputRef.current?.click()
  }

  const handleFileSelected = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file && pendingReceiptId !== null) {
      uploadMutation.mutate({ id: pendingReceiptId, file })
    }
    e.target.value = ''
    setPendingReceiptId(null)
  }

  return (
    <Card>
      <CardHeader className='flex flex-row items-center justify-between'>
        <CardTitle>پرداخت‌های دریافتی از مشتریان</CardTitle>
        <Button onClick={openCreate} className='gap-2'>
          <IconPlus className='h-4 w-4' />
          ثبت پرداخت
        </Button>
      </CardHeader>
      <CardContent>
        <input
          ref={fileInputRef}
          type='file'
          accept='image/*,application/pdf'
          className='hidden'
          onChange={handleFileSelected}
        />
        {isLoading ? (
          <Skeleton className='h-40 w-full rounded-lg' />
        ) : payments.length === 0 ? (
          <EmptyState message='هنوز هیچ پرداختی ثبت نشده است.' />
        ) : (
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>مشتری</TableHead>
                  <TableHead>پروتکل/لوکیشن</TableHead>
                  <TableHead>فروش</TableHead>
                  <TableHead>هزینه تمام‌شده</TableHead>
                  <TableHead>سود</TableHead>
                  <TableHead>تاریخ</TableHead>
                  <TableHead>رسید</TableHead>
                  <TableHead className='w-24'></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {payments.map((payment) => (
                  <TableRow key={payment.id}>
                    <TableCell
                      className='cursor-pointer font-medium'
                      onClick={() => openEdit(payment)}
                    >
                      {payment.customer_label}
                    </TableCell>
                    <TableCell>
                      {payment.protocol ? (
                        <div className='flex flex-col gap-1'>
                          <Badge variant='secondary'>
                            {PROTOCOL_LABELS_FA[payment.protocol] ?? payment.protocol}
                          </Badge>
                          {(payment.location_label || payment.location_key) && (
                            <span className='text-muted-foreground text-xs'>
                              {payment.location_label || payment.location_key}
                            </span>
                          )}
                        </div>
                      ) : (
                        '—'
                      )}
                    </TableCell>
                    <TableCell className='tabular-nums'>
                      {formatCurrencyFa(payment.amount_toman)}
                    </TableCell>
                    <TableCell className='tabular-nums'>
                      {formatCurrencyFa(payment.cost_toman)}
                    </TableCell>
                    <TableCell
                      className={`tabular-nums font-medium ${payment.profit_toman >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'}`}
                    >
                      {formatCurrencyFa(payment.profit_toman)}
                    </TableCell>
                    <TableCell>{payment.paid_at}</TableCell>
                    <TableCell>
                      {payment.has_receipt ? (
                        <a
                          href={`/api/accounting/payments/${payment.id}/receipt`}
                          target='_blank'
                          rel='noreferrer'
                        >
                          <Badge variant='secondary'>مشاهده رسید</Badge>
                        </a>
                      ) : (
                        <Button
                          variant='ghost'
                          size='icon'
                          title='بارگذاری رسید'
                          onClick={() => handlePickReceipt(payment.id)}
                        >
                          <IconPaperclip className='h-4 w-4' />
                        </Button>
                      )}
                    </TableCell>
                    <TableCell>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='text-destructive'
                        onClick={() => deleteMutation.mutate(payment.id)}
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
            <DialogTitle>{editing ? 'ویرایش پرداخت' : 'ثبت پرداخت'}</DialogTitle>
          </DialogHeader>
          <div className='space-y-4'>
            <div className='grid grid-cols-2 gap-3'>
              <div className='space-y-2'>
                <Label>پروتکل (اختیاری)</Label>
                <Select
                  value={protocol}
                  onValueChange={(value) => {
                    setProtocol(value)
                    setResourceId(NONE)
                  }}
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={NONE}>بدون پروتکل خاص</SelectItem>
                    <SelectItem value='wireguard'>وایرگارد</SelectItem>
                    <SelectItem value='user_manager'>یوزرمنجیر</SelectItem>
                    <SelectItem value='v2ray'>V2Ray</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className='space-y-2'>
                <Label>کاربر فروخته‌شده (اختیاری)</Label>
                <Select
                  value={resourceId}
                  onValueChange={setResourceId}
                  disabled={protocol === NONE}
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue placeholder='انتخاب کاربر' />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={NONE}>بدون کاربر مشخص</SelectItem>
                    {sellableResources.map((r) => (
                      <SelectItem key={r.id} value={String(r.id)}>
                        {r.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className='space-y-2'>
              <Label>مشتری / برچسب</Label>
              <Input
                value={customerLabel}
                onChange={(e) => setCustomerLabel(e.target.value)}
              />
            </div>

            <div className='grid grid-cols-2 gap-3'>
              <div className='space-y-2'>
                <Label>مبلغ فروش (تومان)</Label>
                <Input
                  type='number'
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                />
              </div>
              <div className='space-y-2'>
                <Label>هزینه تمام‌شده (تومان، اختیاری)</Label>
                <Input
                  type='number'
                  value={cost}
                  onChange={(e) => setCost(e.target.value)}
                  placeholder='0'
                />
              </div>
            </div>

            <div className='space-y-2'>
              <Label>یادداشت (اختیاری)</Label>
              <Input value={note} onChange={(e) => setNote(e.target.value)} />
            </div>
            <div className='space-y-2'>
              <Label>تاریخ (YYYY-MM-DD)</Label>
              <Input value={paidAt} onChange={(e) => setPaidAt(e.target.value)} />
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
