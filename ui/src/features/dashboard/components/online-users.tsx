import { IconUserOff } from '@tabler/icons-react'
import { DeviceData } from '@/schema/dashboard.ts'
import { getAvatarInitials } from '@/utils/helper.ts'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

type Props = {
  peers: NonNullable<DeviceData['PeerInfo']>['recent_online_peers']
}

export default function RecentlyOnlineUsers({ peers }: Props) {
  const hasPeers = peers && peers.length > 0

  return (
    <Card className='col-span-1 flex h-full flex-col lg:col-span-2'>
      <CardHeader>
        <CardTitle className='mb-4 text-lg font-semibold'>
          کاربران اخیراً آنلاین
        </CardTitle>
      </CardHeader>
      <CardContent className='flex flex-grow flex-col p-0'>
        {!hasPeers ? (
          <div className='text-muted-foreground flex flex-grow flex-col items-center justify-center gap-4'>
            <IconUserOff size={64} stroke={1.5} />
            <p className='text-center text-base'>
              اخیراً هیچ کاربری آنلاین نبوده است.
            </p>
          </div>
        ) : (
          <div className='-mt-4'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='pr-6'>کاربر</TableHead>
                  <TableHead className='pl-6 text-left'>آخرین بازدید</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {peers.map((peer, idx) => (
                  <TableRow key={idx}>
                    <TableCell className='py-4 pr-6 font-medium'>
                      <div className='flex items-center gap-3'>
                        <div className='flex h-9 w-9 items-center justify-center rounded-full bg-sky-800 text-sm font-bold text-white'>
                          {getAvatarInitials(peer.name)}
                        </div>
                        <span>{peer.name}</span>
                      </div>
                    </TableCell>
                    <TableCell className='text-muted-foreground pl-6 text-left'>
                      {peer.last_seen} پیش
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
