import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { IconCopy, IconKey, IconTrash } from '@tabler/icons-react'
import { createApiKey, fetchApiKeys, revokeApiKey } from '@/api/api-key.ts'
import { Badge } from '@/components/ui/badge'
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
import { ConfirmDialog } from '@/components/confirm-dialog'
import { copyToClipboard } from '@/lib/clipboard'

function backendErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof AxiosError) {
    const message = err.response?.data?.message
    if (typeof message === 'string' && message.length > 0) return message
  }
  return fallback
}

// ApiKeyCard is phase 4-3's admin-facing key management surface: unlike
// SupportTokenCard's single, expiring, support-only credential, these keys
// are standing, admin-issued credentials for external panels/automation
// scripts to call the /api/external/* surface -- see the backend's
// model.ApiKey doc comment for the full design (multiple concurrent keys,
// individually labeled and revocable, never expiring on their own).
export function ApiKeyCard() {
  const queryClient = useQueryClient()
  const [label, setLabel] = useState('')
  const [justCreatedKey, setJustCreatedKey] = useState<string | null>(null)
  const [revokeTargetId, setRevokeTargetId] = useState<number | null>(null)

  const { data: keys, isLoading } = useQuery({
    queryKey: ['api_keys'],
    queryFn: fetchApiKeys,
  })

  const createMutation = useMutation({
    mutationFn: () => createApiKey(label.trim()),
    onSuccess: (result) => {
      setJustCreatedKey(result.key)
      setLabel('')
      queryClient.invalidateQueries({ queryKey: ['api_keys'] })
      toast.success('کلید API ایجاد شد.')
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'ایجاد کلید API ناموفق بود.')),
  })

  const revokeMutation = useMutation({
    mutationFn: (id: number) => revokeApiKey(id),
    onSuccess: () => {
      setRevokeTargetId(null)
      queryClient.invalidateQueries({ queryKey: ['api_keys'] })
      toast.success('کلید API لغو شد.')
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'لغو کلید API ناموفق بود.')),
  })

  const handleCopy = async () => {
    if (!justCreatedKey) return
    const succeeded = await copyToClipboard(justCreatedKey)
    toast[succeeded ? 'success' : 'error'](
      succeeded ? 'کلید در کلیپ‌بورد کپی شد.' : 'کپی کلید ناموفق بود.'
    )
  }

  return (
    <Card>
      <CardHeader className='flex flex-row items-center gap-3 pb-2'>
        <IconKey className='h-5 w-5' />
        <div>
          <CardTitle className='text-lg'>کلیدهای API خارجی</CardTitle>
          <CardDescription>
            برای اتصال پنل‌ها یا اسکریپت‌های خارجی به بخش /api/external، مستقل
            از ورود ادمین. هر کلید را جداگانه می‌سازید، نام‌گذاری می‌کنید و در
            صورت نیاز لغو می‌کنید.
          </CardDescription>
        </div>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='flex max-w-md items-end gap-2'>
          <div className='flex-1 space-y-1'>
            <Label htmlFor='api-key-label'>برچسب کلید</Label>
            <Input
              id='api-key-label'
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder='مثلاً: همگام‌سازی پنل ایران'
              maxLength={100}
            />
          </div>
          <Button
            onClick={() => createMutation.mutate()}
            disabled={createMutation.isPending || label.trim().length === 0}
          >
            {createMutation.isPending ? 'در حال ایجاد...' : 'ایجاد کلید'}
          </Button>
        </div>

        {justCreatedKey && (
          <div className='space-y-2 rounded-lg border p-3'>
            <p className='text-sm font-medium'>
              این کلید را همین حالا کپی کنید -- دیگر نمایش داده نخواهد شد:
            </p>
            <div className='flex max-w-md items-center gap-2'>
              <Input readOnly value={justCreatedKey} className='font-mono text-xs' />
              <Button variant='outline' size='icon' onClick={handleCopy}>
                <IconCopy className='h-4 w-4' />
              </Button>
            </div>
          </div>
        )}

        {isLoading ? (
          <p className='text-muted-foreground text-sm'>در حال بارگذاری...</p>
        ) : !keys || keys.length === 0 ? (
          <p className='text-muted-foreground text-sm'>هنوز هیچ کلید API ایجاد نشده است.</p>
        ) : (
          <div className='space-y-2'>
            {keys.map((k) => (
              <div
                key={k.id}
                className='flex items-center justify-between rounded-lg border p-3'
              >
                <div className='space-y-1'>
                  <div className='flex items-center gap-2'>
                    <span className='font-medium'>{k.label}</span>
                    <Badge variant={k.revoked ? 'outline' : 'default'}>
                      {k.revoked ? 'لغوشده' : 'فعال'}
                    </Badge>
                  </div>
                  <p className='text-muted-foreground font-mono text-xs'>
                    {k.key_prefix}••••••••
                  </p>
                  <p className='text-muted-foreground text-xs'>
                    {k.last_used_at
                      ? `آخرین استفاده: ${new Date(k.last_used_at * 1000).toLocaleString()}`
                      : 'هنوز استفاده نشده'}
                  </p>
                </div>
                {!k.revoked && (
                  <Button
                    variant='outline'
                    size='icon'
                    onClick={() => setRevokeTargetId(k.id)}
                    disabled={revokeMutation.isPending}
                  >
                    <IconTrash className='h-4 w-4' />
                  </Button>
                )}
              </div>
            ))}
          </div>
        )}
      </CardContent>

      <ConfirmDialog
        open={revokeTargetId !== null}
        onOpenChange={(open) => !open && setRevokeTargetId(null)}
        title='کلید API لغو شود؟'
        desc='این کار بلافاصله دسترسی این کلید را می‌بندد. هر کلاینتی که از این کلید استفاده می‌کند دیگر نمی‌تواند به بخش خارجی API متصل شود.'
        destructive
        confirmText='لغو'
        isLoading={revokeMutation.isPending}
        handleConfirm={() => revokeTargetId !== null && revokeMutation.mutate(revokeTargetId)}
      />
    </Card>
  )
}
