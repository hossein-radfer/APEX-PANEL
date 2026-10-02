import { createFileRoute } from '@tanstack/react-router'
import XuiPanels from '@/features/xui-panels'

export const Route = createFileRoute('/_authenticated/xui-panels/')({
  component: XuiPanels,
})
