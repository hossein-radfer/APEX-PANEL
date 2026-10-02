import { useNavigate, useRouter } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'

export default function ForbiddenError() {
  const navigate = useNavigate()
  const { history } = useRouter()
  return (
    <div className='h-svh'>
      <div className='m-auto flex h-full w-full flex-col items-center justify-center gap-2'>
        <h1 className='text-[7rem] leading-tight font-bold'>۴۰۳</h1>
        <span className='font-medium'>دسترسی غیرمجاز</span>
        <p className='text-muted-foreground text-center'>
          شما مجوز لازم برای مشاهده‌ی <br />
          این بخش را ندارید.
        </p>
        <div className='mt-6 flex gap-4'>
          <Button variant='outline' onClick={() => history.go(-1)}>
            بازگشت
          </Button>
          <Button onClick={() => navigate({ to: '/' })}>صفحه اصلی</Button>
        </div>
      </div>
    </div>
  )
}
