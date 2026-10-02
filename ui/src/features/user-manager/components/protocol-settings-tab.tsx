'use client'

import { useEffect, useRef, useState } from 'react'
import {
  PROTOCOL_CONFIG_PROTOCOLS,
  ProtocolConfigProtocol,
} from '@/schema/user-manager.ts'
import { DownloadIcon, TrashIcon, UploadIcon } from 'lucide-react'
import { toast } from 'sonner'
import {
  fetchUserManagerProtocolCertificateFile,
  fetchUserManagerProtocolClientAppFile,
} from '@/api/user-manager.ts'
import {
  filenameWithFallbackExtension,
  triggerBlobDownload,
} from '@/lib/download.ts'
import { useUserManagerProtocolConfigsQuery } from '@/hooks/user-manager/useUserManagerProtocolConfigsQuery.ts'
import { useUpsertUserManagerProtocolConfigMutation } from '@/hooks/user-manager/useUpsertUserManagerProtocolConfigMutation.ts'
import { useUploadUserManagerProtocolCertificateFileMutation } from '@/hooks/user-manager/useUploadUserManagerProtocolCertificateFileMutation.ts'
import { useUploadUserManagerProtocolClientAppFileMutation } from '@/hooks/user-manager/useUploadUserManagerProtocolClientAppFileMutation.ts'
import { useDeleteUserManagerProtocolCertificateFileMutation } from '@/hooks/user-manager/useDeleteUserManagerProtocolCertificateFileMutation.ts'
import { useDeleteUserManagerProtocolClientAppFileMutation } from '@/hooks/user-manager/useDeleteUserManagerProtocolClientAppFileMutation.ts'
import { Button } from '@/components/ui/button.tsx'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card.tsx'
import { Checkbox } from '@/components/ui/checkbox.tsx'
import { ConfirmDialog } from '@/components/confirm-dialog.tsx'
import { Input } from '@/components/ui/input.tsx'
import { Label } from '@/components/ui/label.tsx'
import { Skeleton } from '@/components/ui/skeleton.tsx'
import { Textarea } from '@/components/ui/textarea.tsx'

interface ProtocolFormState {
  port: string
  server_address: string
  certificate_name: string
  enabled: boolean
  notes: string
}

const emptyState: ProtocolFormState = {
  port: '',
  server_address: '',
  certificate_name: '',
  enabled: true,
  notes: '',
}

