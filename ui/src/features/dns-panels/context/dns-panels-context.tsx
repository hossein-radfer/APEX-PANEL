import React, { useState } from 'react'
import { DNSPanel } from '@/schema/dns-panel.ts'
import useDialogState from '@/hooks/use-dialog-state'

type DNSPanelsDialogType = 'add' | 'edit' | 'delete'

interface DNSPanelsContextType {
  open: DNSPanelsDialogType | null
  setOpen: (str: DNSPanelsDialogType | null) => void
  currentRow: DNSPanel | null
  setCurrentRow: React.Dispatch<React.SetStateAction<DNSPanel | null>>
}

const DNSPanelsContext = React.createContext<DNSPanelsContextType | null>(
  null
)

interface Props {
  children: React.ReactNode
}

export default function DNSPanelsProvider({ children }: Props) {
  const [open, setOpen] = useDialogState<DNSPanelsDialogType>(null)
  const [currentRow, setCurrentRow] = useState<DNSPanel | null>(null)

  return (
    <DNSPanelsContext value={{ open, setOpen, currentRow, setCurrentRow }}>
      {children}
    </DNSPanelsContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useDNSPanels = () => {
  const context = React.useContext(DNSPanelsContext)

  if (!context) {
    throw new Error('useDNSPanels has to be used within <DNSPanelsContext>')
  }

  return context
}
