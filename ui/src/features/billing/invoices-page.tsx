import { useMemo, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { IconReceipt, IconSend, IconCash, IconBan } from '@tabler/icons-react'
import { toast } from 'sonner'
import { formatCurrencyFa } from '@/features/reports/lib/format.ts'
import { useAuthStore } from '@/stores/authStore.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  createInvoice,
  issueInvoice,
  payInvoice,
  cancelInvoice,
  type Invoice,
} from '@/api/invoice.ts'
import { useResellerInvoicesQuery } from '@/hooks/billing/useInvoiceQueries.ts'
import { useResellersListQuery } from '@/hooks/resellers/useResellersListQuery.ts'

export default function InvoicesPage() {
  const queryClient = useQueryClient()

  const role = useAuthStore((state) => state.auth.admin?.role)
  const authResellerID = useAuthStore((state) => state.auth.admin?.reseller_id)
  const isReseller = role === 'reseller'

  const [adminResellerIDInput, setAdminResellerIDInput] = useState('')
  const [amount, setAmount] = useState('')
  const [description, setDescription] = useState('')
  const [dueDate, setDueDate] = useState('')

  const { data: resellers = [] } = useResellersListQuery(!isReseller)

  const effectiveResellerID = useMemo(() => {
    if (isReseller) {
      return authResellerID ?? null
    }
    const parsed = Number.parseInt(adminResellerIDInput, 10)
    return Number.isFinite(parsed) && parsed > 0 ? parsed : null
  }, [isReseller, authResellerID, adminResellerIDInput])

  const { data: invoices, isLoading } = useResellerInvoicesQuery(effectiveResellerID)

  const invalidateInvoices = () => {
    queryClient.invalidateQueries({ queryKey: ['invoices', effectiveResellerID] })
    queryClient.invalidateQueries({ queryKey: ['wallet_balance', effectiveResellerID] })
    queryClient.invalidateQueries({ queryKey: ['wallet_ledger', effectiveResellerID] })
  }

  const createMutation = useMutation({
    mutationFn: async () => {
      if (!effectiveResellerID) {
        throw new Error('reseller id is required')
      }
      const amountValue = Math.round(Number.parseFloat(amount))
      return createInvoice(effectiveResellerID, {
        amount: amountValue,
        description,
        due_date: dueDate ? Math.floor(new Date(dueDate).getTime() / 1000) : undefined,
        auto_issue: true,
      })
    },
    onSuccess: () => {
      toast.success('فاکتور ایجاد و صادر شد.')
      setAmount('')
      setDescription('')
      setDueDate('')
      invalidateInvoices()
    },
  })

  const issueMutation = useMutation({
    mutationFn: (invoiceID: number) => issueInvoice(invoiceID),
    onSuccess: () => {
      toast.success('فاکتور صادر شد.')
      invalidateInvoices()
    },
  })

  const payMutation = useMutation({
    mutationFn: (invoiceID: number) => payInvoice(invoiceID),
    onSuccess: () => {
      toast.success('فاکتور پرداخت شد.')
      invalidateInvoices()
    },
  })

  const cancelMutation = useMutation({
    mutationFn: (invoiceID: number) => cancelInvoice(invoiceID),
    onSuccess: () => {
      toast.success('فاکتور لغو شد.')
      invalidateInvoices()
    },
  })

  return (
    <>
      <Header>
        <div className='flex items-center gap-3'>
          <IconReceipt className='h-5 w-5' />
          <h1 className='text-2xl font-bold tracking-tight'>فاکتورها</h1>
        </div>
        <div className='ml-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main>
        <div className='space-y-6'>
          {!isReseller && (
            <Card>
              <CardHeader>
                <CardTitle>انتخاب نماینده</CardTitle>
              </CardHeader>
              <CardContent className='max-w-sm space-y-2'>
                <Label htmlFor='reseller-id'>نماینده</Label>
                <Select
                  value={adminResellerIDInput}
                  onValueChange={setAdminResellerIDInput}
                >
                  <SelectTrigger id='reseller-id' className='w-full'>
                    <SelectValue placeholder='یک نماینده انتخاب کنید' />
                  </SelectTrigger>
                  <SelectContent>
                    {resellers.length === 0 ? (
                      <div className='text-muted-foreground px-2 py-1.5 text-sm'>
                        نماینده‌ای یافت نشد.
                      </div>
                    ) : (
                      resellers.map((reseller) => (
                        <SelectItem
                          key={reseller.id}
                          value={String(reseller.id)}
                        >
                          {reseller.name} ({reseller.username})
                        </SelectItem>
                      ))
                    )}
                  </SelectContent>
                </Select>
              </CardContent>
            </Card>
          )}

          {!isReseller && (
            <Card>
              <CardHeader>
                <CardTitle>ایجاد فاکتور</CardTitle>
              </CardHeader>
              <CardContent className='grid gap-4 md:grid-cols-2'>
                <div className='space-y-2'>
                  <Label htmlFor='amount'>مبلغ (تومان)</Label>
                  <Input
                    id='amount'
                    type='number'
                    min='1'
                    step='1'
                    value={amount}
                    onChange={(e) => setAmount(e.target.value)}
                    placeholder='0'
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='due-date'>تاریخ سررسید</Label>
                  <Input
                    id='due-date'
                    type='date'
                    value={dueDate}
                    onChange={(e) => setDueDate(e.target.value)}
                  />
                </div>
                <div className='space-y-2 md:col-span-2'>
                  <Label htmlFor='description'>توضیحات</Label>
                  <Input
                    id='description'
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    placeholder='اشتراک ماهانه یا اصلاحیه'
                  />
                </div>
                <div className='space-y-2 md:col-span-2'>
                  <Button
                    onClick={() => createMutation.mutate()}
                    disabled={!effectiveResellerID || !amount || createMutation.isPending}
                  >
                    {createMutation.isPending ? 'در حال ایجاد...' : 'ایجاد و صدور فاکتور'}
                  </Button>
                  {!effectiveResellerID && (
                    <p className='text-muted-foreground text-sm'>
                      پیش از ایجاد فاکتور، یک نماینده انتخاب کنید.
                    </p>
                  )}
                  {effectiveResellerID && !amount && (
                    <p className='text-muted-foreground text-sm'>
                      پیش از ایجاد فاکتور، مبلغ را وارد کنید.
                    </p>
                  )}
                </div>
              </CardContent>
            </Card>
          )}

          <Card>
            <CardHeader>
              <CardTitle>لیست فاکتورها</CardTitle>
            </CardHeader>
            <CardContent className='space-y-3'>
              {!effectiveResellerID ? (
                <p className='text-muted-foreground'>برای مشاهده فاکتورها، یک نماینده انتخاب کنید.</p>
              ) : isLoading ? (
                <p className='text-muted-foreground'>در حال بارگذاری فاکتورها...</p>
              ) : invoices && invoices.length > 0 ? (
                invoices.map((invoice: Invoice) => {
                  const canIssue = invoice.status === 'DRAFT' && !isReseller
                  const canPay = invoice.status === 'ISSUED'
                  const canCancel = (invoice.status === 'DRAFT' || invoice.status === 'ISSUED') && !isReseller
                  const statusLabel =
                    invoice.status === 'DRAFT'
                      ? 'پیش‌نویس'
                      : invoice.status === 'ISSUED'
                        ? 'صادرشده'
                        : invoice.status === 'PAID'
                          ? 'پرداخت‌شده'
                          : invoice.status === 'CANCELLED'
                            ? 'لغوشده'
                            : invoice.status

                  return (
                    <div key={invoice.id} className='rounded-lg border p-4'>
                      <div className='flex flex-wrap items-center justify-between gap-2'>
                        <div>
                          <p className='font-medium'>{invoice.invoice_number}</p>
                          <p className='text-muted-foreground text-sm'>{invoice.description || 'بدون توضیحات'}</p>
                        </div>
                        <div className='flex items-center gap-2'>
                          <Badge variant={invoice.status === 'PAID' ? 'default' : 'secondary'}>
                            {statusLabel}
                          </Badge>
                          <p className='text-sm font-semibold tabular-nums'>
                            {formatCurrencyFa(invoice.amount)}
                          </p>
                        </div>
                      </div>

                      <div className='mt-3 flex flex-wrap gap-2'>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => issueMutation.mutate(invoice.id)}
                          disabled={!canIssue || issueMutation.isPending}
                        >
                          <IconSend className='mr-1 h-4 w-4' />
                          صدور
                        </Button>
                        <Button
                          size='sm'
                          onClick={() => payMutation.mutate(invoice.id)}
                          disabled={!canPay || payMutation.isPending}
                        >
                          <IconCash className='mr-1 h-4 w-4' />
                          پرداخت
                        </Button>
                        <Button
                          variant='destructive'
                          size='sm'
                          onClick={() => cancelMutation.mutate(invoice.id)}
                          disabled={!canCancel || cancelMutation.isPending}
                        >
                          <IconBan className='mr-1 h-4 w-4' />
                          لغو
                        </Button>
                      </div>
                    </div>
                  )
                })
              ) : (
                <p className='text-muted-foreground'>فاکتوری یافت نشد.</p>
              )}
            </CardContent>
          </Card>
        </div>
      </Main>
    </>
  )
}
