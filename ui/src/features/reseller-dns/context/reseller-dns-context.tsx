import React, { useState } from 'react'
import { DNSAccount } from '@/schema/dns-account.ts'
import useDialogState from '@/hooks/use-dialog-state'

type ResellerDNSDialogType = 'add' | 'edit' | 'delete' | 'share'

interface ResellerDNSContextType {
  open: ResellerDNSDialogType | null
  setOpen: (str: ResellerDNSDialogType | null) => void
  currentRow: DNSAccount | null
  setCurrentRow: React.Dispatch<React.SetStateAction<DNSAccount | null>>
}

const ResellerDNSContext = React.createContext<ResellerDNSContextType | null>(
  null
)

interface Props {
  children: React.ReactNode
}

export default function ResellerDNSProvider({ children }: Props) {
  const [open, setOpen] = useDialogState<ResellerDNSDialogType>(null)
  const [currentRow, setCurrentRow] = useState<DNSAccount | null>(null)

  return (
    <ResellerDNSContext value={{ open, setOpen, currentRow, setCurrentRow }}>
      {children}
    </ResellerDNSContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useResellerDNS = () => {
  const context = React.useContext(ResellerDNSContext)

  if (!context) {
    throw new Error(
      'useResellerDNS has to be used within <ResellerDNSContext>'
    )
  }

  return context
}
