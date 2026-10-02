import { z } from 'zod'

export const TICKET_STATUSES = ['open', 'closed'] as const
export const TicketStatusSchema = z.enum(TICKET_STATUSES)
export type TicketStatus = z.infer<typeof TicketStatusSchema>

export const MESSAGE_AUTHORS = ['admin', 'customer'] as const
export const MessageAuthorSchema = z.enum(MESSAGE_AUTHORS)
export type MessageAuthor = z.infer<typeof MessageAuthorSchema>

export const MESSAGE_KINDS = ['text', 'attachment', 'voice', 'diagnostic'] as const
export const MessageKindSchema = z.enum(MESSAGE_KINDS)
export type MessageKind = z.infer<typeof MessageKindSchema>

export const TicketSchema = z.object({
  ID: z.number(),
  LicenseID: z.number(),
  Subject: z.string(),
  Status: TicketStatusSchema,
  LastMessageAt: z.string(),
  AdminUnreadCount: z.number(),
  CustomerUnreadCount: z.number(),
  CreatedAt: z.string(),
  UpdatedAt: z.string(),
})
export type Ticket = z.infer<typeof TicketSchema>

export const TicketMessageSchema = z.object({
  ID: z.number(),
  TicketID: z.number(),
  Author: MessageAuthorSchema,
  Kind: MessageKindSchema,
  Body: z.string(),
  AttachmentPath: z.string().optional(),
  AttachmentName: z.string().optional(),
  AttachmentMimeType: z.string().optional(),
  AttachmentSizeBytes: z.number().optional(),
  CreatedAt: z.string(),
})
export type TicketMessage = z.infer<typeof TicketMessageSchema>

export const TicketMessagesResponseSchema = z.object({
  ticket: TicketSchema,
  messages: z.array(TicketMessageSchema),
})
