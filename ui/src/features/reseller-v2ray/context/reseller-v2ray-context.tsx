import React, { useState } from 'react'
import { V2RayPackage } from '@/schema/v2ray.ts'
import useDialogState from '@/hooks/use-dialog-state'

type ResellerV2RayDialogType =
  | 'add'
  | 'edit'
  | 'delete'
  | 'share'
  | 'live-usage'

interface ResellerV2RayContextType {
  open: ResellerV2RayDialogType | null
  setOpen: (str: ResellerV2RayDialogType | null) => void
  currentRow: V2RayPackage | null
  setCurrentRow: React.Dispatch<React.SetStateAction<V2RayPackage | null>>
}

const ResellerV2RayContext =
  React.createContext<ResellerV2RayContextType | null>(null)

interface Props {
  children: React.ReactNode
}

export default function ResellerV2RayProvider({ children }: Props) {
  const [open, setOpen] = useDialogState<ResellerV2RayDialogType>(null)
  const [currentRow, setCurrentRow] = useState<V2RayPackage | null>(null)

  return (
    <ResellerV2RayContext
      value={{ open, setOpen, currentRow, setCurrentRow }}
    >
      {children}
    </ResellerV2RayContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useResellerV2Ray = () => {
  const context = React.useContext(ResellerV2RayContext)

  if (!context) {
    throw new Error(
      'useResellerV2Ray has to be used within <ResellerV2RayContext>'
    )
  }

  return context
}