function ProtocolConfigCard({ protocol }: { protocol: ProtocolConfigProtocol }) {
  const { data: configs = [], isLoading } = useUserManagerProtocolConfigsQuery()
  const { mutateAsync: upsert, isPending } =
    useUpsertUserManagerProtocolConfigMutation()
  const {
    mutateAsync: uploadCertificate,
    isPending: isCertificateUploading,
  } = useUploadUserManagerProtocolCertificateFileMutation()
  const { mutateAsync: uploadClientApp, isPending: isClientAppUploading } =
    useUploadUserManagerProtocolClientAppFileMutation()
  const {
    mutateAsync: deleteCertificate,
    isPending: isCertificateDeleting,
  } = useDeleteUserManagerProtocolCertificateFileMutation()
  const { mutateAsync: deleteClientApp, isPending: isClientAppDeleting } =
    useDeleteUserManagerProtocolClientAppFileMutation()

  const certificateInputRef = useRef<HTMLInputElement>(null)
  const clientAppInputRef = useRef<HTMLInputElement>(null)
  const [isCertificateDownloading, setIsCertificateDownloading] =
    useState(false)
  const [isClientAppDownloading, setIsClientAppDownloading] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<
    'certificate' | 'client-app' | null
  >(null)

  const existing = configs.find((c) => c.protocol === protocol)

  const [state, setState] = useState<ProtocolFormState>(emptyState)

  useEffect(() => {
    if (existing) {
      setState({
        port: String(existing.port),
        server_address: existing.server_address ?? '',
        certificate_name: existing.certificate_name ?? '',
        enabled: existing.enabled,
        notes: existing.notes ?? '',
      })
    }
  }, [existing])

  const handleSave = async () => {
    const port = Number(state.port)
    if (!port || port < 1 || port > 65535) {
      toast.error('لطفاً یک پورت معتبر وارد کنید (۱ تا ۶۵۵۳۵).')
      return
    }

    try {
      await upsert({
        protocol,
        port,
        server_address: state.server_address || undefined,
        certificate_name: state.certificate_name || undefined,
        enabled: state.enabled,
        notes: state.notes || undefined,
      })
      toast.success(`تنظیمات ${protocol.toUpperCase()} با موفقیت ذخیره شد.`, {
        duration: 5000,
      })
    } catch {
      toast.error('ذخیره تنظیمات پروتکل ناموفق بود. دوباره تلاش کنید.')
    }
  }

  const handleCertificateFileSelected = async (
    e: React.ChangeEvent<HTMLInputElement>
  ) => {
    const file = e.target.files?.[0]
    if (!file) return
    try {
      await uploadCertificate({ protocol, file })
      toast.success('فایل گواهی/کیفیت با موفقیت آپلود شد.', {
        duration: 5000,
      })
    } catch {
      toast.error('آپلود فایل ناموفق بود. دوباره تلاش کنید.')
    } finally {
      e.target.value = ''
    }
  }

  const handleClientAppFileSelected = async (
    e: React.ChangeEvent<HTMLInputElement>
  ) => {
    const file = e.target.files?.[0]
    if (!file) return
    try {
      await uploadClientApp({ protocol, file })
      toast.success('اپلیکیشن کلاینت با موفقیت آپلود شد.', { duration: 5000 })
    } catch {
      toast.error('آپلود فایل ناموفق بود. دوباره تلاش کنید.')
    } finally {
      e.target.value = ''
    }
  }

  const handleDownloadCertificate = async () => {
    setIsCertificateDownloading(true)
    try {
      const { blob, filename } =
        await fetchUserManagerProtocolCertificateFile(protocol)
      triggerBlobDownload(
        blob,
        filename ?? filenameWithFallbackExtension(`${protocol}-certificate`, blob)
      )
    } catch {
      toast.error('دانلود فایل ناموفق بود.')
    } finally {
      setIsCertificateDownloading(false)
    }
  }

  const handleDownloadClientApp = async () => {
    setIsClientAppDownloading(true)
    try {
      const { blob, filename } =
        await fetchUserManagerProtocolClientAppFile(protocol)
      triggerBlobDownload(
        blob,
        filename ?? filenameWithFallbackExtension(`${protocol}-client-app`, blob)
      )
    } catch {
      toast.error('دانلود فایل ناموفق بود.')
    } finally {
      setIsClientAppDownloading(false)
    }
  }

  const handleConfirmDelete = async () => {
    if (deleteTarget === 'certificate') {
      try {
        await deleteCertificate(protocol)
        toast.success('فایل گواهی/کیفیت حذف شد.', { duration: 5000 })
      } catch {
        toast.error('حذف فایل ناموفق بود. دوباره تلاش کنید.')
      }
    } else if (deleteTarget === 'client-app') {
      try {
        await deleteClientApp(protocol)
        toast.success('اپلیکیشن کلاینت حذف شد.', { duration: 5000 })
      } catch {
        toast.error('حذف فایل ناموفق بود. دوباره تلاش کنید.')
      }
    }
    setDeleteTarget(null)
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className='flex items-center justify-between'>
          <span className='uppercase'>{protocol}</span>
        </CardTitle>
      </CardHeader>
      <CardContent className='space-y-4'>
        {isLoading ? (
          <div className='space-y-3'>
            <Skeleton className='h-9 w-full' />
            <Skeleton className='h-9 w-full' />
            <Skeleton className='h-9 w-full' />
          </div>
        ) : (
          <>
            <div className='grid gap-4 md:grid-cols-2'>
              <div className='space-y-2'>
                <Label>پورت</Label>
                <Input
                  type='number'
                  min='1'
                  max='65535'
                  value={state.port}
                  onChange={(e) =>
                    setState((prev) => ({ ...prev, port: e.target.value }))
                  }
                  placeholder='مثلاً 1701'
                />
              </div>
              <div className='space-y-2'>
                <Label>آدرس سرور (اختیاری)</Label>
                <Input
                  value={state.server_address}
                  onChange={(e) =>
                    setState((prev) => ({
                      ...prev,
                      server_address: e.target.value,
                    }))
                  }
                  placeholder='در صورت خالی بودن، آدرس پیش‌فرض سرور استفاده می‌شود'
                />
              </div>
              <div className='space-y-2'>
                <Label>نام گواهی (اختیاری)</Label>
                <Input
                  value={state.certificate_name}
                  onChange={(e) =>
                    setState((prev) => ({
                      ...prev,
                      certificate_name: e.target.value,
                    }))
                  }
                  placeholder='مثلاً server-cert (SSTP/OpenVPN/IKEv2)'
                />
              </div>
              <div className='flex items-center gap-2 pt-6'>
                <Checkbox
                  checked={state.enabled}
                  onCheckedChange={(checked) =>
                    setState((prev) => ({ ...prev, enabled: Boolean(checked) }))
                  }
                />
                <Label>فعال (فقط اطلاع‌رسانی)</Label>
              </div>
            </div>
            <div className='space-y-2'>
              <Label>یادداشت (اختیاری، نمایش به مشتریان)</Label>
              <Textarea
                value={state.notes}
                onChange={(e) =>
                  setState((prev) => ({ ...prev, notes: e.target.value }))
                }
                placeholder='در صفحه‌ی اشتراک هر مشتری برای این پروتکل نمایش داده می‌شود، مثلاً «فقط UDP باز است»'
              />
            </div>

            <div className='grid gap-4 md:grid-cols-2'>
              <div className='space-y-2 rounded-lg border p-3'>
                <Label>فایل گواهی / کیفیت</Label>
                <p className='text-muted-foreground text-xs'>
                  فایل مرجع برای این پروتکل (مثلاً یک خروجی گواهی). به‌صورت
                  خودکار به مشتریان نمایش داده نمی‌شود -- فقط از همین‌جا
                  قابل دانلود است.
                </p>
                <input
                  ref={certificateInputRef}
                  type='file'
                  className='hidden'
                  onChange={handleCertificateFileSelected}
                />
                <div className='flex gap-2'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    disabled={isCertificateUploading}
                    onClick={() => certificateInputRef.current?.click()}
                  >
                    <UploadIcon className='mr-1 h-3.5 w-3.5' />
                    {isCertificateUploading ? 'در حال آپلود...' : 'آپلود'}
                  </Button>
                  {existing?.has_certificate_file && (
                    <Button
                      type='button'
                      variant='secondary'
                      size='sm'
                      disabled={isCertificateDownloading}
                      onClick={handleDownloadCertificate}
                    >
                      <DownloadIcon className='mr-1 h-3.5 w-3.5' />
                      {isCertificateDownloading ? 'در حال دانلود...' : 'دانلود'}
                    </Button>
                  )}
                  {existing?.has_certificate_file && (
                    <Button
                      type='button'
                      variant='destructive'
                      size='sm'
                      disabled={isCertificateDeleting}
                      onClick={() => setDeleteTarget('certificate')}
                    >
                      <TrashIcon className='mr-1 h-3.5 w-3.5' />
                      حذف
                    </Button>
                  )}
                </div>
                {!existing?.has_certificate_file && (
                  <p className='text-muted-foreground text-xs'>
                    هنوز فایلی آپلود نشده است.
                  </p>
                )}
              </div>

              <div className='space-y-2 rounded-lg border p-3'>
                <Label>اپلیکیشن کلاینت</Label>
                <p className='text-muted-foreground text-xs'>
                  اپلیکیشن/نصب‌کننده‌ای که مشتریان برای اتصال با این پروتکل
                  نیاز دارند. به‌عنوان یک دانلود در هر صفحه‌ی اشتراک این
                  پروتکل نمایش داده می‌شود.
                </p>
                <input
                  ref={clientAppInputRef}
                  type='file'
                  className='hidden'
                  onChange={handleClientAppFileSelected}
                />
                <div className='flex gap-2'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    disabled={isClientAppUploading}
                    onClick={() => clientAppInputRef.current?.click()}
                  >
                    <UploadIcon className='mr-1 h-3.5 w-3.5' />
                    {isClientAppUploading ? 'در حال آپلود...' : 'آپلود'}
                  </Button>
                  {existing?.has_client_app_file && (
                    <Button
                      type='button'
                      variant='secondary'
                      size='sm'
                      disabled={isClientAppDownloading}
                      onClick={handleDownloadClientApp}
                    >
                      <DownloadIcon className='mr-1 h-3.5 w-3.5' />
                      {isClientAppDownloading ? 'در حال دانلود...' : 'دانلود'}
                    </Button>
                  )}
                  {existing?.has_client_app_file && (
                    <Button
                      type='button'
                      variant='destructive'
                      size='sm'
                      disabled={isClientAppDeleting}
                      onClick={() => setDeleteTarget('client-app')}
                    >
                      <TrashIcon className='mr-1 h-3.5 w-3.5' />
                      حذف
                    </Button>
                  )}
                </div>
                {!existing?.has_client_app_file && (
                  <p className='text-muted-foreground text-xs'>
                    هنوز فایلی آپلود نشده است.
                  </p>
                )}
              </div>
            </div>

            <Button onClick={handleSave} disabled={isPending}>
              ذخیره تنظیمات {protocol.toUpperCase()}
            </Button>
          </>
        )}
      </CardContent>

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        handleConfirm={handleConfirmDelete}
        title='حذف فایل؟'
        desc={
          deleteTarget === 'certificate'
            ? `این کار فایل گواهی/کیفیت ${protocol.toUpperCase()} را برای همیشه حذف می‌کند.`
            : `این کار اپلیکیشن کلاینت ${protocol.toUpperCase()} را برای همیشه حذف می‌کند. مشتریان دیگر نمی‌توانند آن را از صفحه‌ی اشتراک خود دانلود کنند.`
        }
        isLoading={isCertificateDeleting || isClientAppDeleting}
        confirmText='حذف'
        destructive
      />
    </Card>
  )
}

export function ProtocolSettingsTab() {
  return (
    <div className='space-y-4'>
      <p className='text-muted-foreground text-sm'>
        Record the port each protocol is configured on in Winbox. This panel
        never changes RouterOS&apos;s actual server settings -- it only uses
        this to build correct share/connection-info links.
      </p>
      <div className='grid gap-4 md:grid-cols-2'>
        {PROTOCOL_CONFIG_PROTOCOLS.map((protocol) => (
          <ProtocolConfigCard key={protocol} protocol={protocol} />
        ))}
      </div>
    </div>
  )
}
