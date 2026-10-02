import { useState } from 'react'
import {
  IconApps,
  IconCheck,
  IconNetwork,
  IconRouter,
  IconSparkles,
  IconWorld,
} from '@tabler/icons-react'
import { useCompleteResellerOnboardingMutation } from '@/hooks/resellers/useCompleteResellerOnboardingMutation.ts'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog.tsx'
import { Button } from '@/components/ui/button.tsx'

interface Props {
  resellerId: number
  open: boolean
  onOpenChange: (open: boolean) => void
}

interface Step {
  icon: React.ComponentType<{ className?: string }>
  title: string
  description: string
}

const steps: Step[] = [
  {
    icon: IconSparkles,
    title: 'به پنل خوش آمدید',
    description:
      'این یک راهنمای کوتاه ۵ مرحله‌ای است تا با بخش‌های اصلی پنل نمایندگی آشنا شوید. هر زمان می‌توانید آن را ببندید؛ دوباره نمایش داده نمی‌شود.',
  },
  {
    icon: IconRouter,
    title: 'وایرگارد',
    description:
      'از منوی «وایرگارد» می‌توانید برای کاربران خود پیر (Peer) بسازید. حجم و تعداد مجاز شما توسط مدیر پنل تعیین شده و در بالای صفحه قابل مشاهده است.',
  },
  {
    icon: IconNetwork,
    title: 'یوزرمنجیر (L2TP / PPTP / SSTP / OpenVPN)',
    description:
      'اگر مدیر این قابلیت را برای شما فعال کرده باشد، از منوی «یوزرمنجیر» می‌توانید حساب‌های این پروتکل‌ها را بسازید و مدیریت کنید.',
  },
  {
    icon: IconWorld,
    title: 'V2Ray',
    description:
      'اگر مدیر این قابلیت را برای شما فعال کرده باشد، از منوی «V2Ray» می‌توانید پکیج بسازید و روی سرورهای مجاز خود آن را فعال کنید.',
  },
  {
    icon: IconApps,
    title: 'اپلیکیشن‌ها',
    description:
      'در صورت فعال بودن این قابلیت، از منوی «اپلیکیشن‌ها» می‌توانید یک شناسه‌ی ورود موبایل بسازید که همزمان به چند پروتکل دسترسی داشته باشد. برای هر سوال دیگر، از بخش پشتیبانی داخل پنل استفاده کنید.',
  },
]

export function ResellerOnboardingWizard({
  resellerId,
  open,
  onOpenChange,
}: Props) {
  const [stepIndex, setStepIndex] = useState(0)
  const { mutate: completeOnboarding, isPending } =
    useCompleteResellerOnboardingMutation()

  const isLastStep = stepIndex === steps.length - 1
  const step = steps[stepIndex]
  const Icon = step.icon

  const finish = () => {
    completeOnboarding(resellerId)
    onOpenChange(false)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) finish()
      }}
    >
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <div className='bg-primary/10 text-primary mx-auto flex h-12 w-12 items-center justify-center rounded-full'>
            <Icon className='h-6 w-6' />
          </div>
          <DialogTitle className='text-center'>{step.title}</DialogTitle>
          <DialogDescription className='text-center'>
            {step.description}
          </DialogDescription>
        </DialogHeader>

        <div className='flex items-center justify-center gap-1.5'>
          {steps.map((s, i) => (
            <span
              key={s.title}
              className={`h-1.5 rounded-full transition-all ${
                i === stepIndex
                  ? 'bg-primary w-6'
                  : i < stepIndex
                    ? 'bg-primary/50 w-1.5'
                    : 'bg-muted w-1.5'
              }`}
            />
          ))}
        </div>

        <DialogFooter className='sm:justify-between'>
          <Button
            variant='ghost'
            onClick={finish}
            disabled={isPending}
          >
            رد کردن
          </Button>
          <div className='flex gap-2'>
            {stepIndex > 0 && (
              <Button
                variant='outline'
                onClick={() => setStepIndex((i) => i - 1)}
                disabled={isPending}
              >
                قبلی
              </Button>
            )}
            {isLastStep ? (
              <Button onClick={finish} disabled={isPending} className='gap-2'>
                <IconCheck className='h-4 w-4' />
                شروع کنید
              </Button>
            ) : (
              <Button
                onClick={() => setStepIndex((i) => i + 1)}
                disabled={isPending}
              >
                بعدی
              </Button>
            )}
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
