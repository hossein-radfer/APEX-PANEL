import {
  ReportsAnomaly,
  ReportsAnomalyResponseSchema,
  ReportsDailyUsage,
  ReportsDailyUsageResponseSchema,
  ReportsExpiring,
  ReportsExpiringResponseSchema,
  ReportsFinancial,
  ReportsFinancialBucket,
  ReportsFinancialResponseSchema,
  ReportsOnlineUsers,
  ReportsOnlineUsersResponseSchema,
  ReportsPanelHealth,
  ReportsPanelHealthResponseSchema,
  ReportsPeakUsage,
  ReportsPeakUsageResponseSchema,
  ReportsPopularLocations,
  ReportsPopularLocationsResponseSchema,
  ReportsProtocolShare,
  ReportsProtocolShareResponseSchema,
  ReportsRange,
  ReportsRanking,
  ReportsRankingResponseSchema,
  ReportsRenewalRate,
  ReportsRenewalRateResponseSchema,
  ReportsResource,
  ReportsResourceResponseSchema,
  ResellerQuotaPrediction,
  ResellerQuotaPredictionResponseSchema,
} from '@/schema/reports.ts'
import axiosInstance from '@/api/axios-instance.ts'
import { triggerBlobDownload } from '@/lib/download.ts'

// Report 1
export const fetchDailyUsageReport = async (
  range: ReportsRange,
  protocol?: string
): Promise<ReportsDailyUsage> => {
  const { data } = await axiosInstance.get('/reports/daily-usage', {
    params: { range, protocol: protocol || undefined },
  })
  const parsed = ReportsDailyUsageResponseSchema.parse(data)
  return parsed.data
}

// Report 2
export const fetchPeakUsageReport = async (
  range: ReportsRange,
  entity?: string
): Promise<ReportsPeakUsage> => {
  const { data } = await axiosInstance.get('/reports/peak-usage', {
    params: { range, entity: entity || undefined },
  })
  const parsed = ReportsPeakUsageResponseSchema.parse(data)
  return parsed.data
}

// Report 3
export const fetchResellerRankingReport = async (
  range: ReportsRange
): Promise<ReportsRanking> => {
  const { data } = await axiosInstance.get('/reports/reseller-ranking', {
    params: { range },
  })
  const parsed = ReportsRankingResponseSchema.parse(data)
  return parsed.data
}

// Report 4
export const fetchUserRankingReport = async (
  range: ReportsRange,
  protocol: string
): Promise<ReportsRanking> => {
  const { data } = await axiosInstance.get('/reports/user-ranking', {
    params: { range, protocol },
  })
  const parsed = ReportsRankingResponseSchema.parse(data)
  return parsed.data
}

// Report 5
export const fetchResellerActivityReport = async (
  range: ReportsRange
): Promise<ReportsRanking> => {
  const { data } = await axiosInstance.get('/reports/reseller-activity', {
    params: { range },
  })
  const parsed = ReportsRankingResponseSchema.parse(data)
  return parsed.data
}

// Report 6
export const fetchExpiringSoonReport = async (): Promise<ReportsExpiring> => {
  const { data } = await axiosInstance.get('/reports/expiring-soon')
  const parsed = ReportsExpiringResponseSchema.parse(data)
  return parsed.data
}

// Report 7
export const fetchProtocolShareReport = async (
  range: ReportsRange
): Promise<ReportsProtocolShare> => {
  const { data } = await axiosInstance.get('/reports/protocol-share', {
    params: { range },
  })
  const parsed = ReportsProtocolShareResponseSchema.parse(data)
  return parsed.data
}

// Report 8
export const fetchOnlineUsersReport = async (): Promise<ReportsOnlineUsers> => {
  const { data } = await axiosInstance.get('/reports/online-users')
  const parsed = ReportsOnlineUsersResponseSchema.parse(data)
  return parsed.data
}

// Report 9
export const fetchPanelHealthReport = async (): Promise<ReportsPanelHealth> => {
  const { data } = await axiosInstance.get('/reports/panel-health')
  const parsed = ReportsPanelHealthResponseSchema.parse(data)
  return parsed.data
}

// Report 10
export const fetchFinancialReport = async (
  range: ReportsRange,
  bucket: ReportsFinancialBucket
): Promise<ReportsFinancial> => {
  const { data } = await axiosInstance.get('/reports/financial', {
    params: { range, bucket },
  })
  const parsed = ReportsFinancialResponseSchema.parse(data)
  return parsed.data
}

// Report 11
export const fetchRenewalRateReport = async (
  range: ReportsRange
): Promise<ReportsRenewalRate> => {
  const { data } = await axiosInstance.get('/reports/renewal-rate', {
    params: { range },
  })
  const parsed = ReportsRenewalRateResponseSchema.parse(data)
  return parsed.data
}

// Report 12
export const fetchPopularLocationsReport = async (
  range: ReportsRange
): Promise<ReportsPopularLocations> => {
  const { data } = await axiosInstance.get('/reports/popular-locations', {
    params: { range },
  })
  const parsed = ReportsPopularLocationsResponseSchema.parse(data)
  return parsed.data
}

// Report 13
export const fetchAnomalyAlertsReport = async (): Promise<ReportsAnomaly> => {
  const { data } = await axiosInstance.get('/reports/anomaly-alerts')
  const parsed = ReportsAnomalyResponseSchema.parse(data)
  return parsed.data
}

// Report 15
export const fetchResourceUsageReport = async (
  range: ReportsRange
): Promise<ReportsResource> => {
  const { data } = await axiosInstance.get('/reports/resource-usage', {
    params: { range },
  })
  const parsed = ReportsResourceResponseSchema.parse(data)
  return parsed.data
}

// Report 14 -- streams an actual .xlsx workbook (not JSON), so this
// downloads the blob directly and triggers a synthetic <a download> click
// instead of parsing through a Zod schema, mirroring api/backup.ts's
// downloadBackup pattern (axios blob + Bearer auth header already attached
// by the shared axiosInstance request interceptor).
export const downloadReportsExcelExport = async (
  range: ReportsRange
): Promise<void> => {
  const response = await axiosInstance.get('/reports/export', {
    params: { range },
    responseType: 'blob',
  })

  const blob = new Blob([response.data], {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  })
  const today = new Date().toISOString().split('T')[0]
  triggerBlobDownload(blob, `mwp-reports-${range}-${today}.xlsx`)
}

// Reseller quota prediction (category 2 item 2) -- reseller-scoped, no
// range/protocol params since the endpoint always returns all three
// protocols for the calling reseller.
export const fetchResellerQuotaPrediction =
  async (): Promise<ResellerQuotaPrediction> => {
    const { data } = await axiosInstance.get('/reports/reseller-quota-prediction')
    const parsed = ResellerQuotaPredictionResponseSchema.parse(data)
    return parsed.data
  }
