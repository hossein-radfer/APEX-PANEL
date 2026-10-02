import { Button } from '@/components/ui/button'

export default function MaintenanceError() {
  return (
    <div className='h-svh'>
      <div className='m-auto flex h-full w-full flex-col items-center justify-center gap-2'>
        <h1 className='text-[7rem] leading-tight font-bold'>۵۰۳</h1>
        <span className='font-medium'>سایت در حال به‌روزرسانی است!</span>
        <p className='text-muted-foreground text-center'>
          سرویس در حال حاضر در دسترس نیست. <br />
          به‌زودی بازمی‌گردیم.
        </p>
        <div className='mt-6 flex gap-4'>
          <Button variant='outline'>اطلاعات بیشتر</Button>
        </div>
      </div>
    </div>
  )
}
