import { useMemo, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { fetchWalletBalance, fetchLedgerHistory, creditWallet, debitWallet, LedgerEntry } from '@/api/wallet.ts'
import { formatCurrencyFa, formatDateTimeFa } from '@/features/reports/lib/format.ts'
import { useResellersListQuery } from '@/hooks/resellers/useResellersListQuery.ts'
import { useAuthStore } from '@/stores/authStore.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { IconArrowLeft, IconWallet, IconReceipt, IconPlus, IconMinus } from '@tabler/icons-react'
import { useNavigate, useParams } from '@tanstack/react-router'

// Whole-Toman amounts throughout -- this panel bills exclusively in Toman
// (Iran's currency, no meaningful sub-unit in practical use), so amounts
// are entered/displayed as plain integers via formatCurrencyFa, matching
// features/accounting's established convention. No cents scaling, no
// currency code.

export default function ResellerWalletPage() {
  const { id } = useParams({ strict: false }) as { id?: string }
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const role = useAuthStore((state) => state.auth.admin?.role)
  const isReseller = role === 'reseller'
  const authResellerID = useAuthStore((state) => state.auth.admin?.reseller_id)

  const [pickedResellerID, setPickedResellerID] = useState('')
  const { data: resellers = [] } = useResellersListQuery(!isReseller && !id)

  const resellerID = useMemo(() => {
    if (id) return parseInt(id, 10)
    if (isReseller) return authResellerID ?? null
    const parsed = Number.parseInt(pickedResellerID, 10)
    return Number.isFinite(parsed) && parsed > 0 ? parsed : null
  }, [id, isReseller, authResellerID, pickedResellerID])

  const [creditAmount, setCreditAmount] = useState('')
  const [creditDescription, setCreditDescription] = useState('')
  const [debitAmount, setDebitAmount] = useState('')
  const [debitDescription, setDebitDescription] = useState('')

  const { data: walletData, isLoading: isWalletLoading } = useQuery({
    queryKey: ['wallet_balance', resellerID],
    queryFn: () => fetchWalletBalance(resellerID!),
    enabled: !!resellerID,
  })

  const { data: ledgerEntries, isLoading: isLedgerLoading } = useQuery({
    queryKey: ['wallet_ledger', resellerID, 50],
    queryFn: () => fetchLedgerHistory(resellerID!, 50),
    enabled: !!resellerID,
  })

  const creditMutation = useMutation({
    mutationFn: () => creditWallet(resellerID!, Math.round(Number(creditAmount)), creditDescription),
    onSuccess: (data) => {
      toast.success(`اعتبار افزوده شد. موجودی جدید: ${formatCurrencyFa(data.balance_after)}`)
      queryClient.invalidateQueries({ queryKey: ['wallet_balance', resellerID] })
      queryClient.invalidateQueries({ queryKey: ['wallet_ledger', resellerID] })
      setCreditAmount('')
      setCreditDescription('')
    },
    onError: () => toast.error('افزودن اعتبار ناموفق بود.'),
  })

  const debitMutation = useMutation({
    mutationFn: () => debitWallet(resellerID!, Math.round(Number(debitAmount)), debitDescription),
    onSuccess: (data) => {
      toast.success(`کسر انجام شد. موجودی جدید: ${formatCurrencyFa(data.balance_after)}`)
      queryClient.invalidateQueries({ queryKey: ['wallet_balance', resellerID] })
      queryClient.invalidateQueries({ queryKey: ['wallet_ledger', resellerID] })
      setDebitAmount('')
      setDebitDescription('')
    },
    onError: () => toast.error('کسر از اعتبار ناموفق بود.'),
  })

  return (
    <>
      <Header>
        <div className='flex items-center gap-4'>
          {id && (
            <Button variant='ghost' size='icon' onClick={() => navigate({ to: '/resellers' })}>
              <IconArrowLeft className='h-4 w-4' />
            </Button>
          )}
          <h1 className='text-2xl font-bold tracking-tight'>مدیریت کیف پول</h1>
        </div>
        <div className='ml-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main>
        <div className='space-y-6'>
          {!id && !isReseller && (
            <Card>
              <CardHeader>
                <CardTitle>انتخاب نماینده</CardTitle>
              </CardHeader>
              <CardContent className='max-w-sm space-y-2'>
                <Label htmlFor='wallet-reseller-id'>نماینده</Label>
                <Select value={pickedResellerID} onValueChange={setPickedResellerID}>
                  <SelectTrigger id='wallet-reseller-id' className='w-full'>
                    <SelectValue placeholder='یک نماینده انتخاب کنید' />
                  </SelectTrigger>
                  <SelectContent>
                    {resellers.length === 0 ? (
                      <div className='text-muted-foreground px-2 py-1.5 text-sm'>
                        نماینده‌ای یافت نشد.
                      </div>
                    ) : (
                      resellers.map((reseller) => (
                        <SelectItem key={reseller.id} value={String(reseller.id)}>
                          {reseller.name} ({reseller.username})
                        </SelectItem>
                      ))
                    )}
                  </SelectContent>
                </Select>
              </CardContent>
            </Card>
          )}

          {!resellerID ? (
            <Card>
              <CardContent className='pt-6'>
                <p className='text-muted-foreground'>برای مدیریت کیف پول، یک نماینده انتخاب کنید.</p>
              </CardContent>
            </Card>
          ) : (
          <>
          {/* Balance card */}
          <Card>
            <CardHeader className='flex flex-row items-center gap-3 pb-2'>
              <IconWallet className='h-5 w-5' />
              <CardTitle>موجودی فعلی</CardTitle>
            </CardHeader>
            <CardContent>
              {isWalletLoading ? (
                <p className='text-muted-foreground'>در حال بارگذاری...</p>
              ) : walletData ? (
                <div>
                  <div className='flex items-center gap-3'>
                    <p className='text-4xl font-bold tabular-nums'>
                      {formatCurrencyFa(walletData.balance_amount)}
                    </p>
                    {walletData.is_frozen && (
                      <Badge variant='destructive'>مسدود شده</Badge>
                    )}
                  </div>
                  <p className='text-muted-foreground mt-1 text-sm'>نماینده #{resellerID}</p>
                  {walletData.is_frozen && walletData.frozen_reason && (
                    <p className='text-destructive mt-1 text-sm'>
                      دلیل مسدودسازی: {walletData.frozen_reason}
                    </p>
                  )}
                </div>
              ) : (
                <p className='text-muted-foreground'>کیف پولی برای این نماینده یافت نشد.</p>
              )}
            </CardContent>
          </Card>

          {/* Credit / Debit operations */}
          <div className='grid gap-4 md:grid-cols-2'>
            <Card>
              <CardHeader className='flex flex-row items-center gap-3 pb-2'>
                <IconPlus className='text-green-600 h-5 w-5' />
                <CardTitle className='text-lg'>افزودن اعتبار</CardTitle>
              </CardHeader>
              <CardContent className='space-y-4'>
                <div className='space-y-2'>
                  <Label htmlFor='credit-amount'>مبلغ (تومان)</Label>
                  <Input
                    id='credit-amount'
                    type='number'
                    min='1'
                    step='1'
                    placeholder='0'
                    value={creditAmount}
                    onChange={(e) => setCreditAmount(e.target.value)}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='credit-desc'>توضیحات</Label>
                  <Input
                    id='credit-desc'
                    placeholder='شارژ توسط مدیر...'
                    value={creditDescription}
                    onChange={(e) => setCreditDescription(e.target.value)}
                  />
                </div>
                <Button
                  className='w-full'
                  onClick={() => creditMutation.mutate()}
                  disabled={!creditAmount || Number(creditAmount) <= 0 || creditMutation.isPending}
                >
                  {creditMutation.isPending ? 'در حال پردازش...' : 'افزودن اعتبار'}
                </Button>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className='flex flex-row items-center gap-3 pb-2'>
                <IconMinus className='text-red-600 h-5 w-5' />
                <CardTitle className='text-lg'>کسر از اعتبار</CardTitle>
              </CardHeader>
              <CardContent className='space-y-4'>
                <div className='space-y-2'>
                  <Label htmlFor='debit-amount'>مبلغ (تومان)</Label>
                  <Input
                    id='debit-amount'
                    type='number'
                    min='1'
                    step='1'
                    placeholder='0'
                    value={debitAmount}
                    onChange={(e) => setDebitAmount(e.target.value)}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='debit-desc'>توضیحات</Label>
                  <Input
                    id='debit-desc'
                    placeholder='دلیل کسر...'
                    value={debitDescription}
                    onChange={(e) => setDebitDescription(e.target.value)}
                  />
                </div>
                <Button
                  variant='destructive'
                  className='w-full'
                  onClick={() => debitMutation.mutate()}
                  disabled={!debitAmount || Number(debitAmount) <= 0 || debitMutation.isPending}
                >
                  {debitMutation.isPending ? 'در حال پردازش...' : 'کسر از اعتبار'}
                </Button>
              </CardContent>
            </Card>
          </div>

          {/* Ledger history */}
          <Card>
            <CardHeader className='flex flex-row items-center gap-3 pb-2'>
              <IconReceipt className='h-5 w-5' />
              <CardTitle>تاریخچه تراکنش‌ها</CardTitle>
            </CardHeader>
            <CardContent>
              {isLedgerLoading ? (
                <p className='text-muted-foreground'>در حال بارگذاری تراکنش‌ها...</p>
              ) : ledgerEntries && ledgerEntries.length > 0 ? (
                <div className='space-y-2'>
                  {ledgerEntries.map((entry: LedgerEntry) => (
                    <div
                      key={entry.id}
                      className='flex items-center justify-between rounded-lg border px-4 py-3'
                    >
                      <div>
                        <div className='flex items-center gap-2'>
                          <Badge variant={entry.amount >= 0 ? 'default' : 'destructive'} className='text-xs'>
                            {entry.entry_type === 'CREDIT'
                              ? 'واریز'
                              : entry.entry_type === 'DEBIT'
                                ? 'برداشت'
                                : 'اصلاحیه'}
                          </Badge>
                          <p className='font-medium text-sm'>{entry.description || entry.reference_type || '—'}</p>
                        </div>
                        <p className='text-muted-foreground text-xs mt-1'>
                          {formatDateTimeFa(new Date(entry.created_at * 1000).toISOString())}
                        </p>
                      </div>
                      <div className='text-right'>
                        <p className={`font-bold tabular-nums ${entry.amount >= 0 ? 'text-green-600' : 'text-red-600'}`}>
                          {entry.amount >= 0 ? '+' : ''}
                          {formatCurrencyFa(entry.amount)}
                        </p>
                        <p className='text-muted-foreground text-xs tabular-nums'>
                          موجودی پس از تراکنش: {formatCurrencyFa(entry.balance_after)}
                        </p>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <p className='text-muted-foreground'>هنوز تراکنشی ثبت نشده است.</p>
              )}
            </CardContent>
          </Card>
          </>
          )}
        </div>
      </Main>
    </>
  )
}
