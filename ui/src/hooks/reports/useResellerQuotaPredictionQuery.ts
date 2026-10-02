import { useQuery } from '@tanstack/react-query'
import { fetchResellerQuotaPrediction } from '@/api/reports.ts'

export const useResellerQuotaPredictionQuery = (enabled: boolean = true) =>
  useQuery({
    queryKey: ['reseller_quota_prediction'],
    queryFn: fetchResellerQuotaPrediction,
    enabled,
    refetchInterval: 60_000,
  })
