export function TypingIndicator() {
  return (
    <div className='bg-muted flex w-fit items-center gap-1 rounded-2xl rounded-bl-sm px-3.5 py-2.5'>
      <span
        className='bg-muted-foreground/60 size-1.5 animate-bounce rounded-full'
        style={{ animationDelay: '0ms' }}
      />
      <span
        className='bg-muted-foreground/60 size-1.5 animate-bounce rounded-full'
        style={{ animationDelay: '150ms' }}
      />
      <span
        className='bg-muted-foreground/60 size-1.5 animate-bounce rounded-full'
        style={{ animationDelay: '300ms' }}
      />
    </div>
  )
}
