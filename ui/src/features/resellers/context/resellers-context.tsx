import React, { useState } from 'react'
import useDialogState from '@/hooks/use-dialog-state.tsx'
import { Reseller } from '@/schema/reseller.ts'

type ResellersDialogType = 'add' | 'edit' | 'delete'

interface ResellersContextType {
  open: ResellersDialogType | null
  setOpen: (state: ResellersDialogType | null) => void
  currentRow: Reseller | null
  setCurrentRow: React.Dispatch<React.SetStateAction<Reseller | null>>
}

const ResellersContext = React.createContext<ResellersContextType | null>(null)

interface Props {
  children: React.ReactNode
}

export function ResellersProvider({ children }: Props) {
  const [open, setOpen] = useDialogState<ResellersDialogType>(null)
  const [currentRow, setCurrentRow] = useState<Reseller | null>(null)

  return (
    <ResellersContext.Provider value={{ open, setOpen, currentRow, setCurrentRow }}>
      {children}
    </ResellersContext.Provider>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useResellers = () => {
  const context = React.useContext(ResellersContext)
  if (!context) {
    throw new Error('useResellers must be used within ResellersProvider')
  }
  return context
}

export default ResellersProvider
