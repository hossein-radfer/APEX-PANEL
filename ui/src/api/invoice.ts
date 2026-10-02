import axiosInstance from '@/api/axios-instance.ts'
import { z } from 'zod'

// amount is a whole-Toman integer -- this panel bills exclusively in Toman
// (Iran's currency, no meaningful sub-unit in practical use), so there is
// no currency field/scaling here.
export const InvoiceSchema = z.object({
  id: z.number(),
  reseller_id: z.number(),
  invoice_number: z.string(),
  status: z.string(),
  amount: z.number(),
  description: z.string(),
  period_start: z.number(),
  period_end: z.number(),
  issued_at: z.number().nullable(),
  due_date: z.number().nullable(),
  paid_at: z.number().nullable(),
  notes: z.string().nullable(),
  created_at: z.number(),
})

const InvoiceListResponseSchema = z.object({
  status: z.string(),
  data: z.array(InvoiceSchema),
})

const InvoiceDetailResponseSchema = z.object({
  status: z.string(),
  data: InvoiceSchema,
})

export type Invoice = z.infer<typeof InvoiceSchema>

export interface CreateInvoicePayload {
  amount: number
  description: string
  period_start?: number
  period_end?: number
  due_date?: number
  notes?: string
  auto_issue?: boolean
}

export const fetchResellerInvoices = async (resellerID: number, limit = 50): Promise<Invoice[]> => {
  const { data } = await axiosInstance.get(`/invoice/reseller/${resellerID}`, { params: { limit } })
  const parsed = InvoiceListResponseSchema.parse(data)
  return parsed.data
}

export const fetchInvoiceDetail = async (invoiceID: number): Promise<Invoice> => {
  const { data } = await axiosInstance.get(`/invoice/${invoiceID}`)
  const parsed = InvoiceDetailResponseSchema.parse(data)
  return parsed.data
}

export const createInvoice = async (resellerID: number, payload: CreateInvoicePayload): Promise<Invoice> => {
  const { data } = await axiosInstance.post(`/invoice/reseller/${resellerID}`, payload)
  const parsed = InvoiceDetailResponseSchema.parse(data)
  return parsed.data
}

export const issueInvoice = async (invoiceID: number): Promise<Invoice> => {
  const { data } = await axiosInstance.post(`/invoice/${invoiceID}/issue`)
  const parsed = InvoiceDetailResponseSchema.parse(data)
  return parsed.data
}

export const payInvoice = async (invoiceID: number): Promise<Invoice> => {
  const { data } = await axiosInstance.post(`/invoice/${invoiceID}/pay`)
  const parsed = InvoiceDetailResponseSchema.parse(data)
  return parsed.data
}

export const cancelInvoice = async (invoiceID: number): Promise<Invoice> => {
  const { data } = await axiosInstance.post(`/invoice/${invoiceID}/cancel`)
  const parsed = InvoiceDetailResponseSchema.parse(data)
  return parsed.data
}
