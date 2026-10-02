import React, { useState } from 'react'
import { Peer } from '@/schema/peers.ts'
import useDialogState from '@/hooks/use-dialog-state'

type ResellerPeersDialogType =
  | 'add'
  | 'edit'
  | 'delete'
  | 'share'
  | 'qrCode'
  | 'show_config'
  | 'download_config'

interface ResellerPeersContextType {
  open: ResellerPeersDialogType | null
  setOpen: (str: ResellerPeersDialogType | null) => void
  currentRow: Peer | null
  setCurrentRow: React.Dispatch<React.SetStateAction<Peer | null>>
}

const ResellerPeersContext =
  React.createContext<ResellerPeersContextType | null>(null)

interface Props {
  children: React.ReactNode
}

export default function ResellerPeersProvider({ children }: Props) {
  const [open, setOpen] = useDialogState<ResellerPeersDialogType>(null)
  const [currentRow, setCurrentRow] = useState<Peer | null>(null)

  return (
    <ResellerPeersContext value={{ open, setOpen, currentRow, setCurrentRow }}>
      {children}
    </ResellerPeersContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useResellerPeers = () => {
  const context = React.useContext(ResellerPeersContext)

  if (!context) {
    throw new Error(
      'useResellerPeers has to be used within <ResellerPeersContext>'
    )
  }

  return context
}
