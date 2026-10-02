import { IconRoute } from '@tabler/icons-react'

interface Props {
  children: React.ReactNode
}

export default function AuthLayout({ children }: Props) {
  return (
    <div className='bg-primary-foreground relative grid h-svh max-w-none items-center justify-center overflow-hidden'>
      <div
        aria-hidden
        className='bg-primary/15 pointer-events-none absolute -top-32 -end-32 size-96 rounded-full blur-3xl'
      />
      <div
        aria-hidden
        className='bg-primary/10 pointer-events-none absolute -bottom-32 -start-32 size-96 rounded-full blur-3xl'
      />
      <div className='bg-card relative mx-auto flex w-full flex-col justify-center space-y-2 rounded-2xl border py-8 shadow-lg sm:w-[440px] sm:p-8'>
        <div className='mb-4 flex items-center justify-center gap-3'>
          <IconRoute className='text-primary size-6' />
          <h1 className='text-xl font-semibold tracking-tight'>ApexPanel</h1>
        </div>
        {children}
      </div>
    </div>
  )
}
