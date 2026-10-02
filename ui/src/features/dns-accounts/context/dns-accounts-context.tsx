import React, { useState } from 'react'
import { DNSAccount } from '@/schema/dns-account.ts'
import useDialogState from '@/hooks/use-dialog-state'

type DNSAccountsDialogType = 'add' | 'edit' | 'delete' | 'share' | 'expired'

interface DNSAccountsContextType {
  open: DNSAccountsDialogType | null
  setOpen: (str: DNSAccountsDialogType | null) => void
  currentRow: DNSAccount | null
  setCurrentRow: React.Dispatch<React.SetStateAction<DNSAccount | null>>
}

const DNSAccountsContext = React.createContext<DNSAccountsContextType | null>(
  null
)

interface Props {
  children: React.ReactNode
}

export default function DNSAccountsProvider({ children }: Props) {
  const [open, setOpen] = useDialogState<DNSAccountsDialogType>(null)
  const [currentRow, setCurrentRow] = useState<DNSAccount | null>(null)

  return (
    <DNSAccountsContext value={{ open, setOpen, currentRow, setCurrentRow }}>
      {children}
    </DNSAccountsContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useDNSAccounts = () => {
  const context = React.useContext(DNSAccountsContext)

  if (!context) {
    throw new Error('useDNSAccounts has to be used within <DNSAccountsContext>')
  }

  return context
}
