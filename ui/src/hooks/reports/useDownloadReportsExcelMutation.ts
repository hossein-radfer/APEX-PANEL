import { useMutation } from '@tanstack/react-query'
import { downloadReportsExcelExport } from '@/api/reports.ts'
import { ReportsRange } from '@/schema/reports.ts'

export const useDownloadReportsExcelMutation = () =>
  useMutation({
    mutationFn: (range: ReportsRange) => downloadReportsExcelExport(range),
  })
