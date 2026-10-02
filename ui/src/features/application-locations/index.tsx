import { useEffect, useMemo, useRef, useState } from 'react'
import { useInterfacesListQuery } from '@/hooks/interfaces/useInterfacesListQuery.ts'
import { useUserManagerGroupsQuery } from '@/hooks/user-manager/useUserManagerGroupsQuery.ts'
import { useXuiPanelsListQuery } from '@/hooks/xui-panel/useXuiPanelsListQuery.ts'
import { useApplicationOpenVpnTemplateStatusQuery } from '@/hooks/applications/useApplicationOpenVpnTemplateStatusQuery.ts'
import { useApplicationResourceLocationsQuery } from '@/hooks/applications/useApplicationResourceLocationsQuery.ts'
import { useSetApplicationResourceLocationMutation } from '@/hooks/applications/useSetApplicationResourceLocationMutation.ts'
import { useUploadApplicationOpenVpnTemplateMutation } from '@/hooks/applications/useUploadApplicationOpenVpnTemplateMutation.ts'
import { UploadIcon } from 'lucide-react'
import { toast } from 'sonner'
import { getApiErrorMessage } from '@/lib/api-error.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { EmptyState } from '@/components/ui/empty-state.tsx'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'

interface ResourceRow {
  resourceType: 'wireguard_interface' | 'user_manager_group' | 'xui_panel'
  resourceKey: string
  name: string
}

