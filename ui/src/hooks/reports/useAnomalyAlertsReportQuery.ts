import { useQuery } from '@tanstack/react-query'
import { fetchAnomalyAlertsReport } from '@/api/reports.ts'

export const useAnomalyAlertsReportQuery = () =>
  useQuery({
    queryKey: ['reports_anomaly_alerts'],
    queryFn: fetchAnomalyAlertsReport,
  })
