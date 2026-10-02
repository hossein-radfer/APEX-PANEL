import { useQuery } from '@tanstack/react-query'
import { fetchApplicationOpenVpnTemplateStatus } from '@/api/application.ts'

export const useApplicationOpenVpnTemplateStatusQuery = () =>
  useQuery({
    queryKey: ['application_openvpn_template_status'],
    queryFn: fetchApplicationOpenVpnTemplateStatus,
  })
