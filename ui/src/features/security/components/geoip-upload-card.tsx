import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { IconDatabase, IconRefresh, IconTrash, IconUpload } from '@tabler/icons-react'
import {
  clearGeoIPCache,
  deleteGeoIPASN,
  deleteGeoIPCity,
  fetchGeoIPMode,
  fetchGeoIPStatus,
  setGeoIPMode,
  uploadGeoIPASN,
  uploadGeoIPCity,
} from '@/api/security.ts'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { Switch } from '@/components/ui/switch'

function backendErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof AxiosError) {
    const message = err.response?.data?.message
    if (typeof message === 'string' && message.length > 0) return message
  }
  return fallback
}

function GeoIPSlot({
  title,
  description,
  loaded,
  onUpload,
  onDelete,
  isMutating,
  progress,
}: {
  title: string
  description: string
  loaded: boolean
  onUpload: (file: File) => void
  onDelete: () => void
  isMutating: boolean
  progress: number | null
}) {
  const fileInputRef = useRef<HTMLInputElement>(null)

  return (
    <div className='space-y-3 rounded-lg border p-4'>
      <div className='flex items-center justify-between'>
        <div>
          <p className='font-medium'>{title}</p>
          <p className='text-muted-foreground text-xs'>{description}</p>
        </div>
        <Badge variant={loaded ? 'default' : 'outline'}>
          {loaded ? 'بارگذاری‌شده' : 'بارگذاری‌نشده'}
        </Badge>
      </div>

      <input
        ref={fileInputRef}
        type='file'
        accept='.mmdb'
        className='hidden'
        onChange={(e) => {
          const file = e.target.files?.[0]
          if (file) onUpload(file)
          if (fileInputRef.current) fileInputRef.current.value = ''
        }}
      />

      {progress !== null && (
        <div className='space-y-1'>
          <Progress value={progress} className='h-2' />
          <p className='text-muted-foreground text-xs'>{progress}٪ آپلود شد</p>
        </div>
      )}

      <div className='flex gap-2'>
        <Button
          variant='outline'
          size='sm'
          className='gap-2'
          disabled={isMutating}
          onClick={() => fileInputRef.current?.click()}
        >
          <IconUpload className='h-4 w-4' />
          {loaded ? 'جایگزینی فایل' : 'آپلود فایل'}
        </Button>
        {loaded && (
          <Button
            variant='outline'
            size='sm'
            className='text-destructive gap-2'
            disabled={isMutating}
            onClick={onDelete}
          >
            <IconTrash className='h-4 w-4' />
            حذف
          </Button>
        )}
      </div>
    </div>
  )
}

