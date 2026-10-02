import { useQuery } from '@tanstack/react-query'
import { fetchWalletBalance, fetchLedgerHistory, fetchPricePlans } from '@/api/wallet.ts'

export const useWalletBalanceQuery = (resellerID: number | null | undefined) =>
  useQuery({
    queryKey: ['wallet_balance', resellerID],
    queryFn: () => fetchWalletBalance(resellerID!),
    enabled: !!resellerID,
  })

export const useLedgerHistoryQuery = (resellerID: number | null | undefined, limit = 50) =>
  useQuery({
    queryKey: ['wallet_ledger', resellerID, limit],
    queryFn: () => fetchLedgerHistory(resellerID!, limit),
    enabled: !!resellerID,
  })

export const usePricePlansQuery = () =>
  useQuery({
    queryKey: ['price_plans'],
    queryFn: fetchPricePlans,
  })
