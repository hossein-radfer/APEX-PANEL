import { IconDownload, IconLoader2 } from '@tabler/icons-react'
import { toast } from 'sonner'
import { useDownloadReportsExcelMutation } from '@/hooks/reports/useDownloadReportsExcelMutation.ts'
import { ReportsRange } from '@/schema/reports.ts'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

type ReportsToolbarProps = {
  range: ReportsRange
  onRangeChange: (range: ReportsRange) => void
}

const RANGE_OPTIONS: { value: ReportsRange; label: string }[] = [
  { value: 'today', label: 'امروز' },
  { value: '7d', label: '۷ روز اخیر' },
  { value: '30d', label: '۳۰ روز اخیر' },
]

// The single, page-level time-range picker plus the Excel export button --
// shared across every range-aware report on the page. Reports that are
// always "right now" (expiring-soon, online-users, panel-health,
// anomaly-alerts) don't read `range` at all and ignore this control.
export function ReportsToolbar({ range, onRangeChange }: ReportsToolbarProps) {
  const { mutate: downloadExcel, isPending: isDownloading } =
    useDownloadReportsExcelMutation()

  const handleExport = () => {
    downloadExcel(range, {
      onError: () => {
        toast.error('خروجی اکسل با خطا مواجه شد. دوباره تلاش کنید.')
      },
    })
  }

  return (
    <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
      <div className='flex items-center gap-2'>
        <span className='text-muted-foreground text-sm font-medium'>
          بازه زمانی:
        </span>
        <Select
          value={range}
          onValueChange={(value) => onRangeChange(value as ReportsRange)}
        >
          <SelectTrigger className='w-[160px]' aria-label='انتخاب بازه زمانی'>
            <SelectValue placeholder='بازه زمانی' />
          </SelectTrigger>
          <SelectContent>
            {RANGE_OPTIONS.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <Button
        onClick={handleExport}
        disabled={isDownloading}
        className='w-full sm:w-auto'
      >
        {isDownloading ? (
          <IconLoader2 className='animate-spin' />
        ) : (
          <IconDownload />
        )}
        خروجی اکسل
      </Button>
    </div>
  )
}
