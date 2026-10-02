import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { fetchTunnelPolicies, updateTunnelPolicy } from '@/api/tunnel-health.ts'
import { TunnelPolicy } from '@/schema/tunnel-health.ts'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { IconChevronDown } from '@tabler/icons-react'

function PolicyForm({ policy }: { policy: TunnelPolicy }) {
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<TunnelPolicy>(policy)

  const mutation = useMutation({
    mutationFn: updateTunnelPolicy,
    onSuccess: () => {
      toast.success(`سیاست ${policy.interface_name} ذخیره شد.`)
      queryClient.invalidateQueries({ queryKey: ['tunnel_ai_policies'] })
    },
    onError: () => toast.error('ذخیره سیاست ناموفق بود.'),
  })

  return (
    <div className='grid grid-cols-1 gap-6 py-2 md:grid-cols-2'>
      <div className='space-y-3'>
        <p className='text-sm font-medium'>تشخیص</p>
        <div className='space-y-1'>
          <Label>آستانه شکست پینگ (تعداد)</Label>
          <Input
            type='number'
            value={draft.detection.ping_fail_threshold}
            onChange={(e) =>
              setDraft({
                ...draft,
                detection: {
                  ...draft.detection,
                  ping_fail_threshold: Number(e.target.value),
                },
              })
            }
          />
        </div>
        <div className='space-y-1'>
          <Label>پنجره نامتقارنی Tx/Rx (دقیقه)</Label>
          <Input
            type='number'
            value={draft.detection.tx_rx_asymmetry_window_minutes}
            onChange={(e) =>
              setDraft({
                ...draft,
                detection: {
                  ...draft.detection,
                  tx_rx_asymmetry_window_minutes: Number(e.target.value),
                },
              })
            }
          />
        </div>
        <div className='space-y-1'>
          <Label>نسبت نامتقارنی Rx/Tx (مثلاً 0.05)</Label>
          <Input
            type='number'
            step='0.01'
            value={draft.detection.tx_rx_asymmetry_ratio}
            onChange={(e) =>
              setDraft({
                ...draft,
                detection: {
                  ...draft.detection,
                  tx_rx_asymmetry_ratio: Number(e.target.value),
                },
              })
            }
          />
        </div>
      </div>

      <div className='space-y-3'>
        <p className='text-sm font-medium'>سطح ۱ (احیای نرم)</p>
        <div className='space-y-1'>
          <Label>زمان انتظار پس از اقدام (ثانیه)</Label>
          <Input
            type='number'
            value={draft.level1.wait_seconds}
            onChange={(e) =>
              setDraft({
                ...draft,
                level1: { ...draft.level1, wait_seconds: Number(e.target.value) },
              })
            }
          />
        </div>

        <p className='pt-2 text-sm font-medium'>سطح ۲ (سوییچ بکاپ)</p>
        <div className='space-y-1'>
          <Label>نام تانل بکاپ (خالی = غیرفعال)</Label>
          <Input
            value={draft.level2.backup_target}
            onChange={(e) =>
              setDraft({
                ...draft,
                level2: { ...draft.level2, backup_target: e.target.value },
              })
            }
          />
        </div>
        <div className='space-y-1'>
          <Label>آی‌پی واسطه‌ی تانل بکاپ (مثال: 100.100.77.5)</Label>
          <Input
            value={draft.level2.backup_gateway_ip}
            onChange={(e) =>
              setDraft({
                ...draft,
                level2: { ...draft.level2, backup_gateway_ip: e.target.value },
              })
            }
          />
          <p className='text-muted-foreground text-xs'>
            آی‌پی واسطه‌ای که مسیرها برای رسیدن به تانل بکاپ در جدول
            روتینگ استفاده می‌کنند -- بدون این مقدار، سطح ۲ فقط شبیه‌سازی
            می‌شود و مسیر واقعی جابجا نمی‌شود.
          </p>
        </div>
      </div>

      <div className='space-y-3'>
        <p className='text-sm font-medium'>بازگشت خودکار (Fallback)</p>
        <div className='space-y-1'>
          <Label>مدت پایداری لازم برای بازگشت (ثانیه)</Label>
          <Input
            type='number'
            value={draft.fallback.stable_duration_seconds}
            onChange={(e) =>
              setDraft({
                ...draft,
                fallback: { stable_duration_seconds: Number(e.target.value) },
              })
            }
          />
        </div>
      </div>

      <div className='space-y-3'>
        <p className='text-sm font-medium'>جلوگیری از نوسان (Anti-flapping)</p>
        <div className='space-y-1'>
          <Label>حداکثر تعداد قطع/وصل مجاز</Label>
          <Input
            type='number'
            value={draft.anti_flapping.max_flaps}
            onChange={(e) =>
              setDraft({
                ...draft,
                anti_flapping: {
                  ...draft.anti_flapping,
                  max_flaps: Number(e.target.value),
                },
              })
            }
          />
        </div>
        <div className='space-y-1'>
          <Label>بازه بررسی (دقیقه)</Label>
          <Input
            type='number'
            value={draft.anti_flapping.window_minutes}
            onChange={(e) =>
              setDraft({
                ...draft,
                anti_flapping: {
                  ...draft.anti_flapping,
                  window_minutes: Number(e.target.value),
                },
              })
            }
          />
        </div>
        <div className='space-y-1'>
          <Label>مدت قفل پس از نوسان (ساعت)</Label>
          <Input
            type='number'
            value={draft.anti_flapping.cooldown_hours}
            onChange={(e) =>
              setDraft({
                ...draft,
                anti_flapping: {
                  ...draft.anti_flapping,
                  cooldown_hours: Number(e.target.value),
                },
              })
            }
          />
        </div>
      </div>

      <div className='space-y-3 md:col-span-2'>
        <div className='flex items-center justify-between'>
          <p className='text-sm font-medium'>سطح ۳ (ساخت تانل و مسیر جدید)</p>
          <Switch
            checked={draft.level3.enabled}
            onCheckedChange={(checked) =>
              setDraft({
                ...draft,
                level3: { ...draft.level3, enabled: checked },
              })
            }
          />
        </div>
        <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
          <div className='space-y-1'>
            <Label>نوع تانل (gre / ipip / eoip)</Label>
            <Input
              value={draft.level3.type}
              onChange={(e) =>
                setDraft({
                  ...draft,
                  level3: { ...draft.level3, type: e.target.value },
                })
              }
            />
          </div>
          <div className='space-y-1'>
            <Label>نام اینترفیس جدید</Label>
            <Input
              value={draft.level3.new_interface_name}
              onChange={(e) =>
                setDraft({
                  ...draft,
                  level3: {
                    ...draft.level3,
                    new_interface_name: e.target.value,
                  },
                })
              }
            />
          </div>
          <div className='space-y-1'>
            <Label>آدرس مقصد (remote-address)</Label>
            <Input
              value={draft.level3.remote_address}
              onChange={(e) =>
                setDraft({
                  ...draft,
                  level3: { ...draft.level3, remote_address: e.target.value },
                })
              }
            />
          </div>
          <div className='space-y-1'>
            <Label>آدرس مبدا (local-address، یا "auto")</Label>
            <Input
              value={draft.level3.local_address}
              onChange={(e) =>
                setDraft({
                  ...draft,
                  level3: { ...draft.level3, local_address: e.target.value },
                })
              }
            />
          </div>
          <div className='space-y-1 md:col-span-2'>
            <Label>آی‌پی واسطه‌ی تانل جدید (gateway_ip)</Label>
            <Input
              value={draft.level3.gateway_ip}
              onChange={(e) =>
                setDraft({
                  ...draft,
                  level3: { ...draft.level3, gateway_ip: e.target.value },
                })
              }
            />
            <p className='text-muted-foreground text-xs'>
              آی‌پی واسطه‌ای که مسیرهای جدید بعد از ساخت تانل به سمت آن
              اشاره می‌کنند -- بدون این مقدار، سطح ۳ فقط شبیه‌سازی می‌شود
              (حتی اگر فعال باشد).
            </p>
          </div>
        </div>
        <div className='flex items-center justify-between'>
          <Label>پنهان‌سازی آی‌پی (Masquerade)</Label>
          <Switch
            checked={draft.level3.masquerade}
            onCheckedChange={(checked) =>
              setDraft({
                ...draft,
                level3: { ...draft.level3, masquerade: checked },
              })
            }
          />
        </div>
      </div>

      <div className='md:col-span-2'>
        <Button
          onClick={() => mutation.mutate(draft)}
          disabled={mutation.isPending}
        >
          ذخیره سیاست {policy.interface_name}
        </Button>
      </div>
    </div>
  )
}

