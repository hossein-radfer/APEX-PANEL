'use client'

import { useEffect, useMemo, useRef } from 'react'
import { UseFormReturn, useWatch } from 'react-hook-form'
import { CreatePeerRequest } from '@/schema/peers'
import { useAuthStore } from '@/stores/authStore.ts'
import { useAssignedInterfacesQuery } from '@/hooks/resellers/useAssignedInterfacesQuery.ts'
import { useInterfacesListQuery } from '@/hooks/interfaces/useInterfacesListQuery'
import { usePeerAllowedAddressMutation } from '@/hooks/peers/usePeerAllowedAddressMutation.ts'
import { useServersListQuery } from '@/hooks/servers/useServersListQuery'
import { useServerEndpointsQuery } from '@/hooks/servers/useServerEndpointsQuery'

interface Props {
  isEdit: boolean
  form: UseFormReturn<CreatePeerRequest>
  shouldRefetch: boolean
  // When set, an admin is building a peer on behalf of this reseller: the
  // interface choices must be limited to that reseller's assigned
  // interfaces, not the admin's own full interface list.
  targetResellerId?: number
}

export function usePeerDefaults({
  isEdit,
  form,
  shouldRefetch,
  targetResellerId,
}: Props) {
  const role = useAuthStore((state) => state.auth.admin?.role)
  const isReseller = role === 'reseller'
  const isAdminActingForReseller = !isReseller && targetResellerId !== undefined

  const {
    data: allInterfacesList = [],
    isLoading: isAllInterfacesLoading,
    error: allInterfacesError,
    refetch: refetchAllInterfaces,
  } = useInterfacesListQuery(!isAdminActingForReseller)

  const {
    data: assignedInterfaceIds,
    isLoading: isAssignedInterfacesLoading,
    error: assignedInterfacesError,
    refetch: refetchAssignedInterfaces,
  } = useAssignedInterfacesQuery(targetResellerId)

  const interfacesList = useMemo(() => {
    if (!isAdminActingForReseller) return allInterfacesList
    const allowedIds = new Set(assignedInterfaceIds ?? [])
    return allInterfacesList.filter((iface) => allowedIds.has(iface.id))
  }, [isAdminActingForReseller, allInterfacesList, assignedInterfaceIds])

  const isInterfacesLoading = isAdminActingForReseller
    ? isAllInterfacesLoading || isAssignedInterfacesLoading
    : isAllInterfacesLoading
  const interfacesError = isAdminActingForReseller
    ? assignedInterfacesError ?? allInterfacesError
    : allInterfacesError
  const refetchInterfaces = () => {
    refetchAllInterfaces()
    if (isAdminActingForReseller) refetchAssignedInterfaces()
  }

  // Resellers cannot access the admin server list (it also carries
  // management actions they aren't allowed to use), so they fall back to a
  // credential-free endpoints-only query for the IP address a new peer needs.
  // The same applies when an admin is building a peer for a reseller: only
  // the credential-free endpoint list is needed here.
  const {
    data: adminServersList = [],
    isLoading: isAdminServersLoading,
    error: adminServersError,
    refetch: refetchAdminServers,
  } = useServersListQuery(!isReseller && !isAdminActingForReseller)

  const {
    data: resellerServerEndpoints = [],
    isLoading: isResellerServersLoading,
    error: resellerServersError,
    refetch: refetchResellerServers,
  } = useServerEndpointsQuery(isReseller || isAdminActingForReseller)

  const useResellerServers = isReseller || isAdminActingForReseller
  const serversList = useResellerServers ? resellerServerEndpoints : adminServersList
  const isServersLoading = useResellerServers
    ? isResellerServersLoading
    : isAdminServersLoading
  const serversError = useResellerServers ? resellerServersError : adminServersError
  const refetchServers = useResellerServers
    ? refetchResellerServers
    : refetchAdminServers

  const { mutate: GetPeerAllowedAddress } = usePeerAllowedAddressMutation()

  const hasSetDefaults = useRef(false)

  // Watch for interface_id changes
  const interfaceId = useWatch({
    control: form.control,
    name: 'interface_id',
  })

  useEffect(() => {
    if (!isEdit && shouldRefetch) {
      refetchInterfaces()
      refetchServers()
    }
  }, [shouldRefetch, isEdit, refetchInterfaces, refetchServers])

  useEffect(() => {
    if (
      !isEdit &&
      !hasSetDefaults.current &&
      interfacesList.length > 0 &&
      serversList.length > 0 &&
      !isInterfacesLoading &&
      !isServersLoading
    ) {
      const defaultInterface = interfacesList[0]
      const defaultServer = serversList[0]

      form.reset((prev) => ({
        ...prev,
        interface_name: defaultInterface.name,
        interface_id: defaultInterface.id,
        endpoint: defaultServer.ip_address,
        persistent_keepalive: '00:00:25',
      }))

      hasSetDefaults.current = true
    }
  }, [
    isEdit,
    form,
    interfacesList,
    serversList,
    isInterfacesLoading,
    isServersLoading,
  ])

  // Always fetches a fresh allowed_address from the backend on every
  // interface_id change (usePeerAllowedAddressMutation is a mutation, not a
  // cached query, so there's no stale-cache risk from React Query itself).
  // The risk is a network-ordering race: if the user switches interface A ->
  // B quickly and A's response arrives after B's, applying it would stomp
  // the correct value for B with a stale address from A. requestInterfaceId
  // captures which interface each in-flight request was actually for, so a
  // late response is discarded unless it still matches the currently
  // selected interface_id at the time it resolves.
  const latestRequestedInterfaceId = useRef<number | null>(null)

  useEffect(() => {
    if (interfaceId && !isEdit) {
      latestRequestedInterfaceId.current = interfaceId
      GetPeerAllowedAddress(
        { interface_id: interfaceId },
        {
          onSuccess: (data) => {
            if (latestRequestedInterfaceId.current === interfaceId) {
              form.setValue('allowed_address', data.allowed_address)
            }
          },
        }
      )
    }
  }, [interfaceId, GetPeerAllowedAddress, form, isEdit])

  return {
    interfacesList,
    interfacesError,
    isInterfacesLoading,
    serversList,
    serversError,
    isServersLoading,
  }
}
