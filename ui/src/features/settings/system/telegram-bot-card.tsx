import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import {
  IconBrandTelegram,
  IconCircleCheck,
  IconCircleX,
  IconPlayerPlay,
  IconPlus,
  IconTrash,
} from '@tabler/icons-react'
import {
  addExtraAdminChatID,
  fetchBotSettings,
  fetchExtraAdminChatIDs,
  removeExtraAdminChatID,
  testSocks5Connection,
  updateBotSettings,
} from '@/api/bot-settings.ts'
import {
  TestSocks5Result,
  UpdateBotSettingsRequest,
} from '@/schema/bot-settings.ts'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Separator } from '@/components/ui/separator'

function backendErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof AxiosError) {
    const message = err.response?.data?.message
    if (typeof message === 'string' && message.length > 0) return message
  }
  return fallback
}

interface ToggleDef {
  key: keyof Pick<
    UpdateBotSettingsRequest,
    | 'notify_login_alerts'
    | 'notify_purchase_receipts'
    | 'notify_live_log'
    | 'notify_quota_warnings'
    | 'notify_critical_alerts'
    | 'notify_account_status'
    | 'otp_enabled'
  >
  label: string
  description: string
}

const toggleDefs: ToggleDef[] = [
  {
    key: 'notify_login_alerts',
    label: 'هشدارهای ورود',
    description: 'اطلاع‌رسانی به نماینده هنگام ورود حساب او به پنل وب.',
  },
  {
    key: 'notify_quota_warnings',
    label: 'هشدارهای ترافیک',
    description: 'هشدار به مدیر و نماینده هنگام عبور مصرف از ۹۰٪ سهمیه.',
  },
  {
    key: 'notify_purchase_receipts',
    label: 'رسیدهای خرید',
    description: 'تأیید خرید بسته‌های ترافیک اضافه برای مدیر و نماینده.',
  },
  {
    key: 'notify_live_log',
    label: 'لاگ زنده',
    description: 'ارسال رویدادهای لحظه‌ای (مانند ایجاد وایرگارد جدید توسط نماینده) به مدیر.',
  },
  {
    key: 'notify_critical_alerts',
    label: 'هشدارهای بحرانی',
    description: 'اطلاع فوری به مدیر در صورت در دسترس نبودن میکروتیک یا پایگاه داده.',
  },
  {
    key: 'notify_account_status',
    label: 'تغییر وضعیت حساب',
    description: 'اطلاع‌رسانی به نماینده هنگام فعال یا غیرفعال شدن حساب او.',
  },
  {
    key: 'otp_enabled',
    label: 'کد یکبار مصرف ورود مدیر',
    description:
      'الزام یک کد ۴ رقمی (ارسال‌شده به این چت) پس از وارد کردن صحیح رمز عبور مدیر. شامل ورود نمایندگان نمی‌شود.',
  },
]

