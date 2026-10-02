import { useQuery } from '@tanstack/react-query'
import { fetchV2RaySaleTitles } from '@/api/v2ray.ts'

export const useV2RaySaleTitlesQuery = () =>
  useQuery({
    queryKey: ['v2ray_sale_titles'],
    queryFn: fetchV2RaySaleTitles,
  })
