import React, { useState } from 'react'
import { UserManagerAccount } from '@/schema/user-manager.ts'
import useDialogState from '@/hooks/use-dialog-state'

type ResellerUserManagerDialogType =
  | 'add'
  | 'edit'
  | 'delete'
  | 'share'
  | 'config'
  | 'password'
  | 'bulk-import'

interface ResellerUserManagerContextType {
  open: ResellerUserManagerDialogType | null
  setOpen: (str: ResellerUserManagerDialogType | null) => void
  currentRow: UserManagerAccount | null
  setCurrentRow: React.Dispatch<React.SetStateAction<UserManagerAccount | null>>
}

const ResellerUserManagerContext =
  React.createContext<ResellerUserManagerContextType | null>(null)

interface Props {
  children: React.ReactNode
}

export default function ResellerUserManagerProvider({ children }: Props) {
  const [open, setOpen] = useDialogState<ResellerUserManagerDialogType>(null)
  const [currentRow, setCurrentRow] = useState<UserManagerAccount | null>(null)

  return (
    <ResellerUserManagerContext
      value={{ open, setOpen, currentRow, setCurrentRow }}
    >
      {children}
    </ResellerUserManagerContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useResellerUserManager = () => {
  const context = React.useContext(ResellerUserManagerContext)

  if (!context) {
    throw new Error(
      'useResellerUserManager has to be used within <ResellerUserManagerContext>'
    )
  }

  return context
}