export function GeoIPUploadCard() {
  const queryClient = useQueryClient()
  const { data: status, isLoading } = useQuery({
    queryKey: ['geoip_status'],
    queryFn: fetchGeoIPStatus,
  })
  const { data: mode, isLoading: isModeLoading } = useQuery({
    queryKey: ['geoip_mode'],
    queryFn: fetchGeoIPMode,
  })

  const [cityProgress, setCityProgress] = useState<number | null>(null)
  const [asnProgress, setAsnProgress] = useState<number | null>(null)

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ['geoip_status'] })

  // A successful upload/cache-clear makes the backend re-resolve every
  // currently-open connection's geo data (see SecurityController's own
  // refreshGeoDataAfterUpload/ClearGeoIPCache doc comments) -- invalidating
  // these two here too means the Users/Ether Traffic tables reflect that
  // fix immediately, instead of showing the same stale "—" rows until the
  // admin happens to refresh the whole page.
  const invalidateGeoDependentViews = () => {
    invalidate()
    queryClient.invalidateQueries({ queryKey: ['security_identities'] })
    queryClient.invalidateQueries({ queryKey: ['security_ether_traffic'] })
  }

  // A cache entry resolved by one mode (offline .mmdb vs. the online API)
  // is never re-resolved by the other until cleared -- see
  // GeoIPService.Lookup's own "cache hit never touches either path" rule --
  // so flipping this switch alone would silently keep serving whatever the
  // PREVIOUS mode already cached. Clearing the cache on every mode change
  // (not just on upload, matching the existing "پاک‌سازی کش" button's own
  // invalidateGeoDependentViews call) makes the switch's effect immediate
  // and visible, instead of appearing to do nothing until an admin also
  // remembers to clear the cache separately.
  const setModeMutation = useMutation({
    mutationFn: setGeoIPMode,
    onSuccess: async (_data, onlineEnabled) => {
      await clearGeoIPCache()
      toast.success(
        onlineEnabled
          ? 'حالت آنلاین فعال شد؛ استعلام‌های بعدی از سرویس آنلاین انجام می‌شوند.'
          : 'حالت آفلاین فعال شد؛ استعلام‌های بعدی از فایل‌های محلی انجام می‌شوند.'
      )
      queryClient.invalidateQueries({ queryKey: ['geoip_mode'] })
      invalidateGeoDependentViews()
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'تغییر حالت استعلام ناموفق بود.')),
  })

  const uploadCityMutation = useMutation({
    mutationFn: (file: File) =>
      uploadGeoIPCity(file, setCityProgress),
    onSuccess: () => {
      toast.success('فایل GeoLite2-City با موفقیت بارگذاری شد.')
      setCityProgress(null)
      invalidateGeoDependentViews()
    },
    onError: (err) => {
      toast.error(backendErrorMessage(err, 'بارگذاری فایل City ناموفق بود.'))
      setCityProgress(null)
    },
  })

  const uploadAsnMutation = useMutation({
    mutationFn: (file: File) => uploadGeoIPASN(file, setAsnProgress),
    onSuccess: () => {
      toast.success('فایل GeoLite2-ASN با موفقیت بارگذاری شد.')
      setAsnProgress(null)
      invalidateGeoDependentViews()
    },
    onError: (err) => {
      toast.error(backendErrorMessage(err, 'بارگذاری فایل ASN ناموفق بود.'))
      setAsnProgress(null)
    },
  })

  const clearCacheMutation = useMutation({
    mutationFn: clearGeoIPCache,
    onSuccess: () => {
      toast.success('کش GeoIP پاک شد و اطلاعات کاربران متصل به‌روزرسانی شد.')
      invalidateGeoDependentViews()
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'پاک‌سازی کش ناموفق بود.')),
  })

  const deleteCityMutation = useMutation({
    mutationFn: deleteGeoIPCity,
    onSuccess: () => {
      toast.success('فایل City حذف شد.')
      invalidate()
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'حذف فایل ناموفق بود.')),
  })

  const deleteAsnMutation = useMutation({
    mutationFn: deleteGeoIPASN,
    onSuccess: () => {
      toast.success('فایل ASN حذف شد.')
      invalidate()
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'حذف فایل ناموفق بود.')),
  })

  return (
    <Card>
      <CardHeader className='flex flex-row items-center gap-3 pb-2'>
        <IconDatabase className='h-5 w-5' />
        <div className='flex-1'>
          <CardTitle className='text-lg'>پایگاه‌داده GeoIP</CardTitle>
          <CardDescription>
            فایل‌های GeoLite2-City.mmdb و GeoLite2-ASN.mmdb را برای استعلام
            موقعیت مکانی و اپراتور آی‌پی‌ها بارگذاری کنید.
          </CardDescription>
        </div>
        <Button
          variant='outline'
          size='sm'
          className='gap-2'
          disabled={clearCacheMutation.isPending}
          onClick={() => clearCacheMutation.mutate()}
          title='کش استعلام‌های قبلی را پاک کرده و کاربران متصل را دوباره استعلام می‌کند'
        >
          <IconRefresh className='h-4 w-4' />
          پاک‌سازی کش
        </Button>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='flex items-center justify-between rounded-lg border p-4'>
          <div>
            <Label htmlFor='geoip-online-mode' className='font-medium'>
              حالت استعلام آنلاین
            </Label>
            <p className='text-muted-foreground text-xs'>
              روشن: استعلام از یک سرویس آنلاین انجام می‌شود (نیاز به فایل
              محلی نیست). خاموش: فقط از فایل‌های آپلودشده‌ی زیر استفاده
              می‌شود.
            </p>
          </div>
          <Switch
            id='geoip-online-mode'
            checked={mode?.online_enabled ?? false}
            disabled={isModeLoading || setModeMutation.isPending}
            onCheckedChange={(checked) => setModeMutation.mutate(checked)}
          />
        </div>

        <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
          {isLoading ? (
            <p className='text-muted-foreground text-sm'>در حال بارگذاری...</p>
          ) : (
            <>
              <GeoIPSlot
                title='فایل City (کشور، استان، شهر، مختصات، تایم‌زون)'
                description='GeoLite2-City.mmdb را اینجا وارد کنید'
                loaded={status?.city_loaded ?? false}
                onUpload={(file) => uploadCityMutation.mutate(file)}
                onDelete={() => deleteCityMutation.mutate()}
                isMutating={uploadCityMutation.isPending || deleteCityMutation.isPending}
                progress={cityProgress}
              />
              <GeoIPSlot
                title='فایل ASN (شماره ASN و نام اپراتور)'
                description='GeoLite2-ASN.mmdb را اینجا وارد کنید'
                loaded={status?.asn_loaded ?? false}
                onUpload={(file) => uploadAsnMutation.mutate(file)}
                onDelete={() => deleteAsnMutation.mutate()}
                isMutating={uploadAsnMutation.isPending || deleteAsnMutation.isPending}
                progress={asnProgress}
              />
            </>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