function LocationSection({
  title,
  rows,
  labelsByKey,
  onSave,
  isLoading,
}: {
  title: string
  rows: ResourceRow[]
  labelsByKey: Map<string, string>
  onSave: (row: ResourceRow, label: string) => void
  isLoading: boolean
}) {
  const [drafts, setDrafts] = useState<Record<string, string>>({})

  // Only ever ADDS keys that don't exist in `drafts` yet (a newly-
  // appeared resource) -- never overwrites a key already present, even
  // if `labelsByKey`/`rows` change reference again later (e.g. from an
  // unrelated background refetch). See the memoization fix above (in
  // ApplicationLocations) for why references used to change on every
  // render in the first place; this second layer protects against the
  // remaining, legitimate cases where they still can (a real save
  // completing, a genuinely new resource appearing) without that ever
  // clobbering text the admin is actively mid-typing in a DIFFERENT
  // field, or even the same field between keystrokes.
  useEffect(() => {
    setDrafts((prev) => {
      let changed = false
      const next = { ...prev }
      for (const row of rows) {
        const key = `${row.resourceType}:${row.resourceKey}`
        if (!(key in next)) {
          next[key] = labelsByKey.get(key) ?? ''
          changed = true
        }
      }
      return changed ? next : prev
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows, labelsByKey])

  return (
    <Card>
      <CardHeader>
        <CardTitle className='text-lg'>{title}</CardTitle>
      </CardHeader>
      <CardContent className='space-y-3'>
        {isLoading ? (
          <Skeleton className='h-32 w-full rounded-lg' />
        ) : rows.length === 0 ? (
          <EmptyState message='موردی یافت نشد.' />
        ) : (
          rows.map((row) => {
            const key = `${row.resourceType}:${row.resourceKey}`
            return (
              <div key={key} className='flex items-center gap-3'>
                <span className='w-40 shrink-0 text-sm font-medium'>
                  {row.name}
                </span>
                <Input
                  placeholder='مثلاً: آلمان - فرانکفورت'
                  value={drafts[key] ?? ''}
                  onChange={(e) =>
                    setDrafts((prev) => ({ ...prev, [key]: e.target.value }))
                  }
                  onBlur={() => {
                    const value = drafts[key] ?? ''
                    if (value !== (labelsByKey.get(key) ?? '') && value.trim()) {
                      onSave(row, value.trim())
                    }
                  }}
                />
              </div>
            )
          })
        )}
      </CardContent>
    </Card>
  )
}

// OpenVpnTemplateCard lets the admin upload the ONE global .ovpn
// template every Application's OpenVPN connect-config is combined from
// -- see ApplicationOpenVpnTemplateService's own doc comment (backend)
// for why this is a single shared file (one physical OpenVPN server;
// only the per-group username/password differ, and those already come
// from each group's own auto-created UserManagerAccount).
function OpenVpnTemplateCard() {
  const fileInputRef = useRef<HTMLInputElement>(null)
  const { data: status, isLoading } = useApplicationOpenVpnTemplateStatusQuery()
  const { mutateAsync: uploadTemplate, isPending: isUploading } =
    useUploadApplicationOpenVpnTemplateMutation()

  const handleFileSelected = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    try {
      await uploadTemplate(file)
      toast.success('قالب OpenVPN با موفقیت آپلود شد.', { duration: 5000 })
    } catch (error) {
      toast.error(getApiErrorMessage(error, 'آپلود قالب ناموفق بود.'))
    } finally {
      e.target.value = ''
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className='text-lg'>قالب سراسری OpenVPN</CardTitle>
      </CardHeader>
      <CardContent className='space-y-3'>
        <p className='text-muted-foreground text-sm'>
          یک فایل .ovpn پایه (شامل آدرس/پورت سرور و گواهی) آپلود کنید --
          این فایل بین همه‌ی اپلیکیشن‌ها مشترک است؛ فقط یوزرنیم/پسورد هر
          گروه (که به‌صورت خودکار ساخته می‌شود) به آن اضافه می‌شود. این
          قالب نباید خودش شامل یوزرنیم/پسورد باشد.
        </p>

        {isLoading ? (
          <Skeleton className='h-9 w-full rounded-lg' />
        ) : (
          <>
            <input
              ref={fileInputRef}
              type='file'
              accept='.ovpn'
              className='hidden'
              onChange={handleFileSelected}
            />
            <Button
              variant='outline'
              disabled={isUploading}
              onClick={() => fileInputRef.current?.click()}
            >
              <UploadIcon className='ml-2 h-4 w-4' />
              {isUploading
                ? 'در حال آپلود...'
                : status?.exists
                  ? 'جایگزینی قالب'
                  : 'آپلود قالب'}
            </Button>
            {status?.exists ? (
              <p className='text-muted-foreground text-sm'>
                یک قالب آپلود شده است
                {status.uploaded_at &&
                  ` (آخرین بروزرسانی: ${new Date(status.uploaded_at).toLocaleString('fa-IR')})`}
                .
              </p>
            ) : (
              <p className='text-muted-foreground text-sm'>
                هنوز هیچ قالبی آپلود نشده است -- تا زمانی که آپلود نشود،
                اپلیکیشن موبایل قادر به اتصال از طریق OpenVPN نخواهد بود.
              </p>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}

export default function ApplicationLocations() {
  const { data: interfaces = [], isLoading: isInterfacesLoading } =
    useInterfacesListQuery()
  const { data: groups = [], isLoading: isGroupsLoading } =
    useUserManagerGroupsQuery()
  const { data: panels = [], isLoading: isPanelsLoading } =
    useXuiPanelsListQuery()
  const { data: locations, isLoading: isLocationsLoading } =
    useApplicationResourceLocationsQuery()
  const { mutate: setLocation } = useSetApplicationResourceLocationMutation()

  const labelsByKey = useMemo(() => {
    const map = new Map<string, string>()
    for (const loc of locations ?? []) {
      map.set(`${loc.resource_type}:${loc.resource_key}`, loc.label)
    }
    return map
  }, [locations])

  // Memoized -- confirmed, reported bug this fixes: without this, every
  // parent re-render (including ones React Query triggers in the
  // background, e.g. a window-focus refetch) produced a BRAND NEW array
  // reference here, which retriggered LocationSection's own useEffect
  // (keyed on `rows`) and reset `drafts` back to the last-saved server
  // value mid-typing -- destroying whatever the admin was in the middle
  // of entering. Manifested most visibly with emoji/flag characters
  // (multi-codepoint, typically entered more slowly via an IME/picker,
  // so far more likely to still be "in progress" when a stray re-render
  // landed) but was never actually emoji-specific -- any input could be
  // clobbered by an unlucky-timed re-render.
  const interfaceRows: ResourceRow[] = useMemo(
    () =>
      interfaces.map((iface) => ({
        resourceType: 'wireguard_interface' as const,
        resourceKey: String(iface.id),
        name: iface.name,
      })),
    [interfaces]
  )
  const groupRows: ResourceRow[] = useMemo(
    () =>
      groups.map((group) => ({
        resourceType: 'user_manager_group' as const,
        resourceKey: group.name,
        name: group.name,
      })),
    [groups]
  )
  const panelRows: ResourceRow[] = useMemo(
    () =>
      panels.map((panel) => ({
        resourceType: 'xui_panel' as const,
        resourceKey: String(panel.id),
        name: panel.name,
      })),
    [panels]
  )

  const handleSave = (row: ResourceRow, label: string) => {
    setLocation(
      {
        resource_type: row.resourceType,
        resource_key: row.resourceKey,
        label,
      },
      {
        onSuccess: () =>
          toast.success('موقعیت ذخیره شد.', { duration: 3000 }),
        onError: (error) =>
          toast.error(getApiErrorMessage(error, 'ذخیره‌سازی ناموفق بود.')),
      }
    )
  }

  return (
    <>
      <Header fixed>
        <Search />
        <div className='ml-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main>
        <div className='mb-4'>
          <h2 className='text-2xl font-bold tracking-tight'>
            موقعیت اپلیکیشن
          </h2>
          <p className='text-muted-foreground'>
            برای هر اینترفیس وایرگارد، گروه یوزرمنجیر، و پنل V2Ray یک
            برچسب موقعیت (مثلاً نام کشور/شهر) تعیین کنید -- این برچسب فقط
            برای نمایش در اپلیکیشن موبایل استفاده می‌شود.
          </p>
        </div>

        <div className='space-y-4'>
          <OpenVpnTemplateCard />
          <LocationSection
            title='اینترفیس‌های وایرگارد'
            rows={interfaceRows}
            labelsByKey={labelsByKey}
            onSave={handleSave}
            isLoading={isInterfacesLoading || isLocationsLoading}
          />
          <LocationSection
            title='گروه‌های یوزرمنجیر'
            rows={groupRows}
            labelsByKey={labelsByKey}
            onSave={handleSave}
            isLoading={isGroupsLoading || isLocationsLoading}
          />
          <LocationSection
            title='پنل‌های V2Ray'
            rows={panelRows}
            labelsByKey={labelsByKey}
            onSave={handleSave}
            isLoading={isPanelsLoading || isLocationsLoading}
          />
        </div>
      </Main>
    </>
  )
}
