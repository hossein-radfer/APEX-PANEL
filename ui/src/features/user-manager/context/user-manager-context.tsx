import React, { useState } from 'react'
import { UserManagerAccount } from '@/schema/user-manager.ts'
import useDialogState from '@/hooks/use-dialog-state'

type UserManagerDialogType =
  | 'add'
  | 'edit'
  | 'delete'
  | 'share'
  | 'config'
  | 'password'
  | 'bulk-import'
  | 'bulk-create'
  | 'expired'

interface UserManagerContextType {
  open: UserManagerDialogType | null
  setOpen: (str: UserManagerDialogType | null) => void
  currentRow: UserManagerAccount | null
  setCurrentRow: React.Dispatch<React.SetStateAction<UserManagerAccount | null>>
}

const UserManagerContext = React.createContext<UserManagerContextType | null>(
  null
)

interface Props {
  children: React.ReactNode
}

export default function UserManagerProvider({ children }: Props) {
  const [open, setOpen] = useDialogState<UserManagerDialogType>(null)
  const [currentRow, setCurrentRow] = useState<UserManagerAccount | null>(null)

  return (
    <UserManagerContext value={{ open, setOpen, currentRow, setCurrentRow }}>
      {children}
    </UserManagerContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useUserManager = () => {
  const context = React.useContext(UserManagerContext)

  if (!context) {
    throw new Error(
      'useUserManager has to be used within <UserManagerContext>'
    )
  }

  return context
}
