import React, { useState } from 'react'
import { XuiPanel } from '@/schema/xui-panel.ts'
import useDialogState from '@/hooks/use-dialog-state'

type XuiPanelsDialogType = 'add' | 'edit' | 'delete'

interface XuiPanelsContextType {
  open: XuiPanelsDialogType | null
  setOpen: (str: XuiPanelsDialogType | null) => void
  currentRow: XuiPanel | null
  setCurrentRow: React.Dispatch<React.SetStateAction<XuiPanel | null>>
}

const XuiPanelsContext = React.createContext<XuiPanelsContextType | null>(
  null
)

interface Props {
  children: React.ReactNode
}

export default function XuiPanelsProvider({ children }: Props) {
  const [open, setOpen] = useDialogState<XuiPanelsDialogType>(null)
  const [currentRow, setCurrentRow] = useState<XuiPanel | null>(null)

  return (
    <XuiPanelsContext value={{ open, setOpen, currentRow, setCurrentRow }}>
      {children}
    </XuiPanelsContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useXuiPanels = () => {
  const context = React.useContext(XuiPanelsContext)

  if (!context) {
    throw new Error('useXuiPanels has to be used within <XuiPanelsContext>')
  }

  return context
}
