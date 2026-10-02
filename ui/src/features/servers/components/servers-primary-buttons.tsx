import { IconCloudPlus, IconRefresh } from '@tabler/icons-react'
import { Loader2Icon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ServerConnectionHealthButton } from '@/features/servers/components/server-connection-health-button.tsx'
import { useServers } from '@/features/servers/context/servers-context.tsx'

interface Props {
  serversLength: number
  refetchServersList: () => void
  isServersListRefetching: boolean
}

export function ServersPrimaryButtons({
  serversLength,
  refetchServersList,
  isServersListRefetching,
}: Props) {
  const { setOpen } = useServers()

  return (
    <div className='flex gap-2'>
      <ServerConnectionHealthButton />
      <Button
        variant='outline'
        className='space-x-1'
        disabled={isServersListRefetching}
        onClick={refetchServersList}
      >
        {isServersListRefetching ? (
          <Loader2Icon className='animate-spin' />
        ) : (
          <IconRefresh />
        )}
        <span>بروزرسانی</span>
      </Button>
      <Button
        className='space-x-1'
        disabled={serversLength === 1}
        onClick={() => setOpen('add')}
      >
        <span>افزودن سرور</span> <IconCloudPlus size={18} />
      </Button>
    </div>
  )
}
