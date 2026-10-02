import { useQuery } from '@tanstack/react-query'
import { fetchApplicationsList } from '@/api/application.ts'

export const useApplicationsListQuery = () =>
  useQuery({
    queryKey: ['applications_list'],
    queryFn: fetchApplicationsList,
  })
