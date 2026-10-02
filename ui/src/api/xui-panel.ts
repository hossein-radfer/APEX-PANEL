import {
  CreateXuiPanelRequest,
  CreateXuiPanelSchema,
  TestXuiPanelRequest,
  TestXuiPanelSchema,
  UpdateXuiPanelRequest,
  XuiPanel,
  XuiPanelResponseSchema,
  XuiPanelTestResult,
  XuiPanelTestResultResponseSchema,
  XuiPanelsResponseSchema,
} from '@/schema/xui-panel.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchXuiPanelsList = async (): Promise<XuiPanel[]> => {
  const { data } = await axiosInstance.get('/xui-panel')
  const parsed = XuiPanelsResponseSchema.parse(data)
  return parsed.data || []
}

export const createXuiPanel = async (
  panel: CreateXuiPanelRequest
): Promise<XuiPanel> => {
  const validated = CreateXuiPanelSchema.parse(panel)
  const { data } = await axiosInstance.post('/xui-panel', validated)
  const parsed = XuiPanelResponseSchema.parse(data)
  return parsed.data
}

export const updateXuiPanel = async (
  panel: UpdateXuiPanelRequest
): Promise<XuiPanel> => {
  const { data } = await axiosInstance.put(`/xui-panel/${panel.id}`, panel)
  const parsed = XuiPanelResponseSchema.parse(data)
  return parsed.data
}

export const deleteXuiPanel = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/xui-panel/${id}`)
}

// Tests an already-saved panel's stored credentials.
export const testSavedXuiPanel = async (
  id: number
): Promise<XuiPanelTestResult> => {
  const { data } = await axiosInstance.post(`/xui-panel/${id}/test`)
  const parsed = XuiPanelTestResultResponseSchema.parse(data)
  return parsed.data
}

// Tests in-progress form values before the panel has been saved -- used by
// the create dialog to populate the default_inbound_id dropdown.
export const testUnsavedXuiPanel = async (
  panel: TestXuiPanelRequest
): Promise<XuiPanelTestResult> => {
  const validated = TestXuiPanelSchema.parse(panel)
  const { data } = await axiosInstance.post('/xui-panel/test', validated)
  const parsed = XuiPanelTestResultResponseSchema.parse(data)
  return parsed.data
}
