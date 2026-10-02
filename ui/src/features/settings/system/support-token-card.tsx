import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { IconCopy, IconLifebuoy, IconTrash } from '@tabler/icons-react'
import {
  fetchSupportTokenStatus,
  generateSupportToken,
  revokeSupportToken,
} from '@/api/support-token.ts'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Input } from '@/components/ui/input'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { copyToClipboard } from '@/lib/clipboard'

function backendErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof AxiosError) {
    const message = err.response?.data?.message
    if (typeof message === 'string' && message.length > 0) return message
  }
  return fallback
}

// SupportTokenCard is the customer-facing half of the "privacy-first
// remote debugging" feature: generating a token here is the ONLY way our
// support team can ever see this install's logs remotely -- there is no
// SSH access, no server credential, nothing beyond a time-limited,
// revocable, read-only view of this panel's own log entries. See the
// backend's model.SupportToken doc comment for the full design.
export function SupportTokenCard() {
  const queryClient = useQueryClient()
  const [revokeDialogOpen, setRevokeDialogOpen] = useState(false)
  const [justGeneratedToken, setJustGeneratedToken] = useState<string | null>(null)

  const { data: status, isLoading } = useQuery({
    queryKey: ['support_token_status'],
    queryFn: fetchSupportTokenStatus,
  })

  const generateMutation = useMutation({
    mutationFn: generateSupportToken,
    onSuccess: (result) => {
      setJustGeneratedToken(result.token)
      queryClient.invalidateQueries({ queryKey: ['support_token_status'] })
      toast.success('توکن پشتیبانی ایجاد شد.')
    },
    onError: (err) =>
      toast.error(backendErrorMessage(err, 'ایجاد توکن پشتیبانی ناموفق بود.')),
  })

  const revokeMutation = useMutation({
    mutationFn: revokeSupportToken,
    onSuccess: () => {
      setJustGeneratedToken(null)
      queryClient.invalidateQueries({ queryKey: ['support_token_status'] })
      setRevokeDialogOpen(false)
      toast.success('توکن پشتیبانی لغو شد.')
    },
    onError: (err) =>
      toast.error(backendErrorMessage(err, 'لغو توکن پشتیبانی ناموفق بود.')),
  })

  const handleCopy = async () => {
    if (!justGeneratedToken) return
    const succeeded = await copyToClipboard(justGeneratedToken)
    if (succeeded) {
      toast.success('توکن در کلیپ‌بورد کپی شد.')
    } else {
      toast.error('کپی توکن ناموفق بود.')
    }
  }

  return (
    <Card>
      <CardHeader className='flex flex-row items-center gap-3 pb-2'>
        <IconLifebuoy className='h-5 w-5' />
        <div>
          <CardTitle className='text-lg'>دسترسی پشتیبانی</CardTitle>
          <CardDescription>
            به تیم پشتیبانی ما یک دسترسی موقت و فقط‌خواندنی به لاگ‌های همین
            پنل می‌دهد -- هرگز ورود به سرور یا شل نیست و چیزی فراتر از
            رکوردهای لاگ نخواهد بود. پس از ۲۴ ساعت به‌طور خودکار منقضی می‌شود.
          </CardDescription>
        </div>
      </CardHeader>
      <CardContent className='space-y-4'>
        {isLoading ? (
          <p className='text-muted-foreground text-sm'>در حال بارگذاری...</p>
        ) : (
          <>
            {status?.active ? (
              <Alert>
                <AlertTitle>یک توکن پشتیبانی فعال است</AlertTitle>
                <AlertDescription>
                  انقضا در {new Date(status.expires_at!).toLocaleString()}.
                  {' '}مقدار توکن فقط یک‌بار، در لحظه ایجاد، قابل مشاهده است --
                  در صورت نیاز به اشتراک‌گذاری مجدد، یک توکن جدید بسازید.
                </AlertDescription>
              </Alert>
            ) : (
              <p className='text-muted-foreground text-sm'>
                در حال حاضر هیچ توکن پشتیبانی فعالی وجود ندارد.
              </p>
            )}

            {justGeneratedToken && (
              <div className='space-y-2'>
                <p className='text-sm font-medium'>
                  این توکن را همین حالا کپی کنید -- دیگر نمایش داده نخواهد شد:
                </p>
                <div className='flex max-w-md items-center gap-2'>
                  <Input readOnly value={justGeneratedToken} className='font-mono text-xs' />
                  <Button variant='outline' size='icon' onClick={handleCopy}>
                    <IconCopy className='h-4 w-4' />
                  </Button>
                </div>
              </div>
            )}

            <div className='flex gap-2'>
              <Button
                onClick={() => generateMutation.mutate()}
                disabled={generateMutation.isPending}
              >
                {generateMutation.isPending
                  ? 'در حال ایجاد...'
                  : status?.active
                    ? 'ایجاد توکن جدید'
                    : 'ایجاد توکن پشتیبانی'}
              </Button>
              {status?.active && (
                <Button
                  variant='outline'
                  className='gap-2'
                  onClick={() => setRevokeDialogOpen(true)}
                  disabled={revokeMutation.isPending}
                >
                  <IconTrash className='h-4 w-4' />
                  لغو
                </Button>
              )}
            </div>
          </>
        )}
      </CardContent>

      <ConfirmDialog
        open={revokeDialogOpen}
        onOpenChange={setRevokeDialogOpen}
        title='توکن پشتیبانی لغو شود؟'
        desc='این کار بلافاصله دسترسی لاگ از راه دور را می‌بندد. هر کسی که توکن فعلی را داشته باشد دیگر نمی‌تواند از آن استفاده کند.'
        destructive
        confirmText='لغو'
        isLoading={revokeMutation.isPending}
        handleConfirm={() => revokeMutation.mutate()}
      />
    </Card>
  )
}
