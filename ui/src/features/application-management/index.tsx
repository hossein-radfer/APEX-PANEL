import { useState } from 'react'
import { AppVersion } from '@/schema/application.ts'
import { useAppVersionsQuery } from '@/hooks/applications/useAppVersionsQuery.ts'
import { usePublishAppVersionMutation } from '@/hooks/applications/usePublishAppVersionMutation.ts'
import { useDeleteAppVersionMutation } from '@/hooks/applications/useDeleteAppVersionMutation.ts'
import { useAppMaintenanceModeQuery } from '@/hooks/applications/useAppMaintenanceModeQuery.ts'
import { useSetAppMaintenanceModeMutation } from '@/hooks/applications/useSetAppMaintenanceModeMutation.ts'
import { TrashIcon } from 'lucide-react'
import { toast } from 'sonner'
import { getApiErrorMessage } from '@/lib/api-error.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

// MaintenanceModeCard toggles the GLOBAL gate that blocks every app
// instance -- including one already running the latest version --
// independent of the mandatory-update mechanism below. See the
// backend's own ApplicationVersionService.SetMaintenanceMode doc
// comment: this is meant for e.g. a server migration window, not a
// substitute for marking a bad release mandatory.
function MaintenanceModeCard() {
  const { data: mode, isLoading } = useAppMaintenanceModeQuery()
  const { mutateAsync: setMode, isPending } =
    useSetAppMaintenanceModeMutation()
  const [message, setMessage] = useState('')
  const [messageTouched, setMessageTouched] = useState(false)

  const effectiveMessage = messageTouched ? message : (mode?.message ?? '')

  const handleToggle = async (enabled: boolean) => {
    try {
      await setMode({ enabled, message: effectiveMessage || null })
      toast.success(
        enabled
          ? 'حالت نگهداری فعال شد -- اپلیکیشن برای همه‌ی کاربران مسدود است.'
          : 'حالت نگهداری غیرفعال شد.',
        { duration: 5000 }
      )
    } catch (error) {
      toast.error(getApiErrorMessage(error, 'تغییر حالت نگهداری ناموفق بود.'))
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className='text-lg'>حالت نگهداری (Maintenance)</CardTitle>
      </CardHeader>
      <CardContent className='space-y-4'>
        <p className='text-muted-foreground text-sm'>
          وقتی فعال باشد، هیچ نسخه‌ای از اپلیکیشن (حتی جدیدترین نسخه) قابل
          استفاده نیست -- برای زمانی که در حال انجام تغییرات روی سرور
          هستید و نمی‌خواهید کاربران در این بازه از برنامه استفاده کنند.
        </p>

        {isLoading ? (
          <Skeleton className='h-24 w-full rounded-lg' />
        ) : (
          <>
            <div className='flex items-center gap-3'>
              <Checkbox
                id='maintenance-enabled'
                checked={mode?.enabled ?? false}
                disabled={isPending}
                onCheckedChange={(checked) => handleToggle(Boolean(checked))}
              />
              <Label htmlFor='maintenance-enabled' className='font-normal'>
                حالت نگهداری فعال باشد
              </Label>
              {mode?.enabled && <Badge variant='destructive'>فعال</Badge>}
            </div>

            <div className='space-y-2'>
              <Label htmlFor='maintenance-message'>
                پیام نمایش‌داده‌شده به کاربر (اختیاری)
              </Label>
              <Textarea
                id='maintenance-message'
                placeholder='مثلاً: سرویس تا ساعت ۲۲ امشب در حال بروزرسانی است.'
                value={effectiveMessage}
                onChange={(e) => {
                  setMessage(e.target.value)
                  setMessageTouched(true)
                }}
                onBlur={() => {
                  if (mode?.enabled) {
                    handleToggle(true)
                  }
                }}
              />
            </div>
          </>
        )}
      </CardContent>
    </Card>
  )
}

function PublishVersionCard() {
  const [versionCode, setVersionCode] = useState('')
  const [versionName, setVersionName] = useState('')
  const [releaseNotes, setReleaseNotes] = useState('')
  const [downloadUrl, setDownloadUrl] = useState('')
  const [isMandatory, setIsMandatory] = useState(false)
  const { mutateAsync: publish, isPending } = usePublishAppVersionMutation()

  const handlePublish = async () => {
    const code = Number(versionCode)
    if (!Number.isFinite(code) || code <= 0) {
      toast.error('کد نسخه باید یک عدد مثبت باشد.', { duration: 5000 })
      return
    }
    if (!versionName.trim()) {
      toast.error('نام نسخه الزامی است.', { duration: 5000 })
      return
    }
    if (!downloadUrl.trim()) {
      toast.error('لینک دانلود الزامی است.', { duration: 5000 })
      return
    }

    try {
      await publish({
        version_code: code,
        version_name: versionName.trim(),
        release_notes: releaseNotes.trim() || null,
        download_url: downloadUrl.trim(),
        is_mandatory: isMandatory,
      })
      toast.success('نسخه‌ی جدید منتشر شد.', { duration: 5000 })
      setVersionCode('')
      setVersionName('')
      setReleaseNotes('')
      setDownloadUrl('')
      setIsMandatory(false)
    } catch (error) {
      toast.error(getApiErrorMessage(error, 'انتشار نسخه ناموفق بود.'))
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className='text-lg'>انتشار نسخه‌ی جدید</CardTitle>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='grid grid-cols-1 gap-x-3 gap-y-4 md:grid-cols-2'>
          <div className='space-y-2'>
            <Label htmlFor='version-code'>کد نسخه (عدد صعودی)</Label>
            <Input
              id='version-code'
              type='number'
              step='1'
              min='1'
              placeholder='مثلاً ۱۰'
              value={versionCode}
              onChange={(e) => setVersionCode(e.target.value)}
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='version-name'>نام نسخه (نمایشی)</Label>
            <Input
              id='version-name'
              placeholder='مثلاً ۲.۴.۱'
              value={versionName}
              onChange={(e) => setVersionName(e.target.value)}
            />
          </div>
        </div>

        <div className='space-y-2'>
          <Label htmlFor='download-url'>لینک دانلود</Label>
          <Input
            id='download-url'
            placeholder='لینک مستقیم فایل APK یا صفحه‌ی دانلود'
            value={downloadUrl}
            onChange={(e) => setDownloadUrl(e.target.value)}
          />
        </div>

        <div className='space-y-2'>
          <Label htmlFor='release-notes'>توضیحات نسخه (اختیاری)</Label>
          <Textarea
            id='release-notes'
            placeholder='چه چیزی در این نسخه تغییر کرده؟'
            value={releaseNotes}
            onChange={(e) => setReleaseNotes(e.target.value)}
          />
        </div>

        <div className='flex items-center gap-3'>
          <Checkbox
            id='is-mandatory'
            checked={isMandatory}
            onCheckedChange={(checked) => setIsMandatory(Boolean(checked))}
          />
          <Label htmlFor='is-mandatory' className='font-normal'>
            نصب این نسخه اجباری باشد (نسخه‌های قدیمی‌تر مسدود می‌شوند)
          </Label>
        </div>

        <Button onClick={handlePublish} disabled={isPending}>
          {isPending ? 'در حال انتشار...' : 'انتشار نسخه'}
        </Button>
      </CardContent>
    </Card>
  )
}

function VersionsListCard() {
  const { data: versions = [], isLoading } = useAppVersionsQuery()
  const { mutateAsync: remove } = useDeleteAppVersionMutation()
  const [pendingDelete, setPendingDelete] = useState<AppVersion | null>(null)

  const handleConfirmDelete = async () => {
    if (!pendingDelete) return
    try {
      await remove(pendingDelete.id)
      toast.success('نسخه حذف شد.', { duration: 3000 })
    } catch (error) {
      toast.error(getApiErrorMessage(error, 'حذف نسخه ناموفق بود.'))
    } finally {
      setPendingDelete(null)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className='text-lg'>نسخه‌های منتشرشده</CardTitle>
      </CardHeader>
      <CardContent className='space-y-3'>
        {isLoading ? (
          <Skeleton className='h-32 w-full rounded-lg' />
        ) : versions.length === 0 ? (
          <p className='text-muted-foreground text-sm'>
            هنوز هیچ نسخه‌ای منتشر نشده است.
          </p>
        ) : (
          versions.map((version) => (
            <div
              key={version.id}
              className='flex items-start justify-between gap-3 rounded-lg border p-3'
            >
              <div className='space-y-1'>
                <div className='flex items-center gap-2'>
                  <span className='font-medium'>
                    نسخه {version.version_name}
                  </span>
                  <Badge variant='secondary'>
                    کد {version.version_code}
                  </Badge>
                  {version.is_mandatory && (
                    <Badge variant='destructive'>اجباری</Badge>
                  )}
                </div>
                {version.release_notes && (
                  <p className='text-muted-foreground text-sm'>
                    {version.release_notes}
                  </p>
                )}
                <p className='text-muted-foreground text-xs'>
                  منتشر شده: {new Date(version.published_at).toLocaleString('fa-IR')}
                </p>
              </div>
              <Button
                variant='ghost'
                size='icon'
                onClick={() => setPendingDelete(version)}
              >
                <TrashIcon className='h-4 w-4 text-red-600' />
              </Button>
            </div>
          ))
        )}
      </CardContent>

      <AlertDialog
        open={pendingDelete !== null}
        onOpenChange={(open) => !open && setPendingDelete(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>حذف نسخه</AlertDialogTitle>
            <AlertDialogDescription>
              آیا از حذف نسخه «{pendingDelete?.version_name}» مطمئن هستید؟
              کاربرانی که در حال حاضر همین نسخه را نصب دارند تحت تأثیر قرار
              نمی‌گیرند -- این عمل فقط این نسخه را از فهرست «آخرین نسخه»
              و از زنجیره‌ی به‌روزرسانی اجباری حذف می‌کند.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>انصراف</AlertDialogCancel>
            <AlertDialogAction onClick={handleConfirmDelete}>
              حذف
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  )
}

export default function ApplicationManagement() {
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
            مدیریت اپلیکیشن
          </h2>
          <p className='text-muted-foreground'>
            انتشار نسخه‌های جدید اپلیکیشن موبایل، تعیین اجباری‌بودن
            به‌روزرسانی، و کنترل حالت نگهداری سرویس. این بخش فقط برای
            مدیر قابل مشاهده است.
          </p>
        </div>

        <div className='space-y-4'>
          <MaintenanceModeCard />
          <PublishVersionCard />
          <VersionsListCard />
        </div>
      </Main>
    </>
  )
}
