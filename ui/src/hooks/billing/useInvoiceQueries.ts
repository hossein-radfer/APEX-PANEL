import { useQuery } from '@tanstack/react-query'
import { fetchResellerInvoices, fetchInvoiceDetail } from '@/api/invoice.ts'

export const useResellerInvoicesQuery = (resellerID: number | null | undefined, limit = 50) =>
  useQuery({
    queryKey: ['invoices', resellerID, limit],
    queryFn: () => fetchResellerInvoices(resellerID!, limit),
    enabled: !!resellerID,
  })

export const useInvoiceDetailQuery = (invoiceID: number | null | undefined) =>
  useQuery({
    queryKey: ['invoice', invoiceID],
    queryFn: () => fetchInvoiceDetail(invoiceID!),
    enabled: !!invoiceID,
  })
