import React, { useState } from 'react'
import { Application } from '@/schema/application.ts'
import useDialogState from '@/hooks/use-dialog-state'

type ApplicationDialogType = 'add' | 'edit' | 'delete' | 'details'

interface ApplicationContextType {
  open: ApplicationDialogType | null
  setOpen: (str: ApplicationDialogType | null) => void
  currentRow: Application | null
  setCurrentRow: React.Dispatch<React.SetStateAction<Application | null>>
}

const ApplicationContext = React.createContext<ApplicationContextType | null>(
  null
)

interface Props {
  children: React.ReactNode
}

export default function ApplicationProvider({ children }: Props) {
  const [open, setOpen] = useDialogState<ApplicationDialogType>(null)
  const [currentRow, setCurrentRow] = useState<Application | null>(null)

  return (
    <ApplicationContext value={{ open, setOpen, currentRow, setCurrentRow }}>
      {children}
    </ApplicationContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useApplication = () => {
  const context = React.useContext(ApplicationContext)

  if (!context) {
    throw new Error(
      'useApplication has to be used within <ApplicationContext>'
    )
  }

  return context
}
