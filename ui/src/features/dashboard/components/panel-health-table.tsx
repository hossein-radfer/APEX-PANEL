import { formatNumberFa } from '@/features/reports/lib/format.ts'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

// Shared table shape for the admin dashboard's "panel rollup" cards
// (V2Ray panels, DNS panels) -- these were two copy-pasted raw <table>
// blocks with identical columns (panel / online / count / health), so
// collapsed into one component instead of drifting further apart on
// future edits. Uses the shared Table primitives (row hover, consistent
// cell padding) instead of a bare <table>, matching every other
// feature's list styling.
export interface PanelHealthRow {
  panelId: number
  panelName: string
  onlineCount: number
  itemCount: number
  hasRecentError: boolean
}

interface Props {
  rows: PanelHealthRow[]
  itemCountLabel: string
}

export function PanelHealthTable({ rows, itemCountLabel }: Props) {
  return (
    <div className='overflow-hidden rounded-lg border'>
      <Table>
        <TableHeader>
          <TableRow className='bg-muted/40'>
            <TableHead className='text-start'>پنل</TableHead>
            <TableHead className='text-start'>آنلاین</TableHead>
            <TableHead className='text-start'>{itemCountLabel}</TableHead>
            <TableHead className='text-start'>سلامت</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={row.panelId}>
              <TableCell className='font-medium'>{row.panelName}</TableCell>
              <TableCell>{formatNumberFa(row.onlineCount)}</TableCell>
              <TableCell>{formatNumberFa(row.itemCount)}</TableCell>
              <TableCell>
                {row.hasRecentError ? (
                  <Badge variant='destructive'>خطای همگام‌سازی</Badge>
                ) : (
                  <Badge className='bg-green-600 text-white dark:bg-green-500'>
                    سالم
                  </Badge>
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