export function TelegramBotCard() {
  const queryClient = useQueryClient()
  const { data: settings, isLoading } = useQuery({
    queryKey: ['bot_settings'],
    queryFn: fetchBotSettings,
  })

  const [botToken, setBotToken] = useState('')
  const [adminChatId, setAdminChatId] = useState('')
  const [enabled, setEnabled] = useState(false)
  const [toggles, setToggles] = useState<Record<string, boolean>>({})

  // SOCKS5 proxy (item 7) -- for a panel installed on an Iran-hosted
  // server where Telegram's own API is network-filtered, routing the
  // bot's outbound calls through a proxy is the only way it can reach
  // Telegram at all.
  const [socks5Enabled, setSocks5Enabled] = useState(false)
  const [socks5Address, setSocks5Address] = useState('')
  const [socks5Username, setSocks5Username] = useState('')
  const [socks5Password, setSocks5Password] = useState('')

  // The public domain every Faoxima ("Bot X") instance's webhook/mini-app
  // links are built under -- this is the operator's own domain, so it must
  // be set explicitly per install rather than shipping as a fixed default.
  const [faoximaDomain, setFaoximaDomain] = useState('')

  const { data: extraAdmins = [] } = useQuery({
    queryKey: ['bot_extra_admins'],
    queryFn: fetchExtraAdminChatIDs,
  })
  const [newExtraChatId, setNewExtraChatId] = useState('')
  const [newExtraLabel, setNewExtraLabel] = useState('')

  const addExtraAdminMutation = useMutation({
    mutationFn: addExtraAdminChatID,
    onSuccess: () => {
      toast.success('گیرنده جدید اضافه شد.')
      queryClient.invalidateQueries({ queryKey: ['bot_extra_admins'] })
      setNewExtraChatId('')
      setNewExtraLabel('')
    },
    onError: (err) =>
      toast.error(backendErrorMessage(err, 'افزودن گیرنده ناموفق بود.')),
  })
  const removeExtraAdminMutation = useMutation({
    mutationFn: removeExtraAdminChatID,
    onSuccess: () => {
      toast.success('گیرنده حذف شد.')
      queryClient.invalidateQueries({ queryKey: ['bot_extra_admins'] })
    },
    onError: (err) =>
      toast.error(backendErrorMessage(err, 'حذف گیرنده ناموفق بود.')),
  })

  const handleAddExtraAdmin = () => {
    if (!newExtraChatId.trim()) {
      toast.error('آیدی چت را وارد کنید.')
      return
    }
    addExtraAdminMutation.mutate({
      chat_id: newExtraChatId.trim(),
      label: newExtraLabel.trim() || null,
    })
  }

  // socks5TestResult is deliberately reset (not just re-fetched) whenever
  // any of the proxy form fields change below -- a stale "متصل شد" badge
  // sitting next to fields the admin has since edited would be actively
  // misleading (item 5's whole point is telling the admin whether THIS
  // configuration works).
  const [socks5TestResult, setSocks5TestResult] =
    useState<TestSocks5Result | null>(null)
  const testSocks5Mutation = useMutation({
    mutationFn: testSocks5Connection,
    onSuccess: (result) => setSocks5TestResult(result),
    onError: (err) => {
      setSocks5TestResult(null)
      toast.error(backendErrorMessage(err, 'تست اتصال ناموفق بود.'))
    },
  })

  const handleTestSocks5 = () => {
    if (!socks5Address.trim()) {
      toast.error('ابتدا آدرس پروکسی را وارد کنید.')
      return
    }
    setSocks5TestResult(null)
    testSocks5Mutation.mutate({
      address: socks5Address.trim(),
      username: socks5Username.trim(),
      password: socks5Password,
      password_unchanged:
        socks5Password.trim().length === 0 && settings?.socks5_password_set,
    })
  }

  // Sync local form state whenever fresh settings load -- deliberately NOT
  // a controlled round-trip of bot_token: the input always starts empty, so
  // saving without touching it never overwrites the real token with the
  // masked placeholder the GET response returns.
  useEffect(() => {
    if (!settings) return
    setAdminChatId(settings.admin_chat_id)
    setEnabled(settings.enabled)
    setSocks5Enabled(settings.socks5_enabled)
    setSocks5Address(settings.socks5_address)
    setSocks5Username(settings.socks5_username)
    setFaoximaDomain(settings.faoxima_domain)
    setToggles({
      notify_login_alerts: settings.notify_login_alerts,
      notify_purchase_receipts: settings.notify_purchase_receipts,
      notify_live_log: settings.notify_live_log,
      notify_quota_warnings: settings.notify_quota_warnings,
      notify_critical_alerts: settings.notify_critical_alerts,
      notify_account_status: settings.notify_account_status,
      otp_enabled: settings.otp_enabled,
    })
  }, [settings])

  const updateMutation = useMutation({
    mutationFn: updateBotSettings,
    onSuccess: () => {
      toast.success('تنظیمات ربات تلگرام ذخیره شد.')
      queryClient.invalidateQueries({ queryKey: ['bot_settings'] })
      setBotToken('')
      setSocks5Password('')
    },
    onError: (err) =>
      toast.error(backendErrorMessage(err, 'ذخیره تنظیمات ربات ناموفق بود.')),
  })

  const handleSave = () => {
    const req: UpdateBotSettingsRequest = {
      admin_chat_id: adminChatId,
      enabled,
      socks5_enabled: socks5Enabled,
      socks5_address: socks5Address,
      socks5_username: socks5Username,
      faoxima_domain: faoximaDomain,
      ...toggles,
    }
    // Only send bot_token if the admin actually typed a new one.
    if (botToken.trim().length > 0) {
      req.bot_token = botToken.trim()
    }
    // Same "only send if actually typed" rule as bot_token above --
    // otherwise saving the form without touching the password field would
    // silently blank out an already-set SOCKS5 password.
    if (socks5Password.trim().length > 0) {
      req.socks5_password = socks5Password.trim()
    }
    updateMutation.mutate(req)
  }

  const handleToggleChange = (key: string, value: boolean) => {
    setToggles((prev) => ({ ...prev, [key]: value }))
  }

  return (
    <Card>
      <CardHeader className='flex flex-row items-center gap-3 pb-2'>
        <IconBrandTelegram className='h-5 w-5' />
        <div>
          <CardTitle className='text-lg'>ربات تلگرام</CardTitle>
          <CardDescription>
            یک ربات تلگرام را برای اعلان‌ها و مدیریت از راه دور متصل کنید.
          </CardDescription>
        </div>
      </CardHeader>
      <CardContent className='space-y-6'>
        {isLoading ? (
          <p className='text-muted-foreground text-sm'>در حال بارگذاری...</p>
        ) : (
          <>
            <div className='flex items-center justify-between rounded-lg border p-3'>
              <div>
                <Label htmlFor='bot-enabled' className='text-sm font-medium'>
                  فعال‌سازی ربات
                </Label>
                <p className='text-muted-foreground text-xs'>
                  کلید اصلی. در حالت خاموش، هیچ پیامی ارسال و هیچ دستوری پردازش نمی‌شود.
                </p>
              </div>
              <Switch
                id='bot-enabled'
                checked={enabled}
                onCheckedChange={setEnabled}
              />
            </div>

            <div className='space-y-2'>
              <Label htmlFor='bot-token'>توکن ربات (Bot Token)</Label>
              <Input
                id='bot-token'
                type='password'
                value={botToken}
                onChange={(e) => setBotToken(e.target.value)}
                placeholder={
                  settings?.bot_token_set
                    ? `در حال حاضر تنظیم شده (${settings.bot_token_masked}) — برای حفظ مقدار فعلی خالی بگذارید`
                    : 'یک توکن از @BotFather وارد کنید'
                }
              />
            </div>

            <div className='space-y-2'>
              <Label htmlFor='admin-chat-id'>چت آیدی مدیر (Admin Chat ID)</Label>
              <Input
                id='admin-chat-id'
                value={adminChatId}
                onChange={(e) => setAdminChatId(e.target.value)}
                placeholder='چت آیدی شخصی شما در تلگرام'
              />
              <p className='text-muted-foreground text-xs'>
                برای یافتن chat_id خود، در تلگرام به @userinfobot پیام دهید.
              </p>
            </div>

            <Separator />

            <div className='space-y-3'>
              <p className='text-sm font-medium'>اعلان‌ها</p>
              {toggleDefs.map((def) => (
                <div
                  key={def.key}
                  className='flex items-center justify-between gap-4'
                >
                  <div>
                    <Label htmlFor={`toggle-${def.key}`} className='text-sm'>
                      {def.label}
                    </Label>
                    <p className='text-muted-foreground text-xs'>{def.description}</p>
                  </div>
                  <Switch
                    id={`toggle-${def.key}`}
                    checked={!!toggles[def.key]}
                    onCheckedChange={(value) => handleToggleChange(def.key, value)}
                  />
                </div>
              ))}
            </div>

            <Separator />

            <div className='space-y-3'>
              <div>
                <p className='text-sm font-medium'>گیرندگان اضافی</p>
                <p className='text-muted-foreground text-xs'>
                  چت‌های تلگرامی اضافی که علاوه بر چت آیدی مدیر بالا، هر
                  اعلان مدیر (از جمله کدهای یکبار مصرف) را دریافت می‌کنند.
                </p>
              </div>

              {extraAdmins.length > 0 && (
                <div className='space-y-2'>
                  {extraAdmins.map((extra) => (
                    <div
                      key={extra.id}
                      className='flex items-center justify-between gap-2 rounded-lg border p-2'
                    >
                      <div className='text-sm'>
                        <span className='font-mono'>{extra.chat_id}</span>
                        {extra.label && (
                          <span className='text-muted-foreground'> ({extra.label})</span>
                        )}
                      </div>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='text-destructive h-8 w-8'
                        onClick={() => removeExtraAdminMutation.mutate(extra.id)}
                      >
                        <IconTrash className='h-4 w-4' />
                      </Button>
                    </div>
                  ))}
                </div>
              )}

              <div className='flex gap-2'>
                <Input
                  placeholder='چت آیدی'
                  value={newExtraChatId}
                  onChange={(e) => setNewExtraChatId(e.target.value)}
                  className='flex-1'
                />
                <Input
                  placeholder='برچسب (اختیاری)'
                  value={newExtraLabel}
                  onChange={(e) => setNewExtraLabel(e.target.value)}
                  className='flex-1'
                />
                <Button
                  variant='outline'
                  size='icon'
                  onClick={handleAddExtraAdmin}
                  disabled={addExtraAdminMutation.isPending}
                >
                  <IconPlus className='h-4 w-4' />
                </Button>
              </div>
            </div>

            <Separator />

            <div className='space-y-3'>
              <div>
                <p className='text-sm font-medium'>دامنه‌ی ربات ایکس (Bot X)</p>
                <p className='text-muted-foreground text-xs'>
                  دامنه‌ی عمومی که وبهوک تلگرام و لینک مینی‌اپ هر نمونه از
                  ربات فروش اختصاصی نماینده («ربات ایکس») روی آن ثبت
                  می‌شود. این دامنه متعلق به خودِ شماست و باید پشت یک
                  CDN/شبکه‌ی توزیع (مثل Cloudflare) قرار داشته باشد تا
                  دریافت وبهوک تلگرام پایدار بماند. بدون تنظیم این مقدار،
                  راه‌اندازی ربات‌های جدید نمایندگان با خطا مواجه می‌شود.
                </p>
              </div>
              <div className='space-y-2'>
                <Label htmlFor='faoxima-domain'>دامنه</Label>
                <Input
                  id='faoxima-domain'
                  value={faoximaDomain}
                  onChange={(e) => setFaoximaDomain(e.target.value)}
                  placeholder='bot.example.com'
                  dir='ltr'
                />
              </div>
            </div>

            <Separator />

            <div className='space-y-3'>
              <div>
                <p className='text-sm font-medium'>پروکسی SOCKS5</p>
                <p className='text-muted-foreground text-xs'>
                  برای پنلی که روی سروری با فیلترینگ شبکه تلگرام (مثلاً میزبانی‌شده
                  در ایران) نصب شده -- اتصال ربات به تلگرام را به‌جای اتصال
                  مستقیم، از طریق این پروکسی مسیردهی می‌کند. هم برای ربات این
                  پنل و هم برای سرویس ربات هر نماینده اعمال می‌شود.
                </p>
                <p className='text-muted-foreground text-xs'>
                  نکته برای ربات‌های «ایکس» (سرویس ربات اختصاصی هر نماینده):
                  اگر این پروکسی هنوز تنظیم نشده باشد، مقداری که در متغیر
                  محیطی <code>FAOXIMA_DEFAULT_TELEGRAM_PROXY</code> سرور
                  (فایل <code>.env</code>) تعریف شده به‌عنوان پیش‌فرض برای
                  ربات‌های تازه‌ساز استفاده می‌شود. توصیه می‌شود به‌جای IP
                  مستقیم، یک دامنه ثبت‌شده روی سرویسی مثل Cloudflare برای این
                  پروکسی اختصاص دهید تا در صورت نیاز به تغییر IP در آینده،
                  نیازی به تنظیم مجدد ربات‌های قبلاً ساخته‌شده نباشد.
                </p>
              </div>

              <div className='flex items-center justify-between rounded-lg border p-3'>
                <Label htmlFor='socks5-enabled' className='text-sm font-medium'>
                  فعال‌سازی پروکسی SOCKS5
                </Label>
                <Switch
                  id='socks5-enabled'
                  checked={socks5Enabled}
                  onCheckedChange={setSocks5Enabled}
                />
              </div>

              <div className='space-y-2'>
                <Label htmlFor='socks5-address'>آدرس پروکسی</Label>
                <Input
                  id='socks5-address'
                  value={socks5Address}
                  onChange={(e) => {
                    setSocks5Address(e.target.value)
                    setSocks5TestResult(null)
                  }}
                  placeholder='127.0.0.1:1080'
                />
              </div>

              <div className='grid grid-cols-2 gap-2'>
                <div className='space-y-2'>
                  <Label htmlFor='socks5-username'>نام کاربری (اختیاری)</Label>
                  <Input
                    id='socks5-username'
                    value={socks5Username}
                    onChange={(e) => {
                      setSocks5Username(e.target.value)
                      setSocks5TestResult(null)
                    }}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='socks5-password'>رمز عبور (اختیاری)</Label>
                  <Input
                    id='socks5-password'
                    type='password'
                    value={socks5Password}
                    onChange={(e) => {
                      setSocks5Password(e.target.value)
                      setSocks5TestResult(null)
                    }}
                    placeholder={
                      settings?.socks5_password_set
                        ? 'در حال حاضر تنظیم شده — برای حفظ مقدار فعلی خالی بگذارید'
                        : ''
                    }
                  />
                </div>
              </div>

              <div className='flex items-center gap-3'>
                <Button
                  variant='outline'
                  size='sm'
                  onClick={handleTestSocks5}
                  disabled={testSocks5Mutation.isPending || !socks5Address.trim()}
                >
                  <IconPlayerPlay className='h-4 w-4 me-1' />
                  {testSocks5Mutation.isPending
                    ? 'در حال تست اتصال...'
                    : 'تست اتصال'}
                </Button>

                {socks5TestResult && !testSocks5Mutation.isPending && (
                  <div
                    className={`flex items-center gap-1.5 text-sm ${
                      socks5TestResult.connected
                        ? 'text-emerald-600 dark:text-emerald-400'
                        : 'text-destructive'
                    }`}
                  >
                    {socks5TestResult.connected ? (
                      <>
                        <IconCircleCheck className='h-4 w-4' />
                        <span>
                          متصل شد — پینگ {socks5TestResult.ping_ms} میلی‌ثانیه
                        </span>
                      </>
                    ) : (
                      <>
                        <IconCircleX className='h-4 w-4' />
                        <span>
                          {socks5TestResult.error || 'اتصال برقرار نشد.'}
                        </span>
                      </>
                    )}
                  </div>
                )}
              </div>
            </div>

            <Button
              onClick={handleSave}
              disabled={updateMutation.isPending}
              className='w-full'
            >
              {updateMutation.isPending ? 'در حال ذخیره...' : 'ذخیره تنظیمات'}
            </Button>

            {settings?.auto_backup_enabled && (
              <p className='text-muted-foreground text-xs'>
                پشتیبان‌گیری روزانه خودکار برای ساعت{' '}
                {String(settings.auto_backup_hour).padStart(2, '0')}:
                {String(settings.auto_backup_minute).padStart(2, '0')} UTC
                زمان‌بندی شده است (تنظیم‌شده از طریق دستور /setbackup ربات).
              </p>
            )}

            {settings?.auto_report_enabled && (
              <p className='text-muted-foreground text-xs'>
                گزارش مصرف روزانه خودکار برای ساعت{' '}
                {String(settings.auto_report_hour).padStart(2, '0')}:
                {String(settings.auto_report_minute).padStart(2, '0')} UTC
                زمان‌بندی شده است (تنظیم‌شده از طریق دستور /setreport ربات).
              </p>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}
