import { IconRotateClockwise2 } from '@tabler/icons-react'
import { useRenewalRateReportQuery } from '@/hooks/reports/useRenewalRateReportQuery.ts'
import { ReportsRange } from '@/schema/reports.ts'
import { ReportStatCard } from '@/features/reports/components/report-stat-card.tsx'

type RenewalRateCardProps = {
  range: ReportsRange
}

// Report 11 -- percentage of expired packages/accounts/peers renewed
// shortly after expiry.
export function RenewalRateCard({ range }: RenewalRateCardProps) {
  const { data, isLoading } = useRenewalRateReportQuery(range)

  return (
    <ReportStatCard
      title='نرخ تمدید'
      icon={<IconRotateClockwise2 />}
      value={data?.renewal_rate}
      isLoading={isLoading}
      suffix='٪'
      decimals={1}
      subtitle={
        data
          ? `${data.renewed.toLocaleString('fa-IR')} تمدید از ${data.total_expired.toLocaleString('fa-IR')} انقضا`
          : undefined
      }
    />
  )
}
