import React, { useState } from 'react'
import { V2RayPackage } from '@/schema/v2ray.ts'
import useDialogState from '@/hooks/use-dialog-state'

type V2RayDialogType =
  | 'add'
  | 'edit'
  | 'delete'
  | 'share'
  | 'live-usage'
  | 'bulk-create'
  | 'expired'

interface V2RayContextType {
  open: V2RayDialogType | null
  setOpen: (str: V2RayDialogType | null) => void
  currentRow: V2RayPackage | null
  setCurrentRow: React.Dispatch<React.SetStateAction<V2RayPackage | null>>
}

const V2RayContext = React.createContext<V2RayContextType | null>(null)

interface Props {
  children: React.ReactNode
}

export default function V2RayProvider({ children }: Props) {
  const [open, setOpen] = useDialogState<V2RayDialogType>(null)
  const [currentRow, setCurrentRow] = useState<V2RayPackage | null>(null)

  return (
    <V2RayContext value={{ open, setOpen, currentRow, setCurrentRow }}>
      {children}
    </V2RayContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useV2Ray = () => {
  const context = React.useContext(V2RayContext)

  if (!context) {
    throw new Error('useV2Ray has to be used within <V2RayContext>')
  }

  return context
}
