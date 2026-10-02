import { useState } from 'react'
import { useUserRankingReportQuery } from '@/hooks/reports/useUserRankingReportQuery.ts'
import { ReportsProtocol, ReportsRange } from '@/schema/reports.ts'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { RankingBarChart } from '@/features/reports/components/ranking-bar-chart.tsx'

const PROTOCOL_OPTIONS: { value: ReportsProtocol; label: string }[] = [
  { value: 'wireguard', label: 'وایرگارد' },
  { value: 'user_manager', label: 'یوزر منیجر' },
  { value: 'v2ray', label: 'V2Ray' },
]

// Report 4 -- users ranked by usage, scoped to ONE protocol at a time
// (the backend requires the protocol param) via its own local selector,
// independent from the page-level range picker.
export function UserRankingReport({ range }: { range: ReportsRange }) {
  const [protocol, setProtocol] = useState<ReportsProtocol>('wireguard')
  const { data, isLoading } = useUserRankingReportQuery(range, protocol)

  return (
    <div className='animate-in fade-in slide-in-from-bottom-2 duration-500'>
      <div className='mb-2 flex items-center justify-end gap-2'>
        <span className='text-muted-foreground text-sm'>پروتکل:</span>
        <Select
          value={protocol}
          onValueChange={(value) => setProtocol(value as ReportsProtocol)}
        >
          <SelectTrigger size='sm' className='w-[150px]'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {PROTOCOL_OPTIONS.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <RankingBarChart
        title='رتبه‌بندی کاربران بر اساس مصرف'
        description='پرمصرف‌ترین کاربران پروتکل انتخاب‌شده'
        rows={data?.rows}
        isLoading={isLoading}
        emptyMessage='هیچ کاربری در این پروتکل و بازه مصرفی نداشته است.'
      />
    </div>
  )
}
