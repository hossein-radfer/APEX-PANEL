import { useEffect, useMemo, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import {
  fetchResellerBillingPrices,
  setResellerBillingPrices,
  fetchResellerBillingTiers,
  setResellerBillingTiers,
} from '@/api/resellers.ts'
import { useResellersListQuery } from '@/hooks/resellers/useResellersListQuery.ts'
import { useAssignedInterfacesQuery } from '@/hooks/resellers/useAssignedInterfacesQuery.ts'
import { useAssignedUserManagerGroupsQuery } from '@/hooks/resellers/useAssignedUserManagerGroupsQuery.ts'
import { useInterfacesListQuery } from '@/hooks/interfaces/useInterfacesListQuery.ts'
import { fetchAssignedXuiPanelSummaries } from '@/api/v2ray.ts'
import { useAuthStore } from '@/stores/authStore.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { IconArrowLeft, IconCoin, IconTrash, IconPlus } from '@tabler/icons-react'
import { useNavigate, useParams } from '@tanstack/react-router'

// This page configures Payment-based billing's pricing for one reseller:
// a flat product-wide price, per-location overrides (WireGuard interface /
// User Manager group / V2Ray panel), and manual volume-discount tiers --
// the admin's own explicit requirement: usage should be priced by WHERE
// it happens (European WireGuard interfaces cheaper than Middle-Eastern
// ones, since real bandwidth cost differs) and by HOW MUCH a reseller
// uses in total (a 4000GB/month reseller should pay less per GB than a
// 200GB one), not just one flat rate per product. Meaningless for a
// Volume-based reseller (see the Billing Mode section on the reseller's
// own edit form), but left reachable regardless since a reseller may be
// switched to Payment-based at any time.
const PRODUCTS: {
  key: 'WIREGUARD' | 'USER_MANAGER' | 'V2RAY' | 'APPLICATION'
  label: string
}[] = [
  { key: 'WIREGUARD', label: 'وایرگارد' },
  { key: 'USER_MANAGER', label: 'User Manager (L2TP/PPTP/SSTP/OpenVPN)' },
  { key: 'V2RAY', label: 'V2Ray' },
  // Applications has no per-location concept of its own (see
  // model.Application's own doc comment -- one Application fans out
  // across WireGuard/User Manager/V2Ray resources at once, not one single
  // location), so locationOptions.APPLICATION is intentionally left
  // undefined below -- this product only ever shows the flat/tier pricing
  // sections, never a per-location override row.
  { key: 'APPLICATION', label: 'اپلیکیشن‌ها' },
]

interface LocationOption {
  key: string
  label: string
}

export default function ResellerBillingPricesPage() {
  const { id } = useParams({ strict: false }) as { id?: string }
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const role = useAuthStore((state) => state.auth.admin?.role)
  const isReseller = role === 'reseller'

  const [pickedResellerID, setPickedResellerID] = useState('')
  const { data: resellers = [] } = useResellersListQuery(!isReseller && !id)

  const resellerID = useMemo(() => {
    if (id) return parseInt(id, 10)
    const parsed = Number.parseInt(pickedResellerID, 10)
    return Number.isFinite(parsed) && parsed > 0 ? parsed : null
  }, [id, pickedResellerID])

  // -- Flat / per-location prices --
  const { data: prices, isLoading: pricesLoading } = useQuery({
    queryKey: ['reseller_billing_prices', resellerID],
    queryFn: () => fetchResellerBillingPrices(resellerID!),
    enabled: !!resellerID,
  })

  // Location options this reseller actually has, per product -- only
  // interfaces/groups/panels the reseller is assigned to are offered for
  // per-location pricing, since pricing a location they can't use would
  // be meaningless.
  const { data: assignedInterfaceIds = [] } = useAssignedInterfacesQuery(
    resellerID ?? undefined
  )
  const { data: interfacesList = [] } = useInterfacesListQuery(!!resellerID)
  const { data: assignedUmGroups = [] } = useAssignedUserManagerGroupsQuery(
    resellerID ?? undefined
  )
  const { data: assignedPanels = [] } = useQuery({
    queryKey: ['reseller_assigned_xui_panel_summaries', resellerID],
    queryFn: () => fetchAssignedXuiPanelSummaries(resellerID!),
    enabled: !!resellerID,
  })

  const locationOptions: Record<string, LocationOption[]> = useMemo(() => {
    const wireguard = (assignedInterfaceIds || [])
      .map((interfaceId) => {
        const iface = (interfacesList || [])?.find((i) => i.id === interfaceId)
        return iface ? { key: iface.name, label: iface.name } : null
      })
      .filter((option): option is LocationOption => option !== null)

    const userManager = (assignedUmGroups || []).map((group) => ({
      key: group,
      label: group,
    }))

    const v2ray = (assignedPanels || []).map((panel) => ({
      key: String(panel.id),
      label: panel.name,
    }))

    return { WIREGUARD: wireguard, USER_MANAGER: userManager, V2RAY: v2ray }
  }, [assignedInterfaceIds, interfacesList, assignedUmGroups, assignedPanels])

  // amounts[product]['' ] is the flat/fallback price; amounts[product][locationKey]
  // is a per-location override.
  const [amounts, setAmounts] = useState<Record<string, Record<string, string>>>(
    {}
  )
  useEffect(() => {
    if (!prices) return
    const next: Record<string, Record<string, string>> = {}
    for (const price of prices) {
      if (!next[price.product]) next[price.product] = {}
      next[price.product][price.locationKey] = String(price.pricePerGbAmount)
    }
    setAmounts(next)
  }, [prices])

  const setAmount = (product: string, locationKey: string, value: string) => {
    setAmounts((prev) => ({
      ...prev,
      [product]: { ...prev[product], [locationKey]: value },
    }))
  }

  // Which location keys currently have a visible row -- starts from
  // whatever already has a saved override, plus any the admin adds via
  // "+ Add location price" below.
  const [visibleLocations, setVisibleLocations] = useState<
    Record<string, string[]>
  >({})
  useEffect(() => {
    if (!prices) return
    const next: Record<string, string[]> = {}
    for (const price of prices) {
      if (price.locationKey === '') continue
      if (!next[price.product]) next[price.product] = []
      next[price.product].push(price.locationKey)
    }
    setVisibleLocations(next)
  }, [prices])

  const savePricesMutation = useMutation({
    mutationFn: () => {
      const payload: {
        product: string
        locationKey: string
        pricePerGbAmount: number
      }[] = []
      for (const product of PRODUCTS) {
        const productAmounts = amounts[product.key] || {}
        for (const [locationKey, raw] of Object.entries(productAmounts)) {
          if (raw === undefined || raw === '') continue
          payload.push({
            product: product.key,
            locationKey,
            pricePerGbAmount: Math.round(Number(raw)),
          })
        }
      }
      return setResellerBillingPrices({ resellerId: resellerID!, prices: payload })
    },
    onSuccess: () => {
      toast.success('قیمت‌ها ذخیره شد.')
      queryClient.invalidateQueries({
        queryKey: ['reseller_billing_prices', resellerID],
      })
    },
    onError: () => toast.error('ذخیره قیمت‌ها ناموفق بود.'),
  })

  // -- Volume tiers --
  const { data: tiers, isLoading: tiersLoading } = useQuery({
    queryKey: ['reseller_billing_tiers', resellerID],
    queryFn: () => fetchResellerBillingTiers(resellerID!),
    enabled: !!resellerID,
  })

  type TierRow = { minGb: string; maxGb: string; pricePerGbAmount: string }
  const [tierRows, setTierRows] = useState<Record<string, TierRow[]>>({})
  useEffect(() => {
    if (!tiers) return
    const next: Record<string, TierRow[]> = {}
    for (const tier of tiers) {
      if (!next[tier.product]) next[tier.product] = []
      next[tier.product].push({
        minGb: String(tier.minGb),
        maxGb: tier.maxGb === null ? '' : String(tier.maxGb),
        pricePerGbAmount: String(tier.pricePerGbAmount),
      })
    }
    setTierRows(next)
  }, [tiers])

  const addTierRow = (product: string) => {
    setTierRows((prev) => ({
      ...prev,
      [product]: [
        ...(prev[product] || []),
        { minGb: '', maxGb: '', pricePerGbAmount: '' },
      ],
    }))
  }
  const updateTierRow = (
    product: string,
    index: number,
    field: keyof TierRow,
    value: string
  ) => {
    setTierRows((prev) => {
      const rows = [...(prev[product] || [])]
      rows[index] = { ...rows[index], [field]: value }
      return { ...prev, [product]: rows }
    })
  }
  const removeTierRow = (product: string, index: number) => {
    setTierRows((prev) => {
      const rows = [...(prev[product] || [])]
      rows.splice(index, 1)
      return { ...prev, [product]: rows }
    })
  }

  const saveTiersMutation = useMutation({
    mutationFn: (product: string) => {
      const rows = tierRows[product] || []
      const validRows = rows
        .filter((row) => row.minGb !== '' && row.pricePerGbAmount !== '')
        .map((row) => ({
          minGb: Math.round(Number(row.minGb)),
          maxGb: row.maxGb === '' ? null : Math.round(Number(row.maxGb)),
          pricePerGbAmount: Math.round(Number(row.pricePerGbAmount)),
        }))
      return setResellerBillingTiers({
        resellerId: resellerID!,
        product,
        tiers: validRows,
      })
    },
    onSuccess: () => {
      toast.success('پله‌های قیمتی ذخیره شد.')
      queryClient.invalidateQueries({
        queryKey: ['reseller_billing_tiers', resellerID],
      })
    },
    onError: () => toast.error('ذخیره پله‌های قیمتی ناموفق بود.'),
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
          <h1 className='text-2xl font-bold tracking-tight'>قیمت‌گذاری پرداختی</h1>
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
                <Label htmlFor='billing-prices-reseller-id'>نماینده</Label>
                <Select value={pickedResellerID} onValueChange={setPickedResellerID}>
                  <SelectTrigger id='billing-prices-reseller-id' className='w-full'>
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
                <p className='text-muted-foreground'>
                  برای تنظیم قیمت‌های پرداختی، یک نماینده انتخاب کنید.
                </p>
              </CardContent>
            </Card>
          ) : (
            <>
              <Card>
                <CardHeader className='flex flex-row items-center gap-3 pb-2'>
                  <IconCoin className='h-5 w-5' />
                  <CardTitle>قیمت هر گیگابایت (تومان)</CardTitle>
                </CardHeader>
                <CardContent className='space-y-6'>
                  <p className='text-muted-foreground text-sm'>
                    قیمت پیش‌فرض برای هر محصول، و در صورت نیاز، قیمت جداگانه
                    برای هر لوکیشن (اینترفیس وایرگارد / گروپ یوزرمنجیر / پنل
                    V2Ray) -- اگر برای یک لوکیشن قیمت جداگانه تنظیم نشود،
                    همان قیمت پیش‌فرض محصول اعمال می‌شود.
                  </p>

                  {pricesLoading ? (
                    <p className='text-muted-foreground'>در حال بارگذاری...</p>
                  ) : (
                    <div className='space-y-6'>
                      {PRODUCTS.map((product) => {
                        const options = locationOptions[product.key] || []
                        const shownLocationKeys = visibleLocations[product.key] || []
                        const unusedOptions = options.filter(
                          (option) => !shownLocationKeys.includes(option.key)
                        )
                        return (
                          <div
                            key={product.key}
                            className='space-y-3 rounded-md border p-4'
                          >
                            <div className='space-y-1'>
                              <Label htmlFor={`price-${product.key}`}>
                                {product.label} -- قیمت پیش‌فرض
                              </Label>
                              <Input
                                id={`price-${product.key}`}
                                type='number'
                                min='0'
                                step='1'
                                placeholder='0'
                                value={amounts[product.key]?.[''] ?? ''}
                                onChange={(e) =>
                                  setAmount(product.key, '', e.target.value)
                                }
                              />
                            </div>

                            {shownLocationKeys.length > 0 && (
                              <div className='space-y-2 border-t pt-3'>
                                <p className='text-muted-foreground text-xs'>
                                  قیمت‌های اختصاصی هر لوکیشن
                                </p>
                                {shownLocationKeys.map((locationKey) => {
                                  const option = options.find(
                                    (o) => o.key === locationKey
                                  )
                                  return (
                                    <div
                                      key={locationKey}
                                      className='flex items-center gap-2'
                                    >
                                      <span className='w-40 truncate text-sm'>
                                        {option?.label ?? locationKey}
                                      </span>
                                      <Input
                                        type='number'
                                        min='0'
                                        step='1'
                                        placeholder='0'
                                        value={
                                          amounts[product.key]?.[locationKey] ?? ''
                                        }
                                        onChange={(e) =>
                                          setAmount(
                                            product.key,
                                            locationKey,
                                            e.target.value
                                          )
                                        }
                                      />
                                      <Button
                                        type='button'
                                        variant='ghost'
                                        size='icon'
                                        onClick={() => {
                                          setVisibleLocations((prev) => ({
                                            ...prev,
                                            [product.key]: (
                                              prev[product.key] || []
                                            ).filter((k) => k !== locationKey),
                                          }))
                                          setAmount(product.key, locationKey, '')
                                        }}
                                      >
                                        <IconTrash className='h-4 w-4' />
                                      </Button>
                                    </div>
                                  )
                                })}
                              </div>
                            )}

                            {unusedOptions.length > 0 && (
                              <Select
                                value=''
                                onValueChange={(locationKey) => {
                                  setVisibleLocations((prev) => ({
                                    ...prev,
                                    [product.key]: [
                                      ...(prev[product.key] || []),
                                      locationKey,
                                    ],
                                  }))
                                }}
                              >
                                <SelectTrigger className='w-full sm:w-64'>
                                  <SelectValue placeholder='+ افزودن قیمت اختصاصی لوکیشن' />
                                </SelectTrigger>
                                <SelectContent>
                                  {unusedOptions.map((option) => (
                                    <SelectItem key={option.key} value={option.key}>
                                      {option.label}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                            )}
                          </div>
                        )
                      })}
                    </div>
                  )}

                  <Button
                    className='w-full'
                    onClick={() => savePricesMutation.mutate()}
                    disabled={savePricesMutation.isPending || pricesLoading}
                  >
                    {savePricesMutation.isPending
                      ? 'در حال ذخیره...'
                      : 'ذخیره قیمت‌ها'}
                  </Button>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className='pb-2'>
                  <CardTitle>پله‌های تخفیف حجمی</CardTitle>
                </CardHeader>
                <CardContent className='space-y-6'>
                  <p className='text-muted-foreground text-sm'>
                    برای نمایندگانی که حجم مصرف بالایی دارند می‌توانید قیمت
                    پایین‌تری تعریف کنید -- مثلاً ۰ تا ۵۰۰ گیگ با یک قیمت،
                    ۵۰۰ تا ۲۰۰۰ گیگ ارزان‌تر، و بالای آن ارزان‌تر از همه. در
                    صورت تعریف پله برای یک محصول، پله‌ها جایگزین قیمت
                    پیش‌فرض/اختصاصی همان محصول می‌شوند. آخرین پله را برای
                    "بدون سقف" خالی بگذارید.
                  </p>

                  {tiersLoading ? (
                    <p className='text-muted-foreground'>در حال بارگذاری...</p>
                  ) : (
                    <div className='space-y-6'>
                      {PRODUCTS.map((product) => {
                        const rows = tierRows[product.key] || []
                        return (
                          <div
                            key={product.key}
                            className='space-y-3 rounded-md border p-4'
                          >
                            <p className='text-sm font-medium'>{product.label}</p>

                            {rows.length === 0 ? (
                              <p className='text-muted-foreground text-xs'>
                                پله‌ای تعریف نشده -- قیمت پیش‌فرض/اختصاصی بالا
                                اعمال می‌شود.
                              </p>
                            ) : (
                              <div className='space-y-2'>
                                <div className='text-muted-foreground grid grid-cols-[1fr_1fr_1fr_auto] gap-2 text-xs'>
                                  <span>از (گیگابایت)</span>
                                  <span>تا (گیگابایت، خالی = بی‌سقف)</span>
                                  <span>قیمت هر گیگ (تومان)</span>
                                  <span />
                                </div>
                                {rows.map((row, index) => (
                                  <div
                                    key={index}
                                    className='grid grid-cols-[1fr_1fr_1fr_auto] gap-2'
                                  >
                                    <Input
                                      type='number'
                                      min='0'
                                      step='1'
                                      value={row.minGb}
                                      onChange={(e) =>
                                        updateTierRow(
                                          product.key,
                                          index,
                                          'minGb',
                                          e.target.value
                                        )
                                      }
                                    />
                                    <Input
                                      type='number'
                                      min='1'
                                      step='1'
                                      placeholder='بی‌سقف'
                                      value={row.maxGb}
                                      onChange={(e) =>
                                        updateTierRow(
                                          product.key,
                                          index,
                                          'maxGb',
                                          e.target.value
                                        )
                                      }
                                    />
                                    <Input
                                      type='number'
                                      min='0'
                                      step='1'
                                      value={row.pricePerGbAmount}
                                      onChange={(e) =>
                                        updateTierRow(
                                          product.key,
                                          index,
                                          'pricePerGbAmount',
                                          e.target.value
                                        )
                                      }
                                    />
                                    <Button
                                      type='button'
                                      variant='ghost'
                                      size='icon'
                                      onClick={() =>
                                        removeTierRow(product.key, index)
                                      }
                                    >
                                      <IconTrash className='h-4 w-4' />
                                    </Button>
                                  </div>
                                ))}
                              </div>
                            )}

                            <div className='flex items-center gap-2'>
                              <Button
                                type='button'
                                variant='outline'
                                size='sm'
                                onClick={() => addTierRow(product.key)}
                              >
                                <IconPlus className='h-4 w-4' />
                                افزودن پله
                              </Button>
                              <Button
                                type='button'
                                size='sm'
                                onClick={() => saveTiersMutation.mutate(product.key)}
                                disabled={saveTiersMutation.isPending}
                              >
                                ذخیره پله‌های {product.label}
                              </Button>
                            </div>
                          </div>
                        )
                      })}
                    </div>
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
