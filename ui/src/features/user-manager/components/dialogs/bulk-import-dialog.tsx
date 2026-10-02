import { useRef, useState } from 'react'
import { BulkImportAccountsResult } from '@/schema/user-manager.ts'
import { UploadIcon } from 'lucide-react'
import { toast } from 'sonner'
import { useBulkImportUserManagerAccountsMutation } from '@/hooks/user-manager/useBulkImportUserManagerAccountsMutation.ts'
import { useBulkImportUserManagerAccountsForResellerMutation } from '@/hooks/user-manager/useBulkImportUserManagerAccountsForResellerMutation.ts'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'

type Props = {
  open: boolean
  onOpenChange: (state: boolean) => void
  targetResellerId?: number
}

// Syncs accounts already created on RouterOS by an external script into
// the panel's own database, matched purely by username -- NOT account
// creation. See UserManagerService.BulkImportAccounts's doc comment
// (backend) for the full sync algorithm this dialog triggers.
export function BulkImportDialog({ open, onOpenChange, targetResellerId }: Props) {
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [result, setResult] = useState<BulkImportAccountsResult | null>(null)

  const bulkImport = useBulkImportUserManagerAccountsMutation()
  const bulkImportForReseller = useBulkImportUserManagerAccountsForResellerMutation()
  const isImporting = bulkImport.isPending || bulkImportForReseller.isPending

  const handleOpenChange = (state: boolean) => {
    if (!state) {
      setSelectedFile(null)
      setResult(null)
      if (fileInputRef.current) fileInputRef.current.value = ''
    }
    onOpenChange(state)
  }

  const handleFileSelected = (e: React.ChangeEvent<HTMLInputElement>) => {
    setResult(null)
    setSelectedFile(e.target.files?.[0] ?? null)
  }

  const handleImport = async () => {
    if (!selectedFile) return
    try {
      const importResult =
        targetResellerId !== undefined
          ? await bulkImportForReseller.mutateAsync({
              resellerId: targetResellerId,
              file: selectedFile,
            })
          : await bulkImport.mutateAsync(selectedFile)
      setResult(importResult)
      toast.success(
        `وارد کردن تکمیل شد: ${importResult.imported} وارد شد، ${importResult.already_imported} به‌روزرسانی شد.`,
        { duration: 5000 }
      )
    } catch {
      toast.error('وارد کردن گروهی ناموفق بود. فایل را بررسی کرده و دوباره تلاش کنید.', {
        duration: 5000,
      })
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <UploadIcon className='h-4 w-4' />
            وارد کردن گروهی حساب‌ها
          </DialogTitle>
          <DialogDescription>
            حساب‌هایی که از قبل روی User Manager روتر ساخته شده‌اند را با
            پنل همگام‌سازی کنید. هر خط یک نام کاربری، به‌صورت اختیاری همراه
            با سقف ترافیک و انقضا، مثلاً{' '}
            <code className='bg-muted rounded px-1 py-0.5 text-xs'>
              bb2 12G-30d
            </code>
            . خطی که فقط نام کاربری دارد (مثلاً{' '}
            <code className='bg-muted rounded px-1 py-0.5 text-xs'>bb2</code>
            ) به‌صورت نامحدود وارد می‌شود. گروه و پروفایل مستقیماً از روتر
            خوانده می‌شوند -- حساب‌هایی که آنجا پیدا نشوند، رد می‌شوند.
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4'>
          <div className='space-y-2'>
            <Label htmlFor='bulk-import-file'>فایل TXT</Label>
            <input
              id='bulk-import-file'
              ref={fileInputRef}
              type='file'
              accept='.txt,.csv'
              onChange={handleFileSelected}
              disabled={isImporting}
              className='text-sm'
            />
          </div>

          {result && (
            <Alert>
              <AlertTitle>خلاصه‌ی وارد کردن</AlertTitle>
              <AlertDescription>
                <div className='mt-1 space-y-1'>
                  <p>{result.imported} حساب به‌تازگی وارد شد.</p>
                  <p>{result.already_imported} حساب قبلاً وارد شده بود، محدودیت‌ها به‌روزرسانی شد.</p>
                  <p>{result.skipped} نام کاربری روی روتر یافت نشد و رد شد.</p>
                  {result.malformed.length > 0 && (
                    <p>{result.malformed.length} خط نامعتبر بود و رد شد.</p>
                  )}
                  {result.skipped_usernames.length > 0 && (
                    <p className='text-muted-foreground text-xs'>
                      یافت نشد: {result.skipped_usernames.join(', ')}
                    </p>
                  )}
                </div>
              </AlertDescription>
            </Alert>
          )}
        </div>

        <DialogFooter>
          <Button
            onClick={handleImport}
            disabled={!selectedFile || isImporting}
          >
            {isImporting ? 'در حال وارد کردن...' : 'وارد کردن'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
