import { z } from 'zod'
import axiosInstance from '@/api/axios-instance.ts'
import {
  MessageKind,
  Ticket,
  TicketMessage,
  TicketMessageSchema,
  TicketMessagesResponseSchema,
  TicketSchema,
} from '@/schema/help-center.ts'

// fetchAttachmentBlob downloads an attachment through axiosInstance (so
// the Authorization header is attached the same way every other
// authenticated request gets it) and returns an object URL for direct use
// in <img>/<audio src> or a download link -- this endpoint is JWT-protected
// (see api/http/api.go's setupHelpCenterRoutes), so a plain <img src="...">
// pointing straight at it would 401 with no token attached.
export const fetchAttachmentBlob = async (ticketId: number, messageId: number): Promise<string> => {
  const { data } = await axiosInstance.get(
    `/help-center/tickets/${ticketId}/messages/${messageId}/attachment`,
    { responseType: 'blob' }
  )
  return URL.createObjectURL(data as Blob)
}

// Unlike most of this API layer, these endpoints return license-panel's
// raw JSON shape passed straight through by MWPanel's own backend (see
// api/http/help_center.go's writeRawJSON) rather than the usual
// {status, message, data} envelope -- so these parse the response body
// directly, no `.data` unwrap.

export const fetchTickets = async (): Promise<Ticket[]> => {
  const { data } = await axiosInstance.get('/help-center/tickets')
  return z.array(TicketSchema).parse(data)
}

export const createTicket = async (subject: string, message: string): Promise<Ticket> => {
  const { data } = await axiosInstance.post('/help-center/tickets', { subject, message })
  return TicketSchema.parse(data)
}

export const fetchTicketMessages = async (
  ticketId: number
): Promise<{ ticket: Ticket; messages: TicketMessage[] }> => {
  const { data } = await axiosInstance.get(`/help-center/tickets/${ticketId}`)
  return TicketMessagesResponseSchema.parse(data)
}

export const postTicketMessage = async (
  ticketId: number,
  body: string,
  file?: File,
  kind?: MessageKind
): Promise<TicketMessage> => {
  const form = new FormData()
  form.append('body', body)
  if (file) {
    form.append('file', file)
    form.append('kind', kind ?? 'attachment')
  }
  const { data } = await axiosInstance.post(`/help-center/tickets/${ticketId}/messages`, form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return TicketMessageSchema.parse(data)
}

export const pingTyping = async (ticketId: number): Promise<void> => {
  await axiosInstance.post(`/help-center/tickets/${ticketId}/typing`)
}

export const fetchTypingStatus = async (ticketId: number): Promise<boolean> => {
  const { data } = await axiosInstance.get(`/help-center/tickets/${ticketId}/typing`)
  return Boolean(data?.typing)
}
