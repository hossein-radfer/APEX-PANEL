import { useQuery } from '@tanstack/react-query'
import { fetchPanelHealthReport } from '@/api/reports.ts'

export const usePanelHealthReportQuery = () =>
  useQuery({
    queryKey: ['reports_panel_health'],
    queryFn: fetchPanelHealthReport,
  })