// PolicyTab is the "سیاست هر تانل" subpage -- one form per discovered
// WireGuard interface, seeded with safe defaults on first discovery (see
// TunnelPolicyService.GetOrCreateDefault on the backend). This is how an
// admin corrects a wrong detection threshold per the spec's own section
// ب-11 philosophy: the engine never learns/adjusts itself, only an
// explicit admin edit here changes behavior.
export function PolicyTab() {
  const { data: policies = [], isLoading } = useQuery({
    queryKey: ['tunnel_ai_policies'],
    queryFn: () => fetchTunnelPolicies(),
  })

  if (isLoading) {
    return <Skeleton className='h-40 w-full rounded-lg' />
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>سیاست هر تانل</CardTitle>
      </CardHeader>
      <CardContent>
        {policies.length === 0 ? (
          <p className='text-muted-foreground py-8 text-center text-sm'>
            هنوز هیچ تانلی کشف نشده است.
          </p>
        ) : (
          <div className='space-y-2'>
            {policies.map((policy) => (
              <Collapsible key={policy.id} className='rounded-lg border px-4'>
                <CollapsibleTrigger className='flex w-full items-center justify-between py-3 text-sm font-medium'>
                  {policy.interface_name}
                  <IconChevronDown className='h-4 w-4' />
                </CollapsibleTrigger>
                <CollapsibleContent className='pb-4'>
                  <PolicyForm policy={policy} />
                </CollapsibleContent>
              </Collapsible>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
