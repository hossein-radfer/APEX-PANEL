import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  createTicket,
  fetchTicketMessages,
  fetchTickets,
  fetchTypingStatus,
  pingTyping,
  postTicketMessage,
} from '@/api/help-center.ts'
import { MessageKind } from '@/schema/help-center.ts'

// Polling, not a websocket -- matches every other "live-ish" surface in
// this panel (useUpdateCheckQuery, license-panel's own admin ticket UI).
export const useTicketsQuery = (enabled: boolean = true) =>
  useQuery({
    queryKey: ['help_center_tickets'],
    queryFn: fetchTickets,
    enabled,
    refetchInterval: 5000,
  })

export const useCreateTicketMutation = () => {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ subject, message }: { subject: string; message: string }) =>
      createTicket(subject, message),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['help_center_tickets'] }),
  })
}

export const useTicketMessagesQuery = (ticketId: number | null) =>
  useQuery({
    queryKey: ['help_center_ticket', ticketId],
    queryFn: () => fetchTicketMessages(ticketId as number),
    enabled: ticketId !== null,
    refetchInterval: 2500,
  })

export const useTypingStatusQuery = (ticketId: number | null) =>
  useQuery({
    queryKey: ['help_center_typing', ticketId],
    queryFn: () => fetchTypingStatus(ticketId as number),
    enabled: ticketId !== null,
    refetchInterval: 2500,
  })

export const usePostTicketMessageMutation = (ticketId: number) => {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ body, file, kind }: { body: string; file?: File; kind?: MessageKind }) =>
      postTicketMessage(ticketId, body, file, kind),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['help_center_ticket', ticketId] })
      qc.invalidateQueries({ queryKey: ['help_center_tickets'] })
    },
  })
}

export const usePingTypingMutation = (ticketId: number) =>
  useMutation({ mutationFn: () => pingTyping(ticketId) })
