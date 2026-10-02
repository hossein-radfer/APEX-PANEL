import { useQuery } from '@tanstack/react-query'
import { fetchApplicationResourceLocations } from '@/api/application.ts'

export const useApplicationResourceLocationsQuery = () =>
  useQuery({
    queryKey: ['application_resource_locations'],
    queryFn: fetchApplicationResourceLocations,
  })
